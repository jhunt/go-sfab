package sfab

import (
	"io/ioutil"

	"golang.org/x/crypto/ssh"
)

// PrivateKeyFromFile reads the given file, parses a single
// private key (in PEM format) from it, and returns that.
//
func PrivateKeyFromFile(path string) (ssh.Signer, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ssh.ParsePrivateKey(b)
}

// PrivateKeyFromFile reads the given file, parses a single
// private key (in PEM format) from it, extracts the publici
// key from it, and returns that.
//
func PublicKeyFromFile(path string) (ssh.PublicKey, error) {
	key, err := PrivateKeyFromFile(path)
	if err != nil {
		return nil, err
	}
	return key.PublicKey(), nil
}

// PrivateKeyFromBytes parses a single private key (in PEM
// format) from the passed byte slice, and returns it.
//
func PrivateKeyFromBytes(pem []byte) (ssh.Signer, error) {
	return ssh.ParsePrivateKey(pem)
}

// PublicKeyFromBytes parses a single private key (in PEM
// format) from the passed byte slice, extracts the public
// key from it, and returns it.
//
func PublicKeyFromBytes(pem []byte) (ssh.PublicKey, error) {
	key, err := PrivateKeyFromBytes(pem)
	if err != nil {
		return nil, err
	}
	return key.PublicKey(), nil
}

// PrivateKeyFromBytes parses a single private key (in PEM
// format) from the passed string, and returns it.
//
func PrivateKeyFromString(pem string) (ssh.Signer, error) {
	return ssh.ParsePrivateKey([]byte(pem))
}

// PublicKeyFromBytes parses a single private key (in PEM
// format) from the passed string, extracts the public key
// from it, and returns it.
//
func PublicKeyFromString(pem string) (ssh.PublicKey, error) {
	key, err := PrivateKeyFromString(pem)
	if err != nil {
		return nil, err
	}
	return key.PublicKey(), nil
}
