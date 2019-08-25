package sfab

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

type Handler func([]byte, io.Writer)

type Agent struct {
	Identity string

	PrivateKeyFile string

	Timeout time.Duration
}

func (a *Agent) Connect(proto, host string, handler Handler) error {
	if a.PrivateKeyFile == "" {
		return fmt.Errorf("missing PrivateKeyFile in Agent object.")
	}

	key, err := loadPrivateKey(a.PrivateKeyFile)
	if err != nil {
		return err
	}

	config := &ssh.ClientConfig{
		User:            a.Identity,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(key)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), /* FIXME */
		Timeout:         a.Timeout,
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
		fmt.Fprintf(os.Stderr, "inbound channel type '%s' from server...\n", newch.ChannelType())

		if newch.ChannelType() != "session" {
			newch.Reject(ssh.UnknownChannelType, "buh-bye!")
		}

		fmt.Fprintf(os.Stderr, "accepting '%s' request and starting up channel...\n", newch.ChannelType())
		ch, reqs, err := newch.Accept()
		if err != nil {
			return err
		}

		for r := range reqs {
			fmt.Fprintf(os.Stderr, "request type '%s' received.\n", r.Type)

			if r.Type != "exec" {
				r.Reply(false, nil)
				continue
			}

			r.Reply(true, nil)
			var payload struct{ Value []byte }
			if err := ssh.Unmarshal(r.Payload, &payload); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "received `exec' payload of [%s]\n", string(payload.Value))
			handler(payload.Value, ch)
			ch.SendRequest("exit-status", false, exited(0))
			break
		}

		fmt.Fprintf(os.Stderr, "closing connection...\n")
		ch.Close()
	}
	return nil
}
