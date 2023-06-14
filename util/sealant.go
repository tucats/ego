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

func NewSealedString(sealedText string) SealedString {
	return SealedString(sealedText)
}

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

func (s SealedString) Unseal() string {
	b, _ := decrypt([]byte(s), ephemera)

	text := string(b)

	return text[7:]
}

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
func random() (string, error) {
	n := 32
	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		return "", errors.NewError(err)
	}

	return base64.URLEncoding.EncodeToString(b), nil
}
