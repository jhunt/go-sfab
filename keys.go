package sfab

import (
	"crypto/rand"
	"crypto/rsa"
	"io/ioutil"

	"golang.org/x/crypto/ssh"
)

type PrivateKey interface {
	ssh.Signer
}

type PublicKey interface {
	ssh.PublicKey
}

// GeneratePrivateKey create a new private (RSA) key,
// and returns it as a PrivateKey.
//
func GeneratePrivateKey(bits int) (PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = key.Validate()
	if err != nil {
		return nil, err
	}

	return ssh.NewSignerFromKey(key)
}

// PrivateKeyFromFile reads the given file, parses a single
// private key (in PEM format) from it, and returns that.
//
func PrivateKeyFromFile(path string) (PrivateKey, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ssh.ParsePrivateKey(b)
}

// PrivateKeyFromBytes parses a single private key (in PEM
// format) from the passed byte slice, and returns it.
//
func PrivateKeyFromBytes(pem []byte) (PrivateKey, error) {
	return ssh.ParsePrivateKey(pem)
}

// PrivateKeyFromString parses a single private key (in PEM
// format) from the passed string, and returns it.
//
func PrivateKeyFromString(pem string) (PrivateKey, error) {
	return ssh.ParsePrivateKey([]byte(pem))
}

// PublicKeyFromPrivateKeyFile reads the given file, parses a
// single private key (in PEM format) from it, extracts the
// public key from it, and returns that.
//
func PublicKeyFromPrivateKeyFile(path string) (PublicKey, error) {
	key, err := PrivateKeyFromFile(path)
	if err != nil {
		return nil, err
	}
	return key.PublicKey(), nil
}

// PublicKeyFromPrivateKeyBytes parses a single private key
// (in PEM format) from the passed byte slice, extracts the
// public key from it, and returns it.
//
func PublicKeyFromPrivateKeyBytes(pem []byte) (PublicKey, error) {
	key, err := PrivateKeyFromBytes(pem)
	if err != nil {
		return nil, err
	}
	return key.PublicKey(), nil
}

// PublicKeyFromPrivateKeyString parses a single private key
// (in PEM format) from the passed string, extracts the public
// key from it, and returns it.
//
func PublicKeyFromPrivateKeyString(pem string) (PublicKey, error) {
	key, err := PrivateKeyFromString(pem)
	if err != nil {
		return nil, err
	}
	return key.PublicKey(), nil
}

func parsePublicKey(b []byte) (PublicKey, error) {
	k, _, _, _, err := ssh.ParseAuthorizedKey(b)
	return k, err
}

// PublicKeyFromFile reads the given file, parses a public key
// from it (in sshd(8) authorized_keys format), and returns it.
//
func PublicKeyFromFile(path string) (PublicKey, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return parsePublicKey(b)
}

// PublicKeyFromBytes parses a public key (in sshd(8) authorized_keys
// format), from the passed byte slice, and returns it.
//
func PublicKeyFromBytes(b []byte) (PublicKey, error) {
	return parsePublicKey(b)
}

// PublicKeyFromString parses a public key (in sshd(8) authorized_keys
// format), from the passed string, and returns it.
//
func PublicKeyFromString(s string) (PublicKey, error) {
	return parsePublicKey([]byte(s))
}
