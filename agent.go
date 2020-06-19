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
// orchestration engine.
//
// Each Handler will be passed the opaque message payload from the Hub as
// its first argument (a slice of bytes, arbitrarily long), and two output
// streams: one for standard output and the other for standard error.
//
// A Handler function returns two values: a Unix-style integer exit code,
// and an error that (if non-nil) will terminate the Agent's main loop.
//
type Handler func(payload []byte, stdout io.Writer, stderr io.Writer) (rc int, err error)

// An Agent represents a client that connects to a Hub over SSH, and awaits
// instructions on what to do.  Each Agent has an identity (its name and
// private key).
//
type Agent struct {
	// Name of this agent, which will be sent to any Hub this Agent connects
	// to, and used to validate authorization (along with its private key).
	//
	Identity string

	// Private Key to use for connecting to upstream sFAB Hubs.
	//
	PrivateKey *PrivateKey

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

	if a.PrivateKey == nil {
		return fmt.Errorf("missing PrivateKey in Agent object.")
	}

	if a.Timeout == 0 {
		a.Timeout = DefaultTimeout
	}

	config := &ssh.ClientConfig{
		User:    a.Identity,
		Auth:    []ssh.AuthMethod{ssh.PublicKeys(a.PrivateKey.signer)},
		Timeout: a.Timeout,
	}
	if a.keys != nil {
		config.HostKeyCallback = a.keys.HostKeyCallback()
	} else {
		config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	log.Debugf("[agent %s] connecting to %s over %s (for up to %fs)...", a.Identity, host, proto, a.Timeout.Seconds())
	socket, err := net.DialTimeout(proto, host, a.Timeout)
	if err != nil {
		return err
	}

	log.Debugf("[agent %s] starting SSH transport negotiation with hub...", a.Identity)
	conn, chans, reqs, err := ssh.NewClientConn(socket, host, config)
	if err != nil {
		return err
	}
	defer conn.Close()

	log.Debugf("[agent %s] ignoring global requests (keepalives, mostly) from hub...", a.Identity)
	go ignoreGlobalRequests(reqs)

	log.Debugf("[agent %s] awaiting channel requests from hub...", a.Identity)
	for newch := range chans {
		log.Debugf("[agent %s] inbound channel type '%s' from hub...", a.Identity, newch.ChannelType())

		if newch.ChannelType() != "session" {
			newch.Reject(ssh.UnknownChannelType, "buh-bye!")
		}

		log.Debugf("[agent %s] accepting '%s' request and starting up channel...", a.Identity, newch.ChannelType())
		ch, reqs, err := newch.Accept()
		if err != nil {
			log.Errorf("[agent %s] failed to accept new '%s' channel: %s", a.Identity, newch.ChannelType(), err)
			return nil
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
				log.Errorf("[agent %s] unable to unmarshal payload from upstream hub: %s", a.Identity, err)
				continue
			}

			log.Debugf("[agent %s] received `exec' payload of [%s]", a.Identity, string(payload.Value))
			rc, err := handler(payload.Value, ch, ch.Stderr())
			ch.SendRequest("exit-status", false, exited(rc))
			if err != nil {
				log.Errorf("[agent %s] handler returned error: %s", a.Identity, err)
				log.Errorf("[agent %s] terminating...", a.Identity)

				log.Debugf("[agent %s] closing connection...", a.Identity)
				ch.Close()
				return nil
			}

			break
		}

		log.Debugf("[agent %s] closing connection...", a.Identity)
		ch.Close()

		log.Debugf("[agent %s] awaiting channel requests from hub...", a.Identity)
	}
	return nil
}
