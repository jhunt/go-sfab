package main

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"

	"golang.org/x/crypto/ssh"
)

func loadPrivateKey(path string) (ssh.Signer, error) {
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

func ignoreNewChannels(in <-chan ssh.NewChannel) {
	for ch := range in {
		ch.Reject(ssh.Prohibited, fmt.Sprintf("read my lips -- no new %s channels", ch.ChannelType()))
	}
}

func ignoreGlobalRequests(ch <-chan *ssh.Request) {
	for r := range ch {
		r.Reply(false, nil)
	}
}

func exited(rc int) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(rc))
	return b
}
