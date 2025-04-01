package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
)

// Encrypt encrypts a string using a password.
func Encrypt(data, password string) (string, error) {
	b, err := encrypt([]byte(data), password)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

// Decrypt decrypts a string using a password.
func Decrypt(data, password string) (string, error) {
	src, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	b, err := decrypt(src, password)
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

func encrypt(data []byte, passphrase string) ([]byte, error) {
	block, _ := aes.NewCipher([]byte(Hash(passphrase)))

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	cipherText := gcm.Seal(nonce, nonce, data, nil)

	return cipherText, nil
}

func decrypt(data []byte, passphrase string) ([]byte, error) {
	key := []byte(Hash(passphrase))

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if nonceSize > len(data) {
		return []byte(""), nil
	}

	nonce, cipherText := data[:nonceSize], data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
