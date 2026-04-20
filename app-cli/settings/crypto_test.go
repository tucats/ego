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
	"strings"
	"testing"
)

// TestEncryptDecrypt_RoundTrip verifies that values encrypted with Argon2id (v2)
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

// TestEncrypt_OutputIsBase64 verifies that Encrypt returns a valid base64 string.
func TestEncrypt_OutputIsBase64(t *testing.T) {
	ct, err := Encrypt("data", "key")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if _, err := base64.StdEncoding.DecodeString(ct); err != nil {
		t.Errorf("Encrypt output is not valid base64: %v", err)
	}
}

// TestEncrypt_ProducesArgon2Magic verifies that Encrypt embeds the v2 Argon2id
// magic prefix in the raw (pre-base64) ciphertext.
func TestEncrypt_ProducesArgon2Magic(t *testing.T) {
	ct, err := Encrypt("data", "key")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(ct)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}

	if len(raw) < len(argon2Magic) {
		t.Fatalf("ciphertext too short to contain magic prefix")
	}

	if !bytes.Equal(raw[:len(argon2Magic)], argon2Magic) {
		t.Errorf("expected v2 Argon2id magic %x, got %x", argon2Magic, raw[:len(argon2Magic)])
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

// TestDecrypt_LegacyMD5Format verifies backwards compatibility: values encrypted
// with the original MD5-keyed scheme still decrypt correctly.
func TestDecrypt_LegacyMD5Format(t *testing.T) {
	plaintext := "legacy profile secret"
	password := "old-profile-passphrase"

	legacyCT, err := legacyEncryptForTest([]byte(plaintext), password)
	if err != nil {
		t.Fatalf("legacyEncrypt: %v", err)
	}

	// Encode as base64 the same way the old Encrypt did.
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

// legacyEncryptForTest reproduces the original MD5-keyed AES-256-GCM encryption
// so backwards-compatibility tests can generate legacy ciphertext without
// depending on the old implementation.
func legacyEncryptForTest(data []byte, passphrase string) ([]byte, error) {
	key := []byte(legacyMD5KeyForTest(passphrase))

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

func legacyMD5KeyForTest(passphrase string) string {
	h := md5.New()
	_, _ = h.Write([]byte(passphrase))

	return hex.EncodeToString(h.Sum(nil))
}
