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

type Key struct {
	private *rsa.PrivateKey
	signer  ssh.Signer

	public *rsa.PublicKey
	sshpub ssh.PublicKey
}

func (k Key) Fingerprint() string {
	return ssh.FingerprintSHA256(k.sshpub)
}

func (k Key) Private() *Key {
	return &Key{
		private: k.private,
		signer:  k.signer,
	}
}

func (k Key) Public() *Key {
	return &Key{
		public: k.public,
		sshpub: k.sshpub,
	}
}

func wrap(private *rsa.PrivateKey, public *rsa.PublicKey) (*Key, error) {
	k := &Key{}

	if private != nil {
		signer, err := ssh.NewSignerFromKey(private)
		if err != nil {
			return nil, err
		}

		k.private = private
		k.signer = signer

		if public == nil {
			public = private.Public().(*rsa.PublicKey)
		}
	}

	if public != nil {
		sshpub, err := ssh.NewPublicKey(public)
		if err != nil {
			return k, err
		}

		k.public = public
		k.sshpub = sshpub
	}

	return k, nil
}

func GenerateKey(bits int) (*Key, error) {
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}

	if err = key.Validate(); err != nil {
		return nil, err
	}

	return wrap(key, nil)
}

func ParseKey(b []byte) (*Key, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("unrecognized key format: no pem blocks found")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return wrap(key, nil)

	case "RSA PUBLIC KEY":
		key, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return wrap(nil, key)

	default:
		return nil, fmt.Errorf("unrecognized key type '%s'", block.Type)
	}
}

func ParseKeyFromString(s string) (*Key, error) {
	return ParseKey([]byte(s))
}

func ParseKeyFromFile(f string) (*Key, error) {
	b, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, err
	}
	return ParseKey(b)
}

func (k Key) IsPrivateKey() bool {
	return k.private != nil
}

func (k Key) IsPublicKey() bool {
	return k.public != nil
}

func (k Key) Encode() []byte {
	if k.private != nil {
		return pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(k.private),
		})
	}

	if k.public != nil {
		return pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(k.public),
		})
	}

	return nil
}

func (k Key) EncodeString() string {
	return string(k.Encode())
}
