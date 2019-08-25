package sfab

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

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

	// Path on the filesystem to the SSH Private Key to use
	// for the SSH server component of this Hub.
	//
	HostKeyFile string

	// Path on the filesystem to the list of authorized
	// keys of sFAB Agents that are statically allowed
	// to connect to this sFAB Hub.
	//
	AuthKeysFile string

	// Concurrency guard, for access to the agents map
	// from multiple (handler) goroutines.
	//
	lk sync.Mutex

	// The network listener that we await new inbound SSH
	// connections on.
	//
	listener net.Listener

	// A directory of registered agents.
	agents map[string]chan []byte
}

func (h *Hub) Listen() error {
	h.lock()
	h.agents = make(map[string]chan []byte)
	h.unlock()

	if h.IPProto == "" {
		h.IPProto = "tcp4"
	}

	if h.HostKeyFile == "" {
		return fmt.Errorf("missing HostKeyFile in Hub object.")
	}

	if h.AuthKeysFile == "" {
		return fmt.Errorf("missing AuthKeysFile in Hub object.")
	}

	hostKey, err := loadPrivateKey(h.HostKeyFile)
	if err != nil {
		return err
	}

	authKeys, err := loadAuthKeys(h.AuthKeysFile)
	if err != nil {
		return err
	}

	certChecker := &ssh.CertChecker{
		IsUserAuthority: func(key ssh.PublicKey) bool {
			return false
		},

		UserKeyFallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			for _, k := range authKeys {
				if bytes.Equal(k.Marshal(), key.Marshal()) {
					return nil, nil
				}
			}

			return nil, fmt.Errorf("unknown public key")
		},
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return certChecker.Authenticate(conn, key)
		},
	}

	config.AddHostKey(hostKey)

	h.listener, err = net.Listen(h.IPProto, h.Bind)
	if err != nil {
		return err
	}

	id := 0
	for {
		fmt.Fprintf(os.Stderr, "awaiting connection...\n")

		id += 1
		socket, err := h.listener.Accept()
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "[%d] inbound connection accepted; starting SSH handshake...\n", id)
		c, chans, reqs, err := ssh.NewServerConn(socket, config)
		if err != nil {
			return err
		}

		go ignoreGlobalRequests(reqs)
		go ignoreNewChannels(chans)

		go func() {
			tick := time.NewTicker(500 * time.Millisecond)
			for range tick.C {
				_, _, err := c.SendRequest("keepalive", true, nil)
				if err != nil && err == io.EOF {
					fmt.Fprintf(os.Stderr, "keepalive failed; disconnecting...\n")
					h.unregister(c.User())
					c.Close()
					return
				}
			}
		}()

		events := make(chan []byte)
		if err := h.register(c.User(), events); err != nil {
			c.Close()
			continue
		}

		go func(id int) {
			for m := range events {
				ch, in, err := c.OpenChannel("session", nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[%d] failed to open session channel: %s\n", id, err)
					if err == io.EOF {
						fmt.Fprintf(os.Stderr, "[%d] disconnecting?\n", id)
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

				fmt.Fprintf(os.Stderr, "[%d] spinning a goroutine to handle channel requests...\n", id)
				go func() {
					for msg := range in {
						fmt.Fprintf(os.Stderr, "[%d] got request of type '%s'...\n", id, msg.Type)
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
					Command: string(m),
				}
				ok, err := ch.SendRequest("exec", true, ssh.Marshal(&ex))
				if err != nil {
					fmt.Fprintf(os.Stderr, "[%d] failed to send exec request to remote agent: %s\n", id, err)
					continue
				}
				if !ok {
					fmt.Fprintf(os.Stderr, "[%d] failed to send exec request to remote agent: general failure?\n", id)
					continue
				}

				fmt.Fprintf(os.Stderr, "[%d] spinning a goroutine to handle standard output...\n", id)
				go func() {
					in := bufio.NewScanner(ch)
					for in.Scan() {
						fmt.Fprintf(os.Stderr, "[%d] STDOUT | %s\n", id, in.Text())
					}
				}()

				fmt.Fprintf(os.Stderr, "[%d] spinning a goroutine to handle standard error...\n", id)
				go func() {
					in := bufio.NewScanner(ch)
					for in.Scan() {
						fmt.Fprintf(os.Stderr, "[%d] STDERR | %s\n", id, in.Text())
					}
				}()

				fmt.Fprintf(os.Stderr, "[%d] closing standard input (we have none to offer anyway)...\n", id)
				ch.CloseWrite()

				fmt.Fprintf(os.Stderr, "[%d] waiting for remote end to finish up...\n", id)
				<-done
			}

			h.unregister(c.User())
			c.Close()
		}(id)
	}
}

func (h *Hub) lock() {
	h.lk.Lock()
}

func (h *Hub) unlock() {
	h.lk.Unlock()
}

func (h *Hub) Send(agent string, payload []byte) error {
	h.lock()
	defer h.unlock()

	if ch, ok := h.agents[agent]; ok {
		fmt.Fprintf(os.Stderr, "writing message to channel...\n")
		ch <- payload
		fmt.Fprintf(os.Stderr, "WROTE message to channel...\n")
		return nil
	} else {
		return fmt.Errorf("no such agent (%s)", agent)
	}
}

func (h *Hub) register(name string, ch chan []byte) error {
	h.lock()
	defer h.unlock()

	fmt.Fprintf(os.Stderr, "registering agent (%s)\n", name)
	if _, found := h.agents[name]; found {
		return fmt.Errorf("agent (%s) already registered", name)
	}

	h.agents[name] = ch
	return nil
}

func (h *Hub) unregister(name string) {
	h.lock()
	defer h.unlock()

	fmt.Fprintf(os.Stderr, "unregistering agent (%s)\n", name)
	if ch, found := h.agents[name]; found {
		delete(h.agents, name)
		close(ch)
	}
	fmt.Fprintf(os.Stderr, "done unregistering agent (%s)\n", name)
}
