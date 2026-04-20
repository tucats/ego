package settings

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

// TestEncryptDecrypt_RoundTrip verifies that values encrypted with Argon2id (v3)
// are correctly recovered by Decrypt.
func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
		password  string
	}{
		{"short string", "hello", "secret"},
		{"empty string", "", "key"},
		{"unicode", "héllo wörld 🔒", "p@ssw0rd"},
		{"token payload", "eyJhbGciOiJBMjU2R0NNIn0.secret-token-value", "server-key"},
		{"database url", "postgres://user:pass@host:5432/db", "profile-salt-uuid"},
		{"binary-like content", string([]byte{0, 1, 2, 3, 255, 254, 253}), "key"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := Encrypt(tc.plaintext, tc.password)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}

			got, err := Decrypt(ciphertext, tc.password)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}

			if got != tc.plaintext {
				t.Errorf("round-trip mismatch: got %q, want %q", got, tc.plaintext)
			}
		})
	}
}

// TestEncryptDecryptRoundtrip verifies additional round-trip cases including
// empty data, empty password, and long data.
func TestEncryptDecryptRoundtrip(t *testing.T) {
	cases := []struct {
		name     string
		data     string
		password string
	}{
		{"simple", "hello world", "secret"},
		{"empty data", "", "secret"},
		{"empty password", "hello", ""},
		{"japanese", "こんにちは世界", "passphrase"},
		{"long data", strings.Repeat("abcdefgh", 1000), "longpassword"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := Encrypt(tc.data, tc.password)
			if err != nil {
				t.Fatalf("Encrypt error: %v", err)
			}

			// New ciphertext must carry the v3 prefix.
			if !strings.HasPrefix(encrypted, encryptionV3Prefix) {
				t.Errorf("expected encrypted value to start with %q, got %q",
					encryptionV3Prefix, encrypted[:minInt(len(encrypted), 10)])
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

// TestEncrypt_OutputIsBase64 verifies that Encrypt returns a valid v3-prefixed
// base64 string.
func TestEncrypt_OutputIsBase64(t *testing.T) {
	ct, err := Encrypt("data", "key")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if !strings.HasPrefix(ct, encryptionV3Prefix) {
		t.Fatalf("expected %q prefix, got %q", encryptionV3Prefix, ct[:minInt(len(ct), 10)])
	}

	if _, err := base64.StdEncoding.DecodeString(ct[len(encryptionV3Prefix):]); err != nil {
		t.Errorf("Encrypt output (after prefix) is not valid base64: %v", err)
	}
}

// TestEncrypt_UniqueSalts verifies that two encryptions of the same plaintext
// with the same key produce different ciphertext due to the random salt and nonce.
func TestEncrypt_UniqueSalts(t *testing.T) {
	ct1, err := Encrypt("same data", "same key")
	if err != nil {
		t.Fatalf("first Encrypt: %v", err)
	}

	ct2, err := Encrypt("same data", "same key")
	if err != nil {
		t.Fatalf("second Encrypt: %v", err)
	}

	if ct1 == ct2 {
		t.Error("expected two encryptions of the same plaintext to produce different ciphertext")
	}
}

// TestEncrypt_SaltEmbedded verifies that the salt is embedded in the raw
// ciphertext (first saltLen bytes after base64 decode and prefix strip).
func TestEncrypt_SaltEmbedded(t *testing.T) {
	ct, err := Encrypt("data", "key")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(ct[len(encryptionV3Prefix):])
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}

	// Raw bytes must be at least salt + nonce + GCM tag.
	minLen := saltLen + 12 + 16
	if len(raw) < minLen {
		t.Fatalf("ciphertext too short: got %d bytes, want at least %d", len(raw), minLen)
	}

	// Encrypting the same data twice with the same key must produce different salts.
	ct2, err := Encrypt("data", "key")
	if err != nil {
		t.Fatalf("second Encrypt: %v", err)
	}

	raw2, _ := base64.StdEncoding.DecodeString(ct2[len(encryptionV3Prefix):])

	if bytes.Equal(raw[:saltLen], raw2[:saltLen]) {
		t.Error("expected different salts for two encryptions")
	}
}

// TestDecrypt_WrongPassword verifies that decryption with the wrong key returns
// an error rather than silently producing garbage.
func TestDecrypt_WrongPassword(t *testing.T) {
	ct, err := Encrypt("secret profile data", "correct-password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = Decrypt(ct, "wrong-password")
	if err == nil {
		t.Error("expected error when decrypting with wrong password, got nil")
	}
}

// TestDecrypt_LegacyMD5Format verifies backward compatibility: values encrypted
// with the original MD5-keyed scheme still decrypt correctly.
func TestDecrypt_LegacyMD5Format(t *testing.T) {
	plaintext := "legacy profile secret"
	password := "old-profile-passphrase"

	legacyCT, err := legacyEncryptForTest([]byte(plaintext), password)
	if err != nil {
		t.Fatalf("legacyEncrypt: %v", err)
	}

	// Encode as plain base64, no version prefix — this is what old Encrypt returned.
	encoded := base64.StdEncoding.EncodeToString(legacyCT)

	got, err := Decrypt(encoded, password)
	if err != nil {
		t.Fatalf("Decrypt of legacy ciphertext: %v", err)
	}

	if got != plaintext {
		t.Errorf("legacy round-trip: got %q, want %q", got, plaintext)
	}
}

// TestDecrypt_LegacyMD5_WrongPassword verifies that legacy ciphertext with the
// wrong password returns an error.
func TestDecrypt_LegacyMD5_WrongPassword(t *testing.T) {
	legacyCT, err := legacyEncryptForTest([]byte("data"), "correct")
	if err != nil {
		t.Fatalf("legacyEncrypt: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString(legacyCT)

	_, err = Decrypt(encoded, "wrong")
	if err == nil {
		t.Error("expected error for wrong password on legacy ciphertext")
	}
}

// TestDecrypt_V2SHA256Format verifies backward compatibility: values encrypted
// with the intermediate SHA-256 v2 scheme still decrypt correctly.
func TestDecrypt_V2SHA256Format(t *testing.T) {
	const (
		data     = "v2 encrypted secret"
		password = "v2password"
	)

	raw, err := v2EncryptForTest([]byte(data), password)
	if err != nil {
		t.Fatalf("v2 encrypt error: %v", err)
	}

	v2Ciphertext := encryptionV2Prefix + base64.StdEncoding.EncodeToString(raw)

	got, err := Decrypt(v2Ciphertext, password)
	if err != nil {
		t.Fatalf("Decrypt of v2 ciphertext: %v", err)
	}

	if got != data {
		t.Errorf("v2 round-trip: got %q, want %q", got, data)
	}
}

// TestDecrypt_V2SHA256_WrongPassword verifies that v2 ciphertext with the wrong
// password returns an error.
func TestDecrypt_V2SHA256_WrongPassword(t *testing.T) {
	raw, err := v2EncryptForTest([]byte("data"), "correct")
	if err != nil {
		t.Fatalf("v2 encrypt error: %v", err)
	}

	v2Ciphertext := encryptionV2Prefix + base64.StdEncoding.EncodeToString(raw)

	_, err = Decrypt(v2Ciphertext, "wrong")
	if err == nil {
		t.Error("expected error for wrong password on v2 ciphertext")
	}
}

// TestDecrypt_InvalidBase64 verifies that a non-base64 input returns an error.
func TestDecrypt_InvalidBase64(t *testing.T) {
	_, err := Decrypt("not-valid-base64!!!", "password")
	if err == nil {
		t.Error("expected error for invalid base64 input, got nil")
	}
}

// TestHash_IsMD5 verifies that Hash returns the MD5 hex digest of the input.
func TestHash_IsMD5(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// MD5("") — canonical value
		{"", "d41d8cd98f00b204e9800998ecf8427e"},
		// MD5("hello")
		{"hello", "5d41402abc4b2a76b9719d911017c592"},
	}

	for _, tc := range tests {
		got := obsoleteMD5HasherDoNotUse(tc.in)
		if got != tc.want {
			t.Errorf("Hash(%q) = %q, want %q", tc.in, got, tc.want)
		}

		if len(got) != 32 {
			t.Errorf("Hash(%q) length = %d, want 32", tc.in, len(got))
		}

		if strings.ToLower(got) != got {
			t.Errorf("Hash(%q) is not lowercase hex: %q", tc.in, got)
		}
	}
}

// TestHash_Deterministic verifies that the same input always produces the same digest.
func TestHash_Deterministic(t *testing.T) {
	h1 := obsoleteMD5HasherDoNotUse("profile-salt")
	h2 := obsoleteMD5HasherDoNotUse("profile-salt")

	if h1 != h2 {
		t.Errorf("Hash is not deterministic: %q != %q", h1, h2)
	}
}

// TestHash_DifferentInputs verifies that different inputs produce different digests.
func TestHash_DifferentInputs(t *testing.T) {
	if obsoleteMD5HasherDoNotUse("a") == obsoleteMD5HasherDoNotUse("b") {
		t.Error("Hash collision between 'a' and 'b'")
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

// TestNeedsNewHash verifies that NeedsNewHash correctly identifies values that
// require re-encryption.
func TestNeedsNewHash(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{"v3 current", encryptionV3Prefix + "somebase64data", false},
		{"v2 intermediate", encryptionV2Prefix + "somebase64data", true},
		{"legacy no prefix", "somebase64data", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NeedsNewHash(tc.data); got != tc.want {
				t.Errorf("NeedsNewHash(%q) = %v, want %v", tc.data, got, tc.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------

// legacyEncryptForTest replicates the original MD5-based encrypt so that
// legacy backward-compatibility tests can generate genuine pre-v2 ciphertext.
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

// v2EncryptForTest replicates the intermediate SHA-256-based encrypt so that
// v2 backward-compatibility tests can generate genuine v2 ciphertext.
func v2EncryptForTest(data []byte, passphrase string) ([]byte, error) {
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

// minInt returns the smaller of a and b.
func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}
