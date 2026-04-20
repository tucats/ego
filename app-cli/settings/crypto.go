package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// encryptionV2Prefix marks ciphertext produced by the intermediate SHA-256
	// key-derivation scheme. Retained so existing v2 values can still be read.
	encryptionV2Prefix = "v2:"

	// encryptionV3Prefix marks ciphertext produced by the current Argon2id
	// key-derivation scheme. All new encryption uses this prefix.
	encryptionV3Prefix = "v3:"

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
// result as encryptionV3Prefix + base64-encoded string.
//
// Wire format of the raw bytes before base64 encoding:
//
//	[16-byte salt][12-byte nonce][AES-GCM ciphertext+tag]
//
// Decrypt automatically recognizes v3 (Argon2id), v2 (SHA-256), and legacy
// (MD5, no prefix) ciphertext, so existing profile data continues to work.
func Encrypt(data, password string) (string, error) {
	b, err := encrypt([]byte(data), password)
	if err != nil {
		return "", err
	}

	return encryptionV3Prefix + base64.StdEncoding.EncodeToString(b), nil
}

// NeedsNewHash reports whether a ciphertext string was not produced by the
// current Argon2id scheme and therefore should be re-encrypted. Both legacy
// MD5 (no prefix) and v2 SHA-256 values return true.
func NeedsNewHash(data string) bool {
	return !strings.HasPrefix(data, encryptionV3Prefix)
}

// Decrypt decrypts a string produced by Encrypt. It selects the key-derivation
// function based on the version prefix:
//
//   - "v3:" prefix: Argon2id key derivation — current scheme.
//   - "v2:" prefix: SHA-256 key derivation — intermediate scheme.
//   - no prefix:    MD5 key derivation — legacy scheme, for existing stored values.
//
// If the passphrase is incorrect or the ciphertext has been tampered with,
// an error is returned and the result is an empty string.
func Decrypt(data, password string) (string, error) {
	switch {
	case strings.HasPrefix(data, encryptionV3Prefix):
		src, err := base64.StdEncoding.DecodeString(data[len(encryptionV3Prefix):])
		if err != nil {
			return "", err
		}

		b, err := decryptArgon2id(src, password)
		if err != nil {
			return "", err
		}

		return string(b), nil

	case strings.HasPrefix(data, encryptionV2Prefix):
		src, err := base64.StdEncoding.DecodeString(data[len(encryptionV2Prefix):])
		if err != nil {
			return "", err
		}

		b, err := aesGCMDecrypt(deriveKey(password), src)
		if err != nil {
			return "", err
		}

		return string(b), nil

	default:
		// Legacy scheme: no prefix, MD5 key derivation.
		src, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return "", err
		}

		b, err := legacyDecrypt(src, password)
		if err != nil {
			return "", err
		}

		return string(b), nil
	}
}

// obsoleteMD5HasherDoNotUse creates an MD5 hex digest of the input string.
// Retained for legacy profile data that was encrypted with the MD5 scheme.
// Must not be used for new encryption.
func obsoleteMD5HasherDoNotUse(key string) string {
	hasher := md5.New()
	_, _ = hasher.Write([]byte(key))

	return hex.EncodeToString(hasher.Sum(nil))
}

// Hash returns the MD5 hex digest of s. Retained for backward compatibility
// with legacy profile data. Do not use for new encryption.
func Hash(s string) string {
	return obsoleteMD5HasherDoNotUse(s)
}

// deriveKey returns a 32-byte AES-256 key derived from passphrase using
// SHA-256. Used only to decrypt the intermediate v2 format that predates Argon2id.
func deriveKey(passphrase string) []byte {
	h := sha256.Sum256([]byte(passphrase))

	return h[:]
}

// encrypt encrypts data using AES-256-GCM with a key derived from passphrase
// via Argon2id. The returned byte slice is:
//
//	[16-byte salt][12-byte nonce][ciphertext+GCM tag]
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

	// Assemble: salt + nonce + ciphertext.
	out := make([]byte, 0, saltLen+len(cipherText))
	out = append(out, salt...)
	out = append(out, cipherText...)

	return out, nil
}

// decryptArgon2id decrypts v3 ciphertext (Argon2id key derivation).
// data is the raw bytes after base64 decoding: [16-byte salt][nonce][ciphertext].
func decryptArgon2id(data []byte, passphrase string) ([]byte, error) {
	if len(data) < saltLen {
		return []byte(""), nil
	}

	return aesGCMDecrypt(argon2idKey(passphrase, data[:saltLen]), data[saltLen:])
}

// legacyDecrypt handles ciphertext produced by the original MD5-keyed scheme.
// Retained solely for backward compatibility with existing profile files.
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
