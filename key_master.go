package sfab

import (
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

// A KeyMaster handles the specifics of tracking which SSH key pairs
// are acceptable for which subjects (either hostnames, IPs, or agent
// names).  It provides primitives for authorizing and deauthorizing
// these key pairs, and sports some helper methods for integrating with
// the rest of the x/crypto/ssh library.
//
type KeyMaster struct {
	// A map of Public Key -> Subject -> t, indicating
	// which keys we have authorized.
	//
	keys map[string] map[string] bool
}

// Authorize a key pair for one or more subjects (either hostnames,
// IP addresses, or agent names).
//
func (m *KeyMaster) Authorize(key ssh.PublicKey, subjects ...string) {
	k := fmt.Sprintf("%v", key.Marshal())

	if m.keys == nil {
		m.keys = make(map[string] map[string] bool)
	}
	if _, exists := m.keys[k]; !exists {
		m.keys[k] = make(map[string] bool)
	}

	for _, s := range subjects {
		m.keys[k][s] = true
	}
}

// Deauthorize a key pair for one or more subjects (either hostnames,
// IP addresses, or agent names).
//
func (m *KeyMaster) Deauthorize(key ssh.PublicKey, subjects ...string) {
	k := fmt.Sprintf("%v", key.Marshal())

	if m.keys == nil {
		m.keys = make(map[string] map[string] bool)
	}
	if _, exists := m.keys[k]; !exists {
		m.keys[k] = make(map[string] bool)
	}

	for _, s := range subjects {
		m.keys[k][s] = false
	}
}

// Checks whether or not a public key has been pre-authorized for a
// given subject (either a hostname, IP address, or agent name).
//
func (m *KeyMaster) Authorized(subject string, key ssh.PublicKey) bool {
	k := fmt.Sprintf("%v", key.Marshal())
	if m.keys == nil {
		return false
	}

	if _, ok := m.keys[k]; !ok {
		return false
	}
	t, ok := m.keys[k][subject]
	return t && ok
}

// Provide a callback function that can be used by SSH clients
// to whitelist authorized host keys during SSH connection negotiation.
//
func (m *KeyMaster) HostKeyCallback() ssh.HostKeyCallback {
	return func (hostname string, remote net.Addr, key ssh.PublicKey) error {
		if m.Authorized(hostname, key) {
			return nil
		}
		if m.Authorized(remote.String(), key) {
			return nil
		}
		return fmt.Errorf("unrecognized host key")
	}
}
