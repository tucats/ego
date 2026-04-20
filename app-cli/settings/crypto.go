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
)

// encryptionV2Prefix is prepended (before base64 encoding) to all ciphertext
// produced by the current encryption scheme so that Decrypt can tell the two
// apart.  Legacy ciphertext has no prefix; V2 ciphertext always starts with
// this string.
const encryptionV2Prefix = "v2:"

// Encrypt encrypts a string using a password and returns a versioned,
// base64-encoded ciphertext string.  The returned value begins with
// encryptionV2Prefix so that Decrypt can select the correct key-derivation
// path.
func Encrypt(data, password string) (string, error) {
	b, err := encrypt([]byte(data), password)
	if err != nil {
		return "", err
	}

	return encryptionV2Prefix + base64.StdEncoding.EncodeToString(b), nil
}

// NeedsNewHash reports whether a ciphertext string was produced by the legacy
// MD5-based encryption scheme and therefore should be re-encrypted with the
// current SHA-256 scheme.
//
// Pass the ciphertext as it is stored — either the raw database value
// (e.g. "BASE64...") or the file-sidecar value after stripping the
// "encrypted: " storage prefix (e.g. "BASE64..." or "v2:BASE64...").
// Values that already start with encryptionV2Prefix are up-to-date and
// return false.
func NeedsNewHash(data string) bool {
	return !strings.HasPrefix(data, encryptionV2Prefix)
}

// Decrypt decrypts a string produced by Encrypt.  It inspects the prefix of
// the ciphertext to choose the key-derivation function:
//   - "v2:" prefix → SHA-256 key derivation (current scheme)
//   - no prefix    → MD5 hex key derivation (legacy scheme, supported for
//     reading existing encrypted values until they are re-encrypted)
func Decrypt(data, password string) (string, error) {
	if strings.HasPrefix(data, encryptionV2Prefix) {
		// Current scheme: strip the prefix and decrypt with SHA-256 key.
		src, err := base64.StdEncoding.DecodeString(data[len(encryptionV2Prefix):])
		if err != nil {
			return "", err
		}

		b, err := decrypt(src, password)
		if err != nil {
			return "", err
		}

		return string(b), nil
	}

	// Legacy scheme: no prefix means the ciphertext was encrypted with the old
	// MD5-based key derivation.  Decrypt it transparently so existing stored
	// values continue to work.
	src, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	b, err := decryptLegacy(src, password)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Hash creates a hash string from a key string. The hash cannot be
// reversed back into the key string, but two instances of the same
// key string result in the same hash value.
func Hash(key string) string {
	hasher := md5.New()
	_, _ = hasher.Write([]byte(key))

	return hex.EncodeToString(hasher.Sum(nil))
}

// deriveKey returns a 32-byte AES-256 key derived from passphrase using
// SHA-256.  SHA-256 produces exactly 32 bytes, matching the AES-256 key
// length requirement without any additional encoding step.
func deriveKey(passphrase string) []byte {
	h := sha256.Sum256([]byte(passphrase))

	return h[:]
}

// encrypt encrypts data using AES-256-GCM with a key derived from passphrase
// via SHA-256.  The returned byte slice is nonce || ciphertext; both parts are
// needed by decrypt to recover the original data.
func encrypt(data []byte, passphrase string) ([]byte, error) {
	block, err := aes.NewCipher(deriveKey(passphrase))
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

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// decrypt reverses encrypt using AES-256-GCM with a SHA-256-derived key.
func decrypt(data []byte, passphrase string) ([]byte, error) {
	block, err := aes.NewCipher(deriveKey(passphrase))
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

	return gcm.Open(nil, nonce, cipherText, nil)
}

// decryptLegacy decrypts ciphertext that was produced by the original
// MD5-based encryption scheme (before the v2 prefix was introduced).  It
// exists solely to allow existing stored values to be read; all new
// encryption uses the SHA-256 path in encrypt/decrypt above.
//
// Do not use this for new encrypted values.  Once all legacy ciphertext has
// been re-encrypted by Encrypt (which writes the v2 prefix), this function
// can be removed.
func decryptLegacy(data []byte, passphrase string) ([]byte, error) {
	// The old Hash() function returns a 32-character hex string, which happens
	// to be a valid 32-byte AES-256 key.
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

	return gcm.Open(nil, nonce, cipherText, nil)
}
