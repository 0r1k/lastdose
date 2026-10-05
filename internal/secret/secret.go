// Package secret seals the final message and signs tracker state with one
// master key. Separate subkeys are derived for each purpose, so a value made
// for one purpose is useless for the other.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// KeySize is the master key length in bytes.
const KeySize = 32

const (
	labelMessage = "lastdose/final-message/v1"
	labelState   = "lastdose/state-signature/v1"
)

func derive(master []byte, label string) []byte {
	mac := hmac.New(sha256.New, master)
	mac.Write([]byte(label))
	return mac.Sum(nil)
}

// NewKey returns a fresh random master key.
func NewKey() ([]byte, error) {
	k := make([]byte, KeySize)
	_, err := rand.Read(k)
	return k, err
}

func aead(master []byte) (cipher.AEAD, error) {
	if len(master) != KeySize {
		return nil, errors.New("secret: bad key size")
	}
	block, err := aes.NewCipher(derive(master, labelMessage))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Seal encrypts plaintext with AES-256-GCM; the nonce is prepended.
func Seal(master, plaintext []byte) ([]byte, error) {
	g, err := aead(master)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plaintext, []byte(labelMessage)), nil
}

// Open decrypts what Seal produced.
func Open(master, sealed []byte) ([]byte, error) {
	g, err := aead(master)
	if err != nil {
		return nil, err
	}
	if len(sealed) < g.NonceSize() {
		return nil, errors.New("secret: sealed data too short")
	}
	nonce, ct := sealed[:g.NonceSize()], sealed[g.NonceSize():]
	return g.Open(nil, nonce, ct, []byte(labelMessage))
}

// Sign returns a hex HMAC of data.
func Sign(master []byte, data string) string {
	mac := hmac.New(sha256.New, derive(master, labelState))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a signature made by Sign in constant time.
func Verify(master []byte, data, sig string) bool {
	want, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, derive(master, labelState))
	mac.Write([]byte(data))
	return hmac.Equal(mac.Sum(nil), want)
}
