package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"

	"github.com/tucats/ego/errors"
)

// Encrypt encrypts a string using a password.
func Encrypt(data, password string) (string, *errors.EgoError) {
	b, err := encrypt([]byte(data), password)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Decrypt decrypts a string using a password.
func Decrypt(data, password string) (string, *errors.EgoError) {
	b, err := decrypt([]byte(data), password)
	if err != nil {
		return "", err
	}

	return string(b), err
}

// Hash creates a hash string from a key string. The hash cannot be
// reversed back into the key string, but two instances of the same
// key string result in the same hash value.
func Hash(key string) string {
	hasher := md5.New()
	_, _ = hasher.Write([]byte(key))

	return hex.EncodeToString(hasher.Sum(nil))
}

func encrypt(data []byte, passphrase string) ([]byte, *errors.EgoError) {
	block, _ := aes.NewCipher([]byte(Hash(passphrase)))

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New(err)
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.New(err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return ciphertext, nil
}

func decrypt(data []byte, passphrase string) ([]byte, *errors.EgoError) {
	key := []byte(Hash(passphrase))
	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, errors.New(err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New(err)
	}

	nonceSize := gcm.NonceSize()
	if nonceSize > len(data) {
		return []byte(""), nil
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New(err)
	}

	return plaintext, nil
}
