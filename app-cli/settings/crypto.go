package settings

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"

	"golang.org/x/crypto/argon2"
)

// argon2Magic is the 4-byte prefix that identifies v2 (Argon2id) ciphertext in
// the profile store. Using 0xFF as the first byte (non-printable, above base64
// alphabet) ensures no confusion with legacy MD5 ciphertext, which has no prefix.
// The bytes spell ÿEG3 to match the token-layer convention in util/crypto.go.
var argon2Magic = []byte{0xFF, 0x45, 0x47, 0x33} // ÿEG3

const (
	// saltLen is the length of the random per-encryption Argon2id salt.
	saltLen = 16

	// argon2Memory is the Argon2id memory cost in KiB (32 MiB). Exceeds the
	// OWASP 2024 minimum of 19 MiB at t=2 while staying server-friendly.
	argon2Memory = 32 * 1024

	// argon2Time is the Argon2id iteration count.
	argon2Time = 2

	// argon2Threads is the Argon2id parallelism parameter.
	argon2Threads = 1

	// argon2KeyLen is the AES-256 key size in bytes.
	argon2KeyLen = 32
)

// Encrypt encrypts a string using AES-256-GCM with a key derived via Argon2id
// (32 MiB, 2 iterations, 16-byte random per-encryption salt) and returns the
// result as a base64-encoded string.
//
// Wire format of the raw bytes before base64 encoding (v2):
//
//	[4-byte magic ÿEG3][16-byte salt][12-byte nonce][AES-GCM ciphertext+tag]
//
// Decrypt automatically recognizes v2 (Argon2id) and legacy (MD5, no magic)
// ciphertext, so existing profile data continues to decrypt correctly.
func Encrypt(data, password string) (string, error) {
	b, err := encrypt([]byte(data), password)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

// Decrypt decrypts a base64-encoded string that was produced by Encrypt.
// It automatically detects the ciphertext format:
//
//   - v2 (ÿEG3 magic): Argon2id key derivation — current format.
//   - legacy (no magic): MD5 key derivation — retained for existing profile data.
//
// If the passphrase is incorrect or the ciphertext has been tampered with,
// an error is returned and the result is an empty string.
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

// obsoleteMD5HasherDoNotUse creates an MD5 hex digest of the input string.
// Retained for legacy profile data that was encrypted with the MD5 scheme.
// Must not be used for new encryption.
func obsoleteMD5HasherDoNotUse(key string) string {
	hasher := md5.New()
	_, _ = hasher.Write([]byte(key))

	return hex.EncodeToString(hasher.Sum(nil))
}

func encrypt(data []byte, passphrase string) ([]byte, error) {
	// Generate a fresh random salt for Argon2id key derivation.
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(argon2idKey(passphrase, salt))
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	cipherText := gcm.Seal(nonce, nonce, data, nil)

	// Assemble: v2 magic + salt + (nonce + ciphertext)
	out := make([]byte, 0, len(argon2Magic)+saltLen+len(cipherText))
	out = append(out, argon2Magic...)
	out = append(out, salt...)
	out = append(out, cipherText...)

	return out, nil
}

// decrypt dispatches to Argon2id (v2) or the legacy MD5 path based on the
// presence of the 4-byte magic prefix.
func decrypt(data []byte, passphrase string) ([]byte, error) {
	if len(data) > len(argon2Magic) && bytes.Equal(data[:len(argon2Magic)], argon2Magic) {
		return decryptArgon2id(data[len(argon2Magic):], passphrase)
	}

	return legacyDecrypt(data, passphrase)
}

// decryptArgon2id handles v2 ciphertext (magic ÿEG3).
// data has the magic prefix already stripped and begins with the 16-byte salt.
func decryptArgon2id(data []byte, passphrase string) ([]byte, error) {
	if len(data) < saltLen {
		return []byte(""), nil
	}

	return aesGCMDecrypt(argon2idKey(passphrase, data[:saltLen]), data[saltLen:])
}

// legacyDecrypt handles ciphertext produced by the original MD5-keyed scheme.
// Retained solely for backwards compatibility with existing profile files.
func legacyDecrypt(data []byte, passphrase string) ([]byte, error) {
	return aesGCMDecrypt([]byte(obsoleteMD5HasherDoNotUse(passphrase)), data)
}

// aesGCMDecrypt performs AES-256-GCM decryption given a key and data in the
// format [12-byte nonce][ciphertext+16-byte GCM tag].
func aesGCMDecrypt(key, data []byte) ([]byte, error) {
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

	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// argon2idKey derives a 32-byte AES-256 key from a passphrase and salt using
// Argon2id with the parameters defined by the argon2* constants above.
func argon2idKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
}
