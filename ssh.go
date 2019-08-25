package sfab

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

func withAuthKeys(path string, fn func (string, ssh.PublicKey)) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	for {
		key, user, _, rest, err := ssh.ParseAuthorizedKey(b)
		if err != nil {
			// ran out of keys...
			break
		}

		fn(user, key)
		b = rest
	}

	return nil
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
