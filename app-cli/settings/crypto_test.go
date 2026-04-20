package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

// TestEncryptDecryptRoundtrip verifies that a value encrypted with Encrypt can
// be recovered by Decrypt using the same password.
func TestEncryptDecryptRoundtrip(t *testing.T) {
	cases := []struct {
		name     string
		data     string
		password string
	}{
		{"simple", "hello world", "secret"},
		{"empty data", "", "secret"},
		{"empty password", "hello", ""},
		{"unicode", "こんにちは世界", "passphrase"},
		{"long data", strings.Repeat("abcdefgh", 1000), "longpassword"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := Encrypt(tc.data, tc.password)
			if err != nil {
				t.Fatalf("Encrypt error: %v", err)
			}

			// New ciphertext must carry the v2 prefix.
			if !strings.HasPrefix(encrypted, encryptionV2Prefix) {
				t.Errorf("expected encrypted value to start with %q, got %q",
					encryptionV2Prefix, encrypted[:minInt(len(encrypted), 10)])
			}

			got, err := Decrypt(encrypted, tc.password)
			if err != nil {
				t.Fatalf("Decrypt error: %v", err)
			}

			if got != tc.data {
				t.Errorf("round-trip mismatch: got %q, want %q", got, tc.data)
			}
		})
	}
}

// TestDecryptWrongPassword verifies that decryption with the wrong password
// returns an error rather than silently returning garbage.
func TestDecryptWrongPassword(t *testing.T) {
	encrypted, err := Encrypt("sensitive value", "correct-password")
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	_, err = Decrypt(encrypted, "wrong-password")
	if err == nil {
		t.Error("expected an error when decrypting with wrong password, got nil")
	}
}

// TestDecryptLegacyCompatibility verifies that values encrypted with the old
// MD5-based scheme (no v2 prefix) can still be decrypted by Decrypt.
// It uses the unexported legacyEncryptForTest helper to produce genuine
// legacy ciphertext without relying on any external dependency.
func TestDecryptLegacyCompatibility(t *testing.T) {
	const (
		data     = "legacy secret value"
		password = "mypassword"
	)

	// Produce ciphertext using the old MD5-based key derivation.
	raw, err := legacyEncryptForTest([]byte(data), password)
	if err != nil {
		t.Fatalf("legacy encrypt error: %v", err)
	}

	// Encode as plain base64, no version prefix — this is what old Encrypt returned.
	legacyCiphertext := base64.StdEncoding.EncodeToString(raw)

	// Decrypt must recover the plaintext even though there is no v2 prefix.
	got, err := Decrypt(legacyCiphertext, password)
	if err != nil {
		t.Fatalf("Decrypt of legacy ciphertext error: %v", err)
	}

	if got != data {
		t.Errorf("legacy round-trip: got %q, want %q", got, data)
	}
}

// TestEncryptProducesUniqueOutput verifies that encrypting the same plaintext
// twice produces different ciphertexts (due to the random GCM nonce).
func TestEncryptProducesUniqueOutput(t *testing.T) {
	a, err := Encrypt("same data", "same password")
	if err != nil {
		t.Fatalf("first Encrypt error: %v", err)
	}

	b, err := Encrypt("same data", "same password")
	if err != nil {
		t.Fatalf("second Encrypt error: %v", err)
	}

	if a == b {
		t.Error("expected two encryptions of the same input to differ (different nonces)")
	}
}

// TestHashDeterministic verifies that Hash returns the same value for the same
// input across calls.
func TestHashDeterministic(t *testing.T) {
	for range 5 {
		h1 := Hash("some key")
		h2 := Hash("some key")

		if h1 != h2 {
			t.Error("Hash is not deterministic for the same input")
		}
	}
}

// TestHashDistinct verifies that different inputs produce different hashes.
func TestHashDistinct(t *testing.T) {
	if Hash("aaa") == Hash("bbb") {
		t.Error("Hash collision between distinct inputs")
	}
}

// TestDeriveKeyLength verifies that deriveKey always returns exactly 32 bytes,
// which is required for AES-256.
func TestDeriveKeyLength(t *testing.T) {
	for _, p := range []string{"", "short", strings.Repeat("x", 1000)} {
		key := deriveKey(p)
		if len(key) != 32 {
			t.Errorf("deriveKey(%q): got %d bytes, want 32", p, len(key))
		}
	}
}

// --------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------

// legacyEncryptForTest replicates the original MD5-based encrypt function so
// TestDecryptLegacyCompatibility can generate genuine pre-v2 ciphertext.
func legacyEncryptForTest(data []byte, passphrase string) ([]byte, error) {
	key := []byte(Hash(passphrase)) // MD5 hex → 32 bytes, the old scheme

	block, err := aes.NewCipher(key)
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

// minInt returns the smaller of a and b.
func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
