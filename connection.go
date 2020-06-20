package sfab

import (
	"time"

	"github.com/jhunt/go-log"
	"golang.org/x/crypto/ssh"
)

// A connection tracks a single TCP connection,. over which we
// are speaking the SSH protocol and multiplexing (potentially)
// several sessions and exec requests.
//
type connection struct {
	// The underlying SSH server connection
	//
	ssh *ssh.ServerConn

	// The input message channel, which is directly used by
	// the Hub Send() method to communicate to the goroutine
	// that is servicing this connection.
	//
	messages chan Message

	// A "termination" channel that is used to ensure that we
	// can properly hang up the channel (and disconnect the
	// underlying TCP connection) in the face of a variety of
	// failures (broken pipes, errors, protocol refusal, etc.)
	//
	hangup chan int
	hungup bool

	// Reapers are goroutines (represented by their messaging endpoints)
	// that have subscribed to the keepalive goroutine, and are interested
	// in knowing _when_ the underlying connection is closed, EOFs, etc.
	//
	reapers []chan int

	// A "cleanup" function to call when we shut down this
	// connection.  Primarily used to unregister ourselves
	// from the Hub object that is tracking us.
	//
	done func()

	// The SSH (RSA) Public Key that the agent used in the authentication
	// phase of the underlying SSH protocol transport connection handshake.
	// We keep this on-hand so that we can authorize and deauthorize via
	// the Hub's KeyMaster as needed.
	//
	key *PublicKey

	// The identity (user@domain) that the agent used in the authentication
	// phase of the underlying SSH protocol transport connection handshake.
	// We keep this on-hand so that we can authorize and deauthorize via
	// the Hub's KeyMaster as needed, and for finding agents by name,
	// to send them work to do.
	//
	identity string
}

// Signal to the machinery of the connection object that it
// is time to close the underlying TCP connection and start
// shutting things down.
//
// The heavy lifting of this action is taken care of in the
// monitor() method, which will close the SSH channel, inform
// the reapers of our intent to shut down, and call our
// cleanup handler to deregister us from the Hub that created
// us.
//
// This method is idempotent - calling it multiple times (so
// long as they do not interleave) is safe.  Only the first
// such call will have any effect.
//
func (c *connection) Hangup() {
	if !c.hungup {
		c.hangup <- 0
		close(c.hangup)
		c.hungup = false
	}
}

// Monitor the overall health of the underlying TCP connection,
// by sending keepalives at the given interval.  If any of the
// keepalives fail, this function calls Hangup(), which bounces
// back through this function again to do the actual connection
// teardown.
//
// This method is meant to be called in a goroutine.
//
func (c *connection) monitor(t time.Duration) {
	tick := time.NewTicker(t)
	for {
		select {
		case <-tick.C:
			_, _, err := c.ssh.SendRequest("keepalive", true, nil)
			if err != nil {
				log.Errorf("[hub] unable to send keepalive message to '%s': %s", c.identity, err)
				tick.Stop()
				c.Hangup()
			}

		case <-c.hangup:
			c.done()
			c.ssh.Close()

			for _, reaper := range c.reapers {
				reaper <- 1
				close(reaper)
			}
			return
		}
	}
}

// Service a nailed up SSH connection from an authenticated
// (and registered) Agent.  Here's what that entails:
//
//   1. Global Requests on the SSH channel are IGNORED.
//      (this means keepalives too!)
//
//   2. Requests for new channels (usually sessions) are
//      rejected; the way sFAB works, we (the server) do
//      all the channel opening, when it suits our agenda.
//
//   3. Keepalives are sent to the remote end (as global
//      requests) according to the given time interval.
//
//   4. The _messages_ channel of the connection is serviced.
//      Any messages received are attempted as "exec"
//      requests, in their own SSH sessions.
//
//      This step is currently _serial_.
//
//      If a message cannot be executed on the registered
//      Agent, the underlying TCP connection will be closed
//      forcefully, to avoid further loss.
//
func (c *connection) Serve(chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request, t time.Duration) {
	go ignoreGlobalRequests(reqs)
	go ignoreNewChannels(chans)
	go c.monitor(t)

	for msg := range c.messages {
		if err := c.run(msg); err != nil {
			c.Hangup()
			break
		}
	}
}

// Execute a single command on the remote Agent.
// To do this, we first set up a new SSH session, and
// then make an "exec" request through it.
//
// Output from the remote command, as well as its
// exit code / termination error are also sent back
// along the message's _responses_ channel.
//
func (c *connection) run(msg Message) error {
	channel, requests, err := c.ssh.OpenChannel("session", nil)
	if err != nil {
		return err
	}

	session := &session{
		connection: c,
		channel:    channel,
		requests:   requests,
		exit:       make(chan status),
	}
	go session.serviceRequests()

	if err = session.start(string(msg.payload)); err != nil {
		return err
	}

	reaper := make(chan int)
	c.reapers = append(c.reapers, reaper)

	session.finish(msg.responses, reaper)
	return nil
}
