package sfab

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/jhunt/go-log"
	"golang.org/x/crypto/ssh"
)

// DefaultTimeout will be used as a fallback, should an Agent not set
// its Timeout attribute to a non-zero connect timeout.
//
const DefaultTimeout time.Duration = 30 * time.Second

// A Handler is the primary workhorse of the Hub + Agent distributed
// orchestration engine.  Each Handler will be passed the opaque message
// payload from the Hub as its first argument (a slice of bytes, arbitrarily
// long), and an output stream to write responses / output to.
//
type Handler func([]byte, io.Writer)

// An Agent represents a client that connects to a Hub over SSH, and awaits
// instructions on what to do.  Each Agent has an identity (its name and
// private key).
//
type Agent struct {
	// Name of this agent, which will be sent to any Hub this Agent connects
	// to, and used to validate authorization (along with its private key).
	//
	Identity string

	// Path on the filesystem to the SSH Private Key to use for connecting
	// to upstream sFAB Hubs.
	//
	PrivateKeyFile string

	// How long to wait for an upstream Hub to connect.
	//
	Timeout time.Duration

	// A helper object for authorizing Hub host keys by name or IP.
	//
	keys *KeyMaster
}

// Instruct the Agent to (insecurely) accept any host key presented by the
// Hub, when connecting.  This is a terrible idea in production, but can
// be useful in development or debugging scenarios.
//
// Note: calling this function will obliterate any keys authorized by
// the AuthorizeKey() method.
//
func (a *Agent) AcceptAnyHostKey() {
	a.keys = nil
}

// Authorize a specific Hub Host Key, which will be accepted from any Hub
// with the name or IP address given as `host`.
//
func (a *Agent) AuthorizeKey(host string, key ssh.PublicKey) {
	if a.keys == nil {
		a.keys = &KeyMaster{}
	}
	a.keys.Authorize(key, host)
}

// Connect to a remote sFAB Hub, using the given protocol (i.e. "tcp4" or
// "tcp6"), and respond to execution requests with the passed Handler.
//
// This method will block, so if the caller wishes to do other work, this
// is best run in a goroutine.
//
func (a *Agent) Connect(proto, host string, handler Handler) error {
	if a.Identity == "" {
		return fmt.Errorf("missing Identity in Agent object.")
	}

	if a.PrivateKeyFile == "" {
		return fmt.Errorf("missing PrivateKeyFile in Agent object.")
	}

	if a.Timeout == 0 {
		a.Timeout = DefaultTimeout
	}

	key, err := loadPrivateKey(a.PrivateKeyFile)
	if err != nil {
		return err
	}

	config := &ssh.ClientConfig{
		User:    a.Identity,
		Auth:    []ssh.AuthMethod{ssh.PublicKeys(key)},
		Timeout: a.Timeout,
	}
	if a.keys != nil {
		config.HostKeyCallback = a.keys.HostKeyCallback()
	} else {
		config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	socket, err := net.DialTimeout(proto, host, a.Timeout)
	if err != nil {
		return err
	}

	conn, chans, reqs, err := ssh.NewClientConn(socket, host, config)
	if err != nil {
		return err
	}
	defer conn.Close()

	go ignoreGlobalRequests(reqs)
	for newch := range chans {
		log.Debugf("[agent %s] inbound channel type '%s' from server...", a.Identity, newch.ChannelType())

		if newch.ChannelType() != "session" {
			newch.Reject(ssh.UnknownChannelType, "buh-bye!")
		}

		log.Debugf("[agent %s] accepting '%s' request and starting up channel...", a.Identity, newch.ChannelType())
		ch, reqs, err := newch.Accept()
		if err != nil {
			return err
		}

		for r := range reqs {
			log.Debugf("[agent %s] request type '%s' received.", a.Identity, r.Type)

			if r.Type != "exec" {
				r.Reply(false, nil)
				continue
			}

			r.Reply(true, nil)
			var payload struct{ Value []byte }
			if err := ssh.Unmarshal(r.Payload, &payload); err != nil {
				return err
			}

			log.Debugf("[agent %s] received `exec' payload of [%s]", a.Identity, string(payload.Value))
			handler(payload.Value, ch)
			ch.SendRequest("exit-status", false, exited(0))
			break
		}

		log.Debugf("[agent %s] closing connection...", a.Identity)
		ch.Close()
	}
	return nil
}
