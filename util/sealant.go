package util

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"sync"

	"github.com/tucats/ego/errors"
)

var ephemera string
var lock sync.Mutex

type SealedString string

// NewSealedString converts a simple text string into a sealed string
// object, that meets the sealed string interface requirements.
func NewSealedString(sealedText string) SealedString {
	return SealedString(sealedText)
}

// Seal encrypts the given text and returns it as a sealed string object.
// A sealed string is encrypted using an ephemeral password generated for
// each runtime instance of the Ego program. This means that an attempt to
// access runtime memory to find the value of a sealed string results in also
// needing to find the ephemera seed value. This is a way of storing data like
// passwords in memory in a more secure fashion.
func Seal(text string) SealedString {
	lock.Lock()

	if ephemera == "" {
		ephemera, _ = random()
	}

	lock.Unlock()

	salt := randomFragment(7)
	text = salt + text

	b, _ := encrypt([]byte(text), ephemera)

	return SealedString(string(b))
}

// Unseal decrypts the sealed text using the ephemera password value. If the
// string has been tampered with, the decryption will fail and return an
// empty string as the result along.
func (s SealedString) Unseal() string {
	b, _ := decrypt([]byte(s), ephemera)

	text := string(b)

	return text[7:]
}

// randomFragment generates a random string fragment used as part
// of the string sealing operation. The size value indicates the
// number of characters that must be in the resulting string.
func randomFragment(size int) string {
	n := 32
	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		return ""
	}

	text := hex.EncodeToString(b)
	if size > len(text) {
		size = len(text)
	}

	return text[:size]
}

// random generates a random seed value as a string. IT uses the system-level
// cryptographic random number generator for generating the seed. The result
// is encoded using base64 encoding for a more human-readable format.
func random() (string, error) {
	n := 32
	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		return "", errors.New(err)
	}

	return base64.URLEncoding.EncodeToString(b), nil
}
