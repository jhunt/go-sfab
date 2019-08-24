package main

import (
	"encoding/json"
	"strings"
	"bufio"
	"fmt"
	"time"
	"os"
	"io/ioutil"

	"golang.org/x/crypto/ssh"
)

func Client(args []string) {
	timeout := 30
	privKeyFile := "id_rsa"
	host := "127.0.0.1:4771"

	fmt.Fprintf(os.Stderr, "loading SSH client private key from '%s'...\n", privKeyFile)
	key, err := loadPrivateKey(privKeyFile)
	bail(err)

	fmt.Fprintf(os.Stderr, "configuring SSH client parameters...\n")
	config, err := configureSSHClient(key, timeout)
	bail(err)

	fmt.Fprintf(os.Stderr, "connecting to remote SSH server at '%s'...\n", host)
	conn, err := ssh.Dial("tcp4", host, config)
	bail(err)
	defer conn.Close()

	fmt.Fprintf(os.Stderr, "entering dispatch loop...\n")
	for {
		func () {
			fmt.Fprintf(os.Stderr, "starting new session...\n")
			session, err := conn.NewSession()
			bail(err)
			defer session.Close()

			stdout, err := session.StdoutPipe()
			bail(err)
			stdin, err := session.StdinPipe()
			bail(err)

			fmt.Fprintf(os.Stderr, "issuing an `exec' request across the session...\n")
			err = session.Start("im-an-agent-and-im-ok")
			bail(err)

			in := bufio.NewScanner(stdout)
			for in.Scan() {
				msg := in.Text()

				if strings.HasPrefix(msg, "ok: ") {
					fmt.Fprintf(os.Stderr, "server replied | %s.\n", msg)

				} else if strings.HasPrefix(msg, "run: ") {
					msg = strings.TrimPrefix(msg, "run: ")
					fmt.Fprintf(os.Stderr, "server requested we run something:\n")
					fmt.Fprintf(os.Stderr, "  | %s |\n", msg)

					var x map[string] interface{}
					err = json.Unmarshal([]byte(msg), &x)
					if err != nil {
						fmt.Fprintf(stdin, "ERROR: %s\n", err)
						break
					}

					b, err := json.MarshalIndent(x, "json >> ", " . ")
					if err != nil {
						fmt.Fprintf(stdin, "ERROR: %s\n", err)
						break
					}

					output := func(f string, args ...interface{}) {
						fmt.Fprintf(stdin, f, args...)
						fmt.Fprintf(os.Stderr, f, args...)
					}
					output("-----------------[ output ]-------------\n")
					output("json >> %s\n", string(b))
					output("-----------------[ ====== ]-------------\n\n")
					break

				} else {
					fmt.Fprintf(os.Stderr, "unrecognized server message: [%s]\n", msg)
				}
			}
			fmt.Fprintf(os.Stderr, "tearing down session...\n")
		}()
	}
	fmt.Fprintf(os.Stderr, "shutting down...\n")
}

func loadPrivateKey(path string) (ssh.Signer, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ssh.ParsePrivateKey(b)
}

func configureSSHClient(key ssh.Signer, timeout int) (*ssh.ClientConfig, error) {
	return &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(key)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(timeout) * time.Second,
	}, nil
}
