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

// Encrypt encrypts a string using a password. The string is encrypted using AES-256-GCM
// encryption. The resulting string may contain non-printable characters.
func Encrypt(data, password string) (string, error) {
	b, err := encrypt([]byte(data), password)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Decrypt decrypts a string using a password. If the string has been tampered with
// or the password string is incorrect, the decryption will fail and return an
// empty string as the result along with an error. The decryption uses the same
// AES-256-GCM encryption used for encryption.
func Decrypt(data, password string) (string, error) {
	b, err := decrypt([]byte(data), password)
	if err != nil {
		return "", err
	}

	return string(b), err
}

// Hash creates a hash string from a key string. The hash cannot be
// reversed back into the key string, but two instances of the same
// key string result in the same hash value. The hash function used is MD5
// for the underlying message digest function, and the resulting hash value
// is encoded as a hexadecimal string.
func Hash(key string) string {
	hasher := md5.New()
	_, _ = hasher.Write([]byte(key))

	return hex.EncodeToString(hasher.Sum(nil))
}

// Common AES encryption function, used by many parts of Ego when encrypting
// data. The data is any arbitarary array of bytes, and the encrypted result
// is also an array of bytes. If an error occurs during encryption (such as
// the system-livel encryption library not being available), the function will
// return an emtpy byte erray along with an error.
func encrypt(data []byte, passphrase string) ([]byte, error) {
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

// Common AES decryption function, used by many parts of Ego when decrypting
// data. The data is any arbitarary array of bytes, and the decrypted result
// is also an array of bytes. If an error occurs during decryption (such as the
// data has been tampered with or the password string is incorrect), the
// decryption will fail and return a nil array along with an error.
func decrypt(data []byte, passphrase string) ([]byte, error) {
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
