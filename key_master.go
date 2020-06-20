package sfab

import (
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

type disposition int

const (
	UnknownDisposition disposition = 0
	Authorized                     = 1
	NotAuthorized                  = 2

	PublicKeyExtensionName = "sfab-pubkey"
)

type authorization struct {
	disposition disposition
	publicKey   *Key
}

// A KeyMaster handles the specifics of tracking which SSH key pairs
// are acceptable for which subjects (either hostnames, IPs, or agent
// names).  It provides primitives for authorizing and deauthorizing
// these key pairs, and sports some helper methods for integrating with
// the rest of the x/crypto/ssh library.
//
type KeyMaster struct {
	// Whether or not the Key verifier should signal an error (and
	// disconnect a connecting agent) if the key is not authorized.
	//
	strict bool

	// A map of Fingerprint -> Subject -> authorization, indicating
	// which keys we have authorized, and tracking their original
	// (ssh) public keys.
	//
	keys map[string]map[string]*authorization
}

// Authorize a key pair for one or more subjects (either hostnames,
// IP addresses, or agent names).
//
func (m *KeyMaster) Authorize(key *Key, subjects ...string) {
	if key != nil {
		m.track(key, Authorized, subjects...)
	}
}

// Deauthorize a key pair for one or more subjects (either hostnames,
// IP addresses, or agent names).
//
func (m *KeyMaster) Deauthorize(key *Key, subjects ...string) {
	if key != nil {
		m.track(key, NotAuthorized, subjects...)
	}
}

func (m *KeyMaster) track(key *Key, disp disposition, subjects ...string) string {
	k := ssh.FingerprintSHA256(key.sshpub)

	if m.keys == nil {
		m.keys = make(map[string]map[string]*authorization)
	}
	if _, exists := m.keys[k]; !exists {
		m.keys[k] = make(map[string]*authorization)
	}

	for _, s := range subjects {
		if _, exists := m.keys[k][s]; !exists {
			m.keys[k][s] = &authorization{
				disposition: UnknownDisposition,
				publicKey:   key,
			}
		}
		if disp != UnknownDisposition {
			m.keys[k][s].disposition = disp
		}
	}

	return k
}

// Checks whether or not a public key has been pre-authorized for a
// given subject (either a hostname, IP address, or agent name).
//
func (m *KeyMaster) Authorized(subject string, key *Key) bool {
	if key == nil {
		return false
	}
	return m.authorized(subject, key.sshpub)
}

func (m *KeyMaster) authorized(subject string, key ssh.PublicKey) bool {
	k := fmt.Sprintf("%v", ssh.FingerprintSHA256(key))

	if m.keys == nil {
		return false
	}

	if _, ok := m.keys[k]; !ok {
		return false
	}
	v, ok := m.keys[k][subject]
	return ok && v.disposition == Authorized
}

// Provide a callback function that can be used by SSH servers
// to whitelist authorized user keys during SSH connection netotiation.
//
func (m *KeyMaster) userKeyCallback() func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
	return func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		var err error
		pubkey := m.track(&Key{sshpub: key}, UnknownDisposition, c.User())

		if m.strict && !m.authorized(c.User(), key) {
			err = fmt.Errorf("unknown or unauthorized agent key")
		}

		return &ssh.Permissions{
			Extensions: map[string]string{
				PublicKeyExtensionName: pubkey,
			},
		}, err
	}
}

// Provide a callback function that can be used by SSH clients
// to whitelist authorized host keys during SSH connection negotiation.
//
func (m *KeyMaster) hostKeyCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if m.authorized(hostname, key) {
			return nil
		}
		if m.authorized(remote.String(), key) {
			return nil
		}
		if m.authorized("*", key) {
			return nil
		}
		return fmt.Errorf("unrecognized host key")
	}
}

func (m *KeyMaster) publicKeyUsed(conn *ssh.ServerConn) *Key {
	k := conn.Permissions.Extensions[PublicKeyExtensionName]
	if _, exists := m.keys[k]; !exists {
		return nil
	}
	if v, exists := m.keys[k][conn.User()]; exists {
		return v.publicKey
	}
	return nil
}

type Authorization struct {
	PublicKey      *Key
	Identity       string
	KeyFingerprint string
	Authorized     bool
	Known          bool
}

func (m KeyMaster) Authorizations() []Authorization {
	var l []Authorization

	for k := range m.keys {
		for s, authz := range m.keys[k] {
			l = append(l, Authorization{
				PublicKey:      authz.publicKey,
				Identity:       s,
				KeyFingerprint: k,
				Authorized:     authz.disposition == Authorized,
				Known:          authz.disposition != UnknownDisposition,
			})
		}
	}

	return l
}
