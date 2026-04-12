package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

// TestEncryptDecrypt_RoundTrip verifies that data encrypted with the new PBKDF2
// path is correctly recovered by Decrypt.
func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		plaintext string
		password  string
	}{
		{"short string", "hello", "secret"},
		{"empty string", "", "key"},
		{"unicode", "héllo wörld 🔒", "p@ssw0rd"},
		{"json payload", `{"user":"alice","token":"abc123"}`, "longpassphrase"},
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

// TestEncrypt_ProducesMagicPrefix verifies that Encrypt outputs the PBKDF2
// version marker so the format can be detected by Decrypt.
func TestEncrypt_ProducesMagicPrefix(t *testing.T) {
	ct, err := Encrypt("data", "key")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	raw := []byte(ct)
	if len(raw) < len(encryptMagic) {
		t.Fatalf("ciphertext too short to contain magic prefix")
	}

	for i, b := range encryptMagic {
		if raw[i] != b {
			t.Errorf("magic byte %d: got %02x, want %02x", i, raw[i], b)
		}
	}
}

// TestEncrypt_UniqueSalts verifies that two encryptions of the same plaintext
// with the same key produce different ciphertext (due to random salt + nonce).
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

// TestDecrypt_WrongPassword verifies that decryption with an incorrect key
// returns an error rather than silently returning garbage.
func TestDecrypt_WrongPassword(t *testing.T) {
	ct, err := Encrypt("secret data", "correct-password")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = Decrypt(ct, "wrong-password")
	if err == nil {
		t.Error("expected error when decrypting with wrong password, got nil")
	}
}

// TestDecrypt_LegacyMD5Format verifies backwards compatibility: ciphertext
// produced with the old MD5-keyed scheme can still be decrypted.
func TestDecrypt_LegacyMD5Format(t *testing.T) {
	plaintext := "legacy secret payload"
	password := "oldpassword"

	// Produce legacy ciphertext the same way the old code did.
	legacyCT, err := legacyEncryptForTest([]byte(plaintext), password)
	if err != nil {
		t.Fatalf("legacyEncrypt: %v", err)
	}

	got, err := Decrypt(string(legacyCT), password)
	if err != nil {
		t.Fatalf("Decrypt of legacy ciphertext: %v", err)
	}

	if got != plaintext {
		t.Errorf("legacy round-trip: got %q, want %q", got, plaintext)
	}
}

// TestDecrypt_LegacyMD5_WrongPassword verifies that a legacy ciphertext
// decrypted with the wrong password returns an error.
func TestDecrypt_LegacyMD5_WrongPassword(t *testing.T) {
	legacyCT, err := legacyEncryptForTest([]byte("data"), "correct")
	if err != nil {
		t.Fatalf("legacyEncrypt: %v", err)
	}

	_, err = Decrypt(string(legacyCT), "wrong")
	if err == nil {
		t.Error("expected error for wrong password on legacy ciphertext")
	}
}

// TestHash_SHA256 verifies that Hash produces SHA-256 output.
func TestHash_SHA256(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// SHA-256("") — canonical value
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		// SHA-256("hello")
		{"hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
	}

	for _, tc := range tests {
		got := Hash(tc.in)
		if got != tc.want {
			t.Errorf("Hash(%q) = %q, want %q", tc.in, got, tc.want)
		}

		if len(got) != 64 {
			t.Errorf("Hash(%q) length = %d, want 64", tc.in, len(got))
		}

		if strings.ToLower(got) != got {
			t.Errorf("Hash(%q) is not lowercase hex: %q", tc.in, got)
		}
	}
}

// TestHash_Deterministic verifies that the same input always produces the same hash.
func TestHash_Deterministic(t *testing.T) {
	h1 := Hash("input")
	h2 := Hash("input")

	if h1 != h2 {
		t.Errorf("Hash is not deterministic: %q != %q", h1, h2)
	}
}

// TestHash_DifferentInputs verifies that different inputs produce different hashes.
func TestHash_DifferentInputs(t *testing.T) {
	if Hash("a") == Hash("b") {
		t.Error("Hash collision between 'a' and 'b'")
	}
}

// legacyEncryptForTest reproduces the original MD5-keyed AES-256-GCM encryption
// so backwards-compatibility tests can generate legacy ciphertext without
// depending on the old implementation.
func legacyEncryptForTest(data []byte, passphrase string) ([]byte, error) {
	key := legacyMD5KeyForTest(passphrase)

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

func legacyMD5KeyForTest(passphrase string) []byte {
	h := md5.New()
	_, _ = h.Write([]byte(passphrase))

	return []byte(hex.EncodeToString(h.Sum(nil)))
}
