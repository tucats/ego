package util

import (
	"crypto/rand"
	"encoding/base64"
	"sync"

	"github.com/tucats/ego/errors"
)

var ephemera string
var lock sync.Mutex

type SealedString string

func Seal(text string) SealedString {
	lock.Lock()

	if ephemera == "" {
		ephemera, _ = random()
	}

	lock.Unlock()

	b, _ := encrypt([]byte(text), ephemera)

	return SealedString(string(b))
}

func (s SealedString) Unseal() string {
	b, _ := decrypt([]byte(s), ephemera)

	return string(b)
}

func random() (string, error) {
	n := 32
	b := make([]byte, n)

	if _, err := rand.Read(b); err != nil {
		return "", errors.NewError(err)
	}

	return base64.URLEncoding.EncodeToString(b), nil
}
