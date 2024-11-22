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

// NewSealedString creates a new sealed string from the given text.
// If the sealer is not yet initialized, then it is initialized with
// a random seed value.
func NewSealedString(sealedText string) SealedString {
	return SealedString(sealedText)
}

// Seal encrypts the given text using the ephemera seed value.
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

// Unseal decrypts the sealed text using the ephemera seed value.
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

// random generates a random seed value as a string.
func random() (string, error) {
	n := 32
	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		return "", errors.New(err)
	}

	return base64.URLEncoding.EncodeToString(b), nil
}
