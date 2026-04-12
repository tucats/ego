package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"golang.org/x/crypto/pbkdf2"

	"github.com/tucats/ego/errors"
)

// encryptMagic is the 4-byte prefix that identifies PBKDF2-format ciphertext.
// Using 0xFF as the first byte (above printable ASCII) ensures it cannot be
// confused with any structured text payload. The probability of a legacy
// (random-nonce) ciphertext starting with this exact sequence is 1 in 2^32.
var encryptMagic = []byte{0xFF, 0x45, 0x47, 0x4F} // ÿEGO

const (
	// pbkdf2Iterations is the PBKDF2 work factor. 100,000 iterations with
	// SHA-256 meets the OWASP recommendation for interactive logins as of 2024.
	pbkdf2Iterations = 100_000

	// pbkdf2KeyLen is the AES-256 key length in bytes.
	pbkdf2KeyLen = 32

	// saltLen is the length of the random per-encryption PBKDF2 salt.
	saltLen = 16
)

// Encrypt encrypts a string using AES-256-GCM with a key derived via
// PBKDF2-SHA256 (100,000 iterations, 16-byte random per-message salt).
//
// Wire format of the returned byte string:
//
//	[4-byte magic][16-byte salt][12-byte nonce][AES-GCM ciphertext+tag]
func Encrypt(data, password string) (string, error) {
	b, err := encrypt([]byte(data), password)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Decrypt decrypts a string that was produced by Encrypt. It automatically
// detects the ciphertext format:
//
//   - PBKDF2 format (magic prefix present): uses PBKDF2-SHA256 key derivation.
//   - Legacy format (no magic prefix):      uses MD5 key derivation for
//     backwards compatibility with data encrypted before this change.
//
// If the passphrase is incorrect or the ciphertext has been tampered with,
// an error is returned and the result is an empty string.
func Decrypt(data, password string) (string, error) {
	b, err := decrypt([]byte(data), password)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Hash creates a non-reversible SHA-256 hex digest of the input string.
// Two identical inputs always produce the same output. The result is
// a 64-character lowercase hexadecimal string.
//
// This is a general-purpose content digest. It must NOT be used for password
// storage — use auth.HashPassword (bcrypt) for that purpose.
func Hash(key string) string {
	h := sha256.Sum256([]byte(key))

	return hex.EncodeToString(h[:])
}

// --- internal implementation ---

func encrypt(data []byte, passphrase string) ([]byte, error) {
	// Generate a fresh random salt for PBKDF2 key derivation.
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, errors.New(err)
	}

	block, err := aes.NewCipher(pbkdf2Key(passphrase, salt))
	if err != nil {
		return nil, errors.New(err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New(err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.New(err)
	}

	cipherText := gcm.Seal(nonce, nonce, data, nil)

	// Assemble the output: magic + salt + (nonce + ciphertext)
	out := make([]byte, 0, len(encryptMagic)+saltLen+len(cipherText))
	out = append(out, encryptMagic...)
	out = append(out, salt...)
	out = append(out, cipherText...)

	return out, nil
}

// decrypt dispatches to the PBKDF2 or legacy decryption path based on the
// 4-byte magic prefix.
func decrypt(data []byte, passphrase string) ([]byte, error) {
	if len(data) > len(encryptMagic) && bytes.Equal(data[:len(encryptMagic)], encryptMagic) {
		return decryptPBKDF2(data[len(encryptMagic):], passphrase)
	}

	return legacyDecrypt(data, passphrase)
}

// decryptPBKDF2 handles ciphertext produced by the new encrypt function.
// data has the magic prefix already stripped and begins with the 16-byte salt.
func decryptPBKDF2(data []byte, passphrase string) ([]byte, error) {
	if len(data) < saltLen {
		return []byte(""), nil
	}

	return aesGCMDecrypt(pbkdf2Key(passphrase, data[:saltLen]), data[saltLen:])
}

// legacyDecrypt handles ciphertext encrypted with the original MD5-keyed scheme.
// It is retained solely for backwards compatibility with data created before
// the PBKDF2 upgrade and must not be called for new encryption.
func legacyDecrypt(data []byte, passphrase string) ([]byte, error) {
	return aesGCMDecrypt(md5Key(passphrase), data)
}

// aesGCMDecrypt performs AES-256-GCM decryption given a key and data in the
// format [12-byte nonce][ciphertext+16-byte GCM tag].
func aesGCMDecrypt(key, data []byte) ([]byte, error) {
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

	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return nil, errors.New(err)
	}

	return plaintext, nil
}

// pbkdf2Key derives a 32-byte AES-256 key from a passphrase and random salt
// using PBKDF2-SHA256.
func pbkdf2Key(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, pbkdf2KeyLen, sha256.New)
}

// md5Key derives a key using the original MD5 scheme. The output is the
// hex-encoded MD5 digest of the passphrase (32 ASCII bytes). This gives only
// 128 bits of effective entropy and md5 is cryptographically broken.
//
// This function exists solely for decrypting legacy ciphertext and must never
// be used for new encryption.
func md5Key(passphrase string) []byte {
	h := md5.New()
	_, _ = h.Write([]byte(passphrase))

	return []byte(hex.EncodeToString(h.Sum(nil)))
}
