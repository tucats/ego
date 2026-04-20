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

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"

	"github.com/tucats/ego/errors"
)

// encryptMagic is the 4-byte prefix that identifies v2 (PBKDF2-SHA256) ciphertext.
// Using 0xFF as the first byte (above printable ASCII) ensures it cannot be
// confused with any structured text payload. The probability of a legacy
// (random-nonce) ciphertext starting with this exact sequence is 1 in 2^32.
var encryptMagic = []byte{0xFF, 0x45, 0x47, 0x4F} // ÿEGO — v2 PBKDF2

// argon2Magic is the 4-byte prefix that identifies v3 (Argon2id) ciphertext.
// It shares the same 0xFF lead byte and "EG" body as encryptMagic; the trailing
// 0x33 ('3') distinguishes this as version 3.
var argon2Magic = []byte{0xFF, 0x45, 0x47, 0x33} // ÿEG3 — v3 Argon2id

const (
	// pbkdf2Iterations is the PBKDF2 work factor used by the v2 format.
	// Retained for decrypting existing v2 ciphertext only.
	pbkdf2Iterations = 100_000

	// pbkdf2KeyLen / argon2KeyLen is the AES-256 key length in bytes.
	pbkdf2KeyLen  = 32
	argon2KeyLen  = 32

	// saltLen is the length of the random per-encryption salt (shared by all formats).
	saltLen = 16

	// argon2Memory is the Argon2id memory parameter in KiB (32 MiB).
	// This exceeds the OWASP 2024 minimum of 19 MiB at t=2 while staying
	// server-friendly when handling concurrent requests.
	argon2Memory = 32 * 1024

	// argon2Time is the Argon2id iteration count.
	argon2Time = 2

	// argon2Threads is the Argon2id parallelism parameter. 1 keeps each key
	// derivation self-contained and avoids goroutine overhead in the caller.
	argon2Threads = 1
)

// Encrypt encrypts a string using AES-256-GCM with a key derived via
// Argon2id (32 MiB, 2 iterations, 16-byte random per-message salt).
//
// Wire format of the returned byte string (v3):
//
//	[4-byte magic ÿEG3][16-byte salt][12-byte nonce][AES-GCM ciphertext+tag]
//
// Decrypt automatically recognises v3, v2 (PBKDF2-SHA256, magic ÿEGO), and
// the legacy MD5 format, so existing ciphertext continues to decrypt correctly.
func Encrypt(data, password string) (string, error) {
	b, err := encrypt([]byte(data), password)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Decrypt decrypts a string that was produced by Encrypt. It automatically
// detects the ciphertext format by inspecting the leading magic bytes:
//
//   - v3 (ÿEG3 magic): Argon2id key derivation — current format.
//   - v2 (ÿEGO magic): PBKDF2-SHA256 key derivation — retained for existing data.
//   - legacy (no magic): MD5 key derivation — retained for oldest data.
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
	// Generate a fresh random salt for Argon2id key derivation.
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, errors.New(err)
	}

	block, err := aes.NewCipher(argon2idKey(passphrase, salt))
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

	// Assemble the output: v3 magic + salt + (nonce + ciphertext)
	out := make([]byte, 0, len(argon2Magic)+saltLen+len(cipherText))
	out = append(out, argon2Magic...)
	out = append(out, salt...)
	out = append(out, cipherText...)

	return out, nil
}

// decrypt dispatches to the appropriate decryption path based on the
// 4-byte magic prefix: v3 Argon2id, v2 PBKDF2, or legacy MD5.
func decrypt(data []byte, passphrase string) ([]byte, error) {
	if len(data) > len(argon2Magic) && bytes.Equal(data[:len(argon2Magic)], argon2Magic) {
		return decryptArgon2id(data[len(argon2Magic):], passphrase)
	}

	if len(data) > len(encryptMagic) && bytes.Equal(data[:len(encryptMagic)], encryptMagic) {
		return decryptPBKDF2(data[len(encryptMagic):], passphrase)
	}

	return legacyDecrypt(data, passphrase)
}

// decryptArgon2id handles v3 ciphertext (magic ÿEG3).
// data has the magic prefix already stripped and begins with the 16-byte salt.
func decryptArgon2id(data []byte, passphrase string) ([]byte, error) {
	if len(data) < saltLen {
		return []byte(""), nil
	}

	return aesGCMDecrypt(argon2idKey(passphrase, data[:saltLen]), data[saltLen:])
}

// decryptPBKDF2 handles v2 ciphertext (magic ÿEGO) produced by the previous
// encrypt implementation. Retained solely for backwards compatibility.
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

// argon2idKey derives a 32-byte AES-256 key from a passphrase and salt using
// Argon2id with the parameters defined by the argon2* constants above.
// Argon2id is memory-hard and resistant to GPU/ASIC brute-force attacks.
func argon2idKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
}

// pbkdf2Key derives a 32-byte AES-256 key from a passphrase and salt using
// PBKDF2-SHA256. Used only to decrypt existing v2 ciphertext.
func pbkdf2Key(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, pbkdf2KeyLen, sha256.New)
}

// md5Key derives a key using the original MD5 scheme. The output is the
// hex-encoded MD5 digest of the passphrase (32 ASCII bytes). This gives only
// 128 bits of effective entropy and MD5 is cryptographically broken.
//
// This function exists solely for decrypting legacy ciphertext and must never
// be used for new encryption.
func md5Key(passphrase string) []byte {
	h := md5.New()
	_, _ = h.Write([]byte(passphrase))

	return []byte(hex.EncodeToString(h.Sum(nil)))
}
