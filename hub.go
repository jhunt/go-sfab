package main

import (
	"bufio"
	"sync"
	"bytes"
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
)

// A Hub represents a server from whence jobs to execute are
// dispatched.  sFAB Agents connect _to_ a Hub, and await
// instructions.
//
type Hub struct {
	// The unique string that identifies agents as belonging
	// to this particular Hub.
	//
	Hail string

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
	lock sync.Mutex

	// The network listener that we await new inbound SSH
	// connections on.
	//
	listener net.Listener

	// A directory of registered agents.
	agents map[string]chan Message
}

func (h *Hub) Listen() error {
	h.agents = make(map[string] chan Message)

	if h.IPProto == "" {
		h.IPProto = "tcp4"
	}

	if h.Hail == "" {
		h.Hail = DefaultHail
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
		bail(err)

		fmt.Fprintf(os.Stderr, "[%d] inbound connection accepted; starting SSH handshake...\n", id)
		c, chans, _, err := ssh.NewServerConn(socket, config)
		bail(err)

		go func(id int) {
			defer c.Close()
			ident := c.User()
			fmt.Fprintf(os.Stderr, "[%d] connected as user [%s]\n", id, ident)

			events := make(chan Message)
			fmt.Fprintf(os.Stderr, "[%d] registering agent (%s)\n", id, ident)
			err := h.register(ident, events)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%d] failed to register: %s\n", id, err)
				return
			}
			defer h.unregister(ident)

			for newch := range chans {
				if newch.ChannelType() != "session" {
					newch.Reject(ssh.UnknownChannelType, "buh-bye!")
				}

				fmt.Fprintf(os.Stderr, "[%d] accepting '%s' request and starting up channel...\n", id, newch.ChannelType())
				ch, reqs, err := newch.Accept()
				bail(err)

				for r := range reqs {
					fmt.Fprintf(os.Stderr, "[%d] request type '%s' received.\n", id, r.Type)

					if r.Type != "exec" {
						r.Reply(false, nil)
						continue
					}

					r.Reply(true, nil)
					fmt.Fprintf(os.Stderr, "[%d] extracting `exec' payload to determine if this is an ssher client...\n", id)

					var payload struct{ Value []byte }
					err = ssh.Unmarshal(r.Payload, &payload)
					bail(err)

					fmt.Fprintf(os.Stderr, "[%d] received `exec' payload of [%s]\n", id, string(payload.Value))
					fmt.Fprintf(os.Stderr, "[%d]         or in hexadecimal: [% 02x]\n", id, payload.Value)

					if string(payload.Value) != h.Hail {
						fmt.Fprintf(ch, "err: please say the magic word.\n")
						ch.SendRequest("exit-status", false, exited(1))
						break
					}

					fmt.Fprintf(ch, "info: welcome to the club, %s!\n", ident)
					fmt.Fprintf(os.Stderr, "[%d] agent handshake established; entering dispatch loop...\n", id)

					in := bufio.NewScanner(ch)

					for ev := range events {
						fmt.Fprintf(ch, "%s: %s\n", ev.Type, ev.Text())
						for in.Scan() {
							fmt.Printf("[%d] out | %s\n", id, in.Text())
						}
						fmt.Fprintf(os.Stderr, "[%d] sending exit-status 0...\n", id)
						ch.SendRequest("exit-status", false, exited(0))
						break
					}
				}

				fmt.Fprintf(os.Stderr, "[%d] closing connection...\n", id)
				ch.Close()
			}

			fmt.Fprintf(os.Stderr, "[%d] done with incoming channels.\n", id)
		}(id)
	}
}

func (h *Hub) Send(agent string, typ MessageType, payload string) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	if ch, ok := h.agents[agent]; ok {
		fmt.Fprintf(os.Stderr, "writing message to channel...\n")
		ch <- Message{
			Type: typ,
			data: []byte(payload),
		}
		fmt.Fprintf(os.Stderr, "WROTE message to channel...\n")
		return nil
	} else {
		return fmt.Errorf("no such agent (%s)", agent)
	}
}

func (h *Hub) register(name string, ch chan Message) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	if _, found := h.agents[name]; found {
		return fmt.Errorf("agent (%s) already registered", name)
	}

	h.agents[name] = ch
	return nil
}

func (h *Hub) unregister(name string) {
	h.lock.Lock()
	defer h.lock.Unlock()

	fmt.Fprintf(os.Stderr, "unregistering agent (%s)\n", name)
	if ch, found := h.agents[name]; found {
		delete(h.agents, name)
		close(ch)
	}
	fmt.Fprintf(os.Stderr, "done unregistering agent (%s)\n", name)
}
