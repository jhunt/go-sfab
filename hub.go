package sfab

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/jhunt/go-log"
	"golang.org/x/crypto/ssh"
)

// A Hub represents a server from whence jobs to execute are
// dispatched.  sFAB Agents connect _to_ a Hub, and await
// instructions.
//
type Hub struct {
	// The IP address (or hostname / FQDN) and TCP port to
	// bind and listen on for incoming SSH connections from
	// sFAB Agents.
	//
	Bind string

	// Which IP protocol (tcp4 or tcp6) to use for binding
	// the server component of this sFAB Hub.
	//
	IPProto string

	// Private Key to use for the server component of this Hub.
	//
	HostKey ssh.Signer

	// How frequently to send KeepAlive messages to connected
	// agents, to keep their TCP transport channels open.
	//
	// By default, no KeepAlives are sent.
	//
	KeepAlive time.Duration

	// Concurrency guard, for access to the agents map
	// from multiple (handler) goroutines.
	//
	lk sync.Mutex

	// The network listener that we await new inbound SSH
	// connections on.
	//
	listener net.Listener

	// The x/crypto/ssh configuration for setting up the
	// server <-> client communication channel(s).
	//
	config *ssh.ServerConfig

	// A directory of registered agents.
	agents map[string]chan Message

	// A KeyMaster, for tracking authorized Agent keys.
	//
	keys *KeyMaster
}

// Listen binds a network socket for the Hub.
//
func (h *Hub) Listen() error {
	h.lock()
	h.agents = make(map[string]chan Message)
	h.unlock()

	if h.IPProto == "" {
		h.IPProto = "tcp4"
	}

	if h.HostKey == nil {
		return fmt.Errorf("missing HostKey in Hub object.")
	}

	ck := &ssh.CertChecker{
		IsUserAuthority: func(key ssh.PublicKey) bool {
			return false
		},

		UserKeyFallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if h.keys != nil && h.keys.Authorized(c.User(), key) {
				return nil, nil
			}
			return nil, fmt.Errorf("unknown public key")
		},
	}

	h.config = &ssh.ServerConfig{
		PublicKeyCallback: ck.Authenticate,
	}
	h.config.AddHostKey(h.HostKey)

	var err error
	h.listener, err = net.Listen(h.IPProto, h.Bind)
	if err != nil {
		return err
	}

	return nil
}

// Serve handls inbound cnnections on the listening socket, and
// services those agents, distributing messages via a session
// channel and an exec request, each.
//
// It is the caller's responsibility to call Listen() before
// invoking this method, or to dispense with both and just use
// ListenAndServe().
//
func (h *Hub) Serve() error {
	if h.listener == nil {
		return fmt.Errorf("this hub has no listener (did you forget to call Listen() first?)")
	}

	id := 0
	for {
		log.Debugf("[hub] awaiting inbound connections...")

		id += 1
		socket, err := h.listener.Accept()
		if err != nil {
			continue
		}

		log.Debugf("[hub conn %d] inbound connection accepted; starting SSH handshake...", id)
		c, chans, reqs, err := ssh.NewServerConn(socket, h.config)
		if err != nil {
			continue
		}

		log.Debugf("[hub conn %d] ignoring global requests from connected agent...", id)
		go ignoreGlobalRequests(reqs)
		log.Debugf("[hub conn %d] ignoring new channel requests from connected agent...", id)
		go ignoreNewChannels(chans)

		if h.KeepAlive > 0 {
			log.Debugf("[hub conn %d] sending keepalives at %fs interval", id, h.KeepAlive.Seconds())
			go func() {
				tick := time.NewTicker(h.KeepAlive)
				for range tick.C {
					_, _, err := c.SendRequest("keepalive", true, nil)
					if err != nil && err == io.EOF {
						log.Debugf("[hub conn %d] keepalive failed (%s); disconnecting...", id, err)
						h.unregister(c.User())
						c.Close()
						return
					}
				}
			}()
		}

		events := make(chan Message)
		if err := h.register(c.User(), events); err != nil {
			c.Close()
			continue
		}

		go func(id int) {
			for m := range events {
				ch, in, err := c.OpenChannel("session", nil)
				if err != nil {
					log.Debugf("[hub conn %d] failed to open a session channel: %s", id, err)
					if err == io.EOF {
						log.Debugf("[hub conn %d] disconnecting.", id)
						break
					}
					continue
				}

				type exited struct {
					code   int
					signal string
					err    error
				}
				rc := exited{code: -1}
				done := make(chan exited)

				log.Debugf("[hub conn %d] spinning a goroutine to handle channel requests...", id)
				go func() {
					for msg := range in {
						log.Debugf("[hub conn %d] received '%s' request...", id, msg.Type)
						switch msg.Type {
						case "exit-status":
							rc.code = int(binary.BigEndian.Uint32(msg.Payload))

						case "exit-signal":
							var siggy struct {
								Signal     string
								CoreDumped bool
								Error      string
								Lang       string
							}
							if err := ssh.Unmarshal(msg.Payload, &siggy); err != nil {
								rc.err = err
								done <- rc
								return
							}
							rc.signal = siggy.Signal
							rc.err = fmt.Errorf("remote error: %s\n", siggy.Error)

						default:
							if msg.WantReply {
								msg.Reply(false, nil)
							}
						}
					}

					done <- rc
				}()

				ex := struct {
					Command string
				}{
					Command: string(m.payload),
				}
				ok, err := ch.SendRequest("exec", true, ssh.Marshal(&ex))
				if err != nil {
					log.Debugf("[hub conn %d] failed to send exec request to remote agent: %s", id, err)
					continue
				}
				if !ok {
					log.Debugf("[hub conn %d] failed to send exec request to remote agent: unspecified failure", id)
					continue
				}

				log.Debugf("[hub conn %d] spinning goroutines to handle standard output and standard error...", id)
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					h.drain(stdoutSource, ch, m.responses)
					log.Debugf("[hub conn %d] done processing STDOUT channel...", id)
					wg.Done()
				}()
				go func() {
					h.drain(stderrSource, ch.Stderr(), m.responses)
					log.Debugf("[hub conn %d] done processing STDERR channel...", id)
					wg.Done()
				}()

				log.Debugf("[hub conn %d] closing standard input (we have none to offer anyway)...", id)
				ch.CloseWrite()

				log.Debugf("[hub conn %d] waiting for remote end to finish up...", id)
				x := <-done

				log.Debugf("[hub conn %d] closing SSH 'session' channel...", id)
				ch.Close()

				log.Debugf("[hub conn %d] waiting for output sink goroutines to spin down...", id)
				wg.Wait()

				log.Debugf("[hub conn %d] sending exit response to our caller...", id)
				m.responses <- Response{
					source: exitSource,
					rc:     x.code,
				}

				log.Debugf("[hub conn %d] closing message response channel...", id)
				close(m.responses)
			}

			h.unregister(c.User())
			c.Close()
		}(id)
	}
}

// ListenAndServe combines both the Listen() and Serve()
// methods into a convenient helper method that runs both,
// serially, and returns whichever error pops up first.
//
// You probably want to run this in the main goroutine, much like
// net/http's ListenAndServe().
//
func (h *Hub) ListenAndServe() error {
	err := h.Listen()
	if err != nil {
		return err
	}

	return h.Serve()
}

func (h *Hub) authorizeKey(agent string, key ssh.PublicKey) {
	if h.keys == nil {
		h.keys = &KeyMaster{}
	}
	h.keys.Authorize(key, agent)
}

// AuthorizeKey tells the Hub to start trusting a given SSH
// key pair, given the public component, for a named agent.
//
// This can be called dynamically, long after a call to Listen(),
// or before.
//
func (h *Hub) AuthorizeKey(agent string, key ssh.PublicKey) {
	h.lock()
	defer h.unlock()

	h.authorizeKey(agent, key)
}

func (h *Hub) deauthorizeKey(agent string, key ssh.PublicKey) {
	if h.keys == nil {
		h.keys = &KeyMaster{}
	}
	h.keys.Deauthorize(key, agent)
}

// DeauthorizeKey tells the Hub to stop trusting a given SSH
// key pair, given the public component, for a named agent.
//
// This can be called dynamically, long after a call to Listen(),
// or before.
//
func (h *Hub) DeauthorizeKey(agent string, key ssh.PublicKey) {
	h.lock()
	defer h.unlock()

	h.deauthorizeKey(agent, key)
}

// AuthorizeKeys reads the given file, which is expected to
// be in OpenSSH's _authorized keys format_, and trusts each
// and every key in their, for the named agents in the associated
// comments.
//
// This can be called dynamically, long after a call to Listen(),
// or before.
//
func (h *Hub) AuthorizeKeys(file string) error {
	h.lock()
	defer h.unlock()

	return withAuthKeys(file, h.authorizeKey)
}

// DeauthorizeKeys reads the given file, which is expected to
// be in OpenSSH's _authorized keys format_, and deauthorizes each
// and every key in their, for the named agents in the associated
// comments.
//
// This can be called dynamically, long after a call to Listen(),
// or before.
//
func (h *Hub) DeauthorizeKeys(file string) error {
	h.lock()
	defer h.unlock()

	return withAuthKeys(file, h.deauthorizeKey)
}

// Send a message to an agent (by name).  Returns an error
// if the named agent is not currently registered with this
// Hub.
//
// If an Agent is found, Responses (including output and the
// ultimate exit code) will be sent via the returned channel.
//
func (h *Hub) Send(agent string, message []byte) (chan Response, error) {
	h.lock()
	defer h.unlock()

	if ch, ok := h.agents[agent]; ok {
		reply := make(chan Response)
		ch <- Message{
			responses: reply,
			payload:   message,
		}
		return reply, nil

	} else {
		return nil, fmt.Errorf("no such agent (%s)", agent)
	}
}

// KnowsAgent checks the Hub's agent directory to see if
// a named agent has registered with this Hub.
//
func (h *Hub) KnowsAgent(agent string) bool {
	h.lock()
	defer h.unlock()

	_, ok := h.agents[agent]
	return ok
}

func (h *Hub) lock() {
	h.lk.Lock()
}

func (h *Hub) unlock() {
	h.lk.Unlock()
}

func (h *Hub) register(name string, ch chan Message) error {
	h.lock()
	defer h.unlock()

	log.Debugf("[hub] registering agent '%s'", name)
	if _, found := h.agents[name]; found {
		return fmt.Errorf("agent '%s' already registered", name)
	}

	h.agents[name] = ch
	return nil
}

func (h *Hub) unregister(name string) {
	h.lock()
	defer h.unlock()

	log.Debugf("[hub] unregistering agent '%s'", name)
	if ch, found := h.agents[name]; found {
		delete(h.agents, name)
		close(ch)
	}
	log.Debugf("[hub] done unregistering agent '%s'", name)
}

func (*Hub) drain(src source, in io.Reader, out chan Response) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		out <- Response{
			source: src,
			text:   sc.Text(),
		}
	}
}
