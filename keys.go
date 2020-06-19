package sfab

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"

	"golang.org/x/crypto/ssh"
)

type PrivateKey struct {
	key    *rsa.PrivateKey
	signer ssh.Signer
}

func (k PrivateKey) PublicKey() PublicKey {
	return PublicKey{
		pub: k.signer.PublicKey(),
	}
}

func (k PrivateKey) Encode() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(k.key),
	})
}

func (k PrivateKey) EncodeString() string {
	return string(k.Encode())
}

type PublicKey struct {
	pub ssh.PublicKey
}

func (k PublicKey) Encode() []byte {
	return k.pub.Marshal()
}

func (k PublicKey) EncodeString() string {
	return string(k.Encode())
}

// GeneratePrivateKey create a new private (RSA) key,
// and returns it as a PrivateKey.
//
func GeneratePrivateKey(bits int) (*PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}

	// Validate Private Key
	err = key.Validate()
	if err != nil {
		return nil, err
	}

	// Fashion a signer that is acceptable to crypto/ssh
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, err
	}

	return &PrivateKey{
		key:    key,
		signer: signer,
	}, nil
}

// PrivateKeyFromFile reads the given file, parses a single
// private key (in PEM format) from it, and returns that.
//
func PrivateKeyFromFile(path string) (*PrivateKey, error) {
	// Read the whole file into memory
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return PrivateKeyFromBytes(b)
}

// PrivateKeyFromBytes parses a single private key (in PEM
// format) from the passed byte slice, and returns it.
//
func PrivateKeyFromBytes(b []byte) (*PrivateKey, error) {
	// Parse PEM into its binary representation (DER / ASN.1)
	// (ignoring any follow-on PEM blocks...)
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("unrecognized private key format: no pem blocks found")
	}

	// Parse our (now DER / ASN.1) key according to PKCS#1
	// (which is *only* for RSA private keys!)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	// Fashion a signer that is acceptable to crypto/ssh
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, err
	}

	return &PrivateKey{
		key:    key,
		signer: signer,
	}, nil
}

// PrivateKeyFromString parses a single private key (in PEM
// format) from the passed string, and returns it.
//
func PrivateKeyFromString(s string) (*PrivateKey, error) {
	return PrivateKeyFromBytes([]byte(s))
}

// PublicKeyFromPrivateKeyFile reads the given file, parses a
// single private key (in PEM format) from it, extracts the
// public key from it, and returns that.
//
func PublicKeyFromPrivateKeyFile(path string) (*PublicKey, error) {
	key, err := PrivateKeyFromFile(path)
	if err != nil {
		return nil, err
	}
	return &PublicKey{
		pub: key.signer.PublicKey(),
	}, nil
}

// PublicKeyFromPrivateKeyBytes parses a single private key
// (in PEM format) from the passed byte slice, extracts the
// public key from it, and returns it.
//
func PublicKeyFromPrivateKeyBytes(b []byte) (*PublicKey, error) {
	key, err := PrivateKeyFromBytes(b)
	if err != nil {
		return nil, err
	}
	return &PublicKey{
		pub: key.signer.PublicKey(),
	}, nil
}

// PublicKeyFromPrivateKeyString parses a single private key
// (in PEM format) from the passed string, extracts the public
// key from it, and returns it.
//
func PublicKeyFromPrivateKeyString(s string) (*PublicKey, error) {
	key, err := PrivateKeyFromString(s)
	if err != nil {
		return nil, err
	}
	return &PublicKey{
		pub: key.signer.PublicKey(),
	}, nil
}

func parsePublicKey(b []byte) (*PublicKey, error) {
	k, _, _, _, err := ssh.ParseAuthorizedKey(b)
	if err != nil {
		return nil, err
	}
	return &PublicKey{
		pub: k,
	}, nil
}

// PublicKeyFromFile reads the given file, parses a public key
// from it (in sshd(8) authorized_keys format), and returns it.
//
func PublicKeyFromFile(path string) (*PublicKey, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return parsePublicKey(b)
}

// PublicKeyFromBytes parses a public key (in sshd(8) authorized_keys
// format), from the passed byte slice, and returns it.
//
func PublicKeyFromBytes(b []byte) (*PublicKey, error) {
	return parsePublicKey(b)
}

// PublicKeyFromString parses a public key (in sshd(8) authorized_keys
// format), from the passed string, and returns it.
//
func PublicKeyFromString(s string) (*PublicKey, error) {
	return parsePublicKey([]byte(s))
}
