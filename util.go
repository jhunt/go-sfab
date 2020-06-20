package sfab

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/crypto/ssh"
)

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
