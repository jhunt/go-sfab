package sfab

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/jhunt/go-log"
	"golang.org/x/crypto/ssh"
)

const DefaultKeepAlive time.Duration = 60 * time.Second

type AgentCallback func(string, Key)

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

	// Whether or not to allow connecting agents to finish
	// connecting before their private keys have been authorized
	// by the hub (or its operators).
	//
	AllowUnauthorizedAgents bool

	// Which IP protocol (tcp4 or tcp6) to use for binding
	// the server component of this sFAB Hub.
	//
	IPProto string

	// Private Key to use for the server component of this Hub.
	//
	HostKey *Key

	// How frequently to send KeepAlive messages to connected
	// agents, to keep their TCP transport channels open.
	//
	// By default, no KeepAlives are sent.
	//
	KeepAlive time.Duration

	// An optional function to be called when a new agent
	// registers with the hub (authorized or not).
	//
	OnConnect AgentCallback

	// An optional function to be called when a registered
	// agent (authorized or not) deregisters from the hub,
	// or is forcibly deregistered after a missed heartbeat.
	//
	OnDisconnect AgentCallback

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
	agents map[string]*connection

	// A directory of awaited agents.
	awaits map[string]chan int

	// A KeyMaster, for tracking authorized Agent keys.
	//
	keys *KeyMaster
}

// Listen binds a network socket for the Hub.
//
func (h *Hub) Listen() error {
	h.lock()
	h.agents = make(map[string]*connection)
	h.awaits = make(map[string]chan int)
	h.unlock()

	if h.IPProto == "" {
		h.IPProto = "tcp4"
	}

	if h.HostKey == nil {
		return fmt.Errorf("missing HostKey in Hub object.")
	}

	h.init()
	ck := &ssh.CertChecker{
		UserKeyFallback: h.keys.userKeyCallback(),
		IsUserAuthority: func(key ssh.PublicKey) bool {
			return false
		},
	}

	h.config = &ssh.ServerConfig{
		PublicKeyCallback: ck.Authenticate,
	}
	h.config.AddHostKey(h.HostKey.signer)

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

	if h.KeepAlive <= 0 {
		h.KeepAlive = DefaultKeepAlive
	}

	for {
		log.Debugf("[hub] awaiting inbound connections...")

		socket, err := h.listener.Accept()
		if err != nil {
			log.Debugf("[hub] failed to accept inbound connection: %s", err)
			continue
		}

		log.Debugf("[hub] inbound connection accepted; starting SSH handshake...")
		c, chans, reqs, err := ssh.NewServerConn(socket, h.config)
		if err != nil {
			log.Debugf("[hub] failed to negotiate SSH transport: %s", err)
			continue
		}

		log.Infof("[hub] registering agent '%s' with public key: %s\n", c.User(), c.Permissions.Extensions[PublicKeyExtensionName])
		connection, err := h.register(c.User(), c)
		if err != nil {
			log.Errorf("[hub] failed to register agent '%s': %s", c.User(), err)
			c.Conn.Close()
			continue
		}
		go connection.Serve(chans, reqs, h.KeepAlive)
		if h.OnConnect != nil {
			log.Infof("[hub] calling onconnect handler for '%s'", connection.identity)
			h.OnConnect(connection.identity, *connection.key)
		}
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

func (h *Hub) init() {
	if h.keys == nil {
		h.keys = &KeyMaster{
			strict: !h.AllowUnauthorizedAgents,
		}
	}
}

func (h *Hub) authorizeKey(agent string, key *Key) {
	h.init()
	h.keys.Authorize(key, agent)
}

// AuthorizeKey tells the Hub to start trusting a given SSH
// key pair, given the public component, for a named agent.
//
// This can be called dynamically, long after a call to Listen(),
// or before.
//
func (h *Hub) AuthorizeKey(agent string, key *Key) {
	h.lock()
	defer h.unlock()

	log.Debugf("authorizing subject '%s' with key [%s]", agent, key.Fingerprint())
	h.authorizeKey(agent, key)
}

func (h *Hub) deauthorizeKey(agent string, key *Key) {
	h.init()
	h.keys.Deauthorize(key, agent)
}

// DeauthorizeKey tells the Hub to stop trusting a given SSH
// key pair, given the public component, for a named agent.
//
// This can be called dynamically, long after a call to Listen(),
// or before.
//
func (h *Hub) DeauthorizeKey(agent string, key *Key) {
	h.lock()
	defer h.unlock()

	log.Debugf("deauthorizing subject '%s' with key [%s]", agent, key.Fingerprint())
	h.deauthorizeKey(agent, key)
}

// Send a message to an agent (by name).  Returns an error
// if the named agent is not currently registered with this
// Hub.
//
// If an Agent is found, Responses (including output and the
// ultimate exit code) will be sent via the returned channel.
//
func (h *Hub) Send(agent string, message []byte, timeout time.Duration) (chan *Response, error) {
	h.lock()
	c, ok := h.agents[agent]
	h.unlock()

	if ok {
		if h.keys.Authorized(agent, c.key) {
			msg := Message{
				responses: make(chan *Response),
				payload:   message,
			}
			select {
			case c.messages <- msg:
				return msg.responses, nil

			case <-time.After(timeout):
				return nil, fmt.Errorf("agent did not respond within %ds", int(timeout.Seconds()))
			}
		} else {
			return nil, fmt.Errorf("agent found but not authorized: %s", agent)
		}
	} else {
		return nil, fmt.Errorf("agent not found: %s", agent)
	}
}

// IgnoreReplies takes a response channel from a
// call to Send() and discards all of the responses
// that are sent across.
//
// It's perfect for a goroutine!
//
func (h *Hub) IgnoreReplies(ch chan *Response) {
	for range ch {
	}
}

// Agents() returns a list of all registered
// (and current!) Agent names, to allow customers
// to blast out messages to _everyone_ if they so
// desire.
//
func (h *Hub) Agents() []string {
	h.lock()
	defer h.unlock()

	agents := make([]string, 0)
	for k := range h.agents {
		agents = append(agents, k)
	}

	return agents
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

func (h *Hub) Await(agent string) chan int {
	h.lock()
	defer h.unlock()

	if ch, ok := h.awaits[agent]; ok {
		return ch
	} else {
		ch := make(chan int)
		h.awaits[agent] = ch
		return ch
	}
}

func (h *Hub) lock() {
	h.lk.Lock()
}

func (h *Hub) unlock() {
	h.lk.Unlock()
}

func (h *Hub) register(name string, conn *ssh.ServerConn) (*connection, error) {
	h.lock()
	defer h.unlock()
	if _, found := h.agents[name]; found {
		return nil, fmt.Errorf("agent '%s' already registered", name)
	}

	h.agents[name] = &connection{
		ssh:      conn,
		messages: make(chan Message),
		hangup:   make(chan int, 1),
		identity: conn.User(),
		key:      h.keys.publicKeyUsed(conn),

		done: func() {
			h.lock()
			defer h.unlock()
			log.Infof("[hub] deregistering agent '%v'...", conn.User())
			if h.OnDisconnect != nil {
				log.Infof("[hub] calling disconnect handler for '%s'", conn.User())
				h.OnDisconnect(conn.User(), *h.keys.publicKeyUsed(conn))
			}
			delete(h.awaits, name)
			delete(h.agents, name)
		},
	}

	if _, found := h.awaits[name]; !found {
		h.awaits[name] = make(chan int)
	}
	close(h.awaits[name])

	return h.agents[name], nil
}

func (h *Hub) Authorizations() []Authorization {
	h.lock()
	defer h.unlock()

	h.init()
	return h.keys.Authorizations()
}
