package main

import (
	"time"
	"bufio"
	"encoding/binary"
	"os"
	"io/ioutil"
	"bytes"
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

func Server(args []string) {
	addr := "127.0.0.1:4771"
	hostKeyFile := "host_key"
	authKeysFile := "id_rsa.pub"

	fmt.Fprintf(os.Stderr, "loading SSH host key from '%s'...\n", hostKeyFile)
	hostKey, err := loadHostKey(hostKeyFile)
	bail(err)

	fmt.Fprintf(os.Stderr, "loading SSH authorized keys from '%s'...\n", authKeysFile)
	authKeys, err := loadAuthKeys(authKeysFile)
	bail(err)

	fmt.Fprintf(os.Stderr, "configuring SSH server parameters...\n")
	config, err := configureSSHServer(hostKey, authKeys)
	bail(err)

	fmt.Fprintf(os.Stderr, "binding listening socket on %s...\n", addr)

	listener, err := net.Listen("tcp4", addr)
	bail(err)

	id := 0
	for {
		fmt.Fprintf(os.Stderr, "awaiting connection...\n")

		id += 1
		socket, err := listener.Accept()
		bail(err)

		fmt.Fprintf(os.Stderr, "[%d] inbound connection accepted; starting SSH handshake...\n", id)
		c, chans, _, err := ssh.NewServerConn(socket, config)
		bail(err)

		func(id int) {
			defer c.Close()

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

					var payload struct { Value []byte }
					err = ssh.Unmarshal(r.Payload, &payload)
					bail(err)

					fmt.Fprintf(os.Stderr, "[%d] received `exec' payload of [%s]\n", id, string(payload.Value))
					fmt.Fprintf(os.Stderr, "[%d]         or in hexadecimal: [% 02x]\n", id, payload.Value)

					if string(payload.Value) != "im-an-agent-and-im-ok" {
						fmt.Fprintf(ch, "err: please say the magic word.\n")
						ch.SendRequest("exit-status", false, exited(1))
						break
					}

					fmt.Fprintf(ch, "ok: welcome to the club!\n")
					fmt.Fprintf(os.Stderr, "[%d] agent handshake established; entering dispatch loop...\n", id)

					time.Sleep(5 * time.Second)
					in := bufio.NewScanner(ch)
					fmt.Fprintf(os.Stderr, "[%d] simulating out-of-band reverse execution...\n", id)
					fmt.Fprintf(ch, "run: {\"some\":\"json\"}\n")
					for in.Scan() {
						fmt.Printf("out | %s\n", in.Text())
					}
					ch.SendRequest("exit-status", false, exited(0))
				}

				fmt.Fprintf(os.Stderr, "[%d] closing connection...\n", id)
				ch.Close()
			}
		}(id)
	}
}

func loadHostKey(path string) (ssh.Signer, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ssh.ParsePrivateKey(b)
}

func loadAuthKeys(path string) ([]ssh.PublicKey, error) {
	var keys []ssh.PublicKey

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	for {
		key, _, _, rest, err := ssh.ParseAuthorizedKey(b)
		if err != nil {
			break
		}

		keys = append(keys, key)
		b = rest
	}

	return keys, nil
}

func configureSSHServer(hostKey ssh.Signer, authKeys []ssh.PublicKey) (*ssh.ServerConfig, error) {
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
	return config, nil
}

func exited(rc int) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(rc))
	return b
}
