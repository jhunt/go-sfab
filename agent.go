package main

import (
	"os"
	"bufio"
	"fmt"
	"time"
	"io"

	"golang.org/x/crypto/ssh"
)

type Handler func (Message, io.Writer) (bool, error)

type Agent struct {
	Hail string

	Identity string

	PrivateKeyFile string

	Timeout time.Duration
}

func (a *Agent) Connect(proto, host string, handler Handler) error {
	if a.PrivateKeyFile == "" {
		return fmt.Errorf("missing PrivateKeyFile in Agent object.")
	}

	if a.Hail == "" {
		a.Hail = DefaultHail
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

	conn, err := ssh.Dial(proto, host, config)
	if err != nil {
		return err
	}
	defer conn.Close()

	for {
		err = a.session(conn, handler)
		if err != nil {
			return err
		}
	}
}

func (a *Agent) session(conn *ssh.Client, handler Handler) error {
	session, err := conn.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	err = session.Start(DefaultHail)
	if err != nil {
		return err
	}

	down := bufio.NewScanner(stdout)
	for down.Scan() {
		msg, err := ParseMessage(down.Text())
		if err != nil {
			fmt.Fprintf(os.Stderr, "unrecognized server message:\n  [%s]\n  %s\n", down.Text(), err)
			continue
		}

		proceed, err := handler(msg, stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to handle message: %s\n", err)
		}
		if !proceed {
			stdin.Close()
			break
		}
	}
	fmt.Fprintf(os.Stderr, "tearing down session...\n")
	return nil
}
