package authserver

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"
)

func TestGenerateCode_Randomness(t *testing.T) {
	codes := make(map[string]bool)

	for i := 0; i < 100; i++ {
		code, err := generateCode()
		if err != nil {
			t.Fatalf("generateCode error: %v", err)
		}

		if codes[code] {
			t.Fatal("duplicate code generated — randomness failure")
		}

		codes[code] = true

		// Codes should be at least 40 characters (32 bytes base64url).
		if len(code) < 40 {
			t.Errorf("code too short: %d chars", len(code))
		}
	}
}

func TestStoreAndConsumeCode(t *testing.T) {
	code, _ := generateCode()
	pending := PendingAuthorization{
		ClientID:    "testclient",
		RedirectURI: "https://example.com/cb",
		Scopes:      []string{"openid"},
		Username:    "alice",
		IssuedAt:    time.Now(),
	}

	storeCode(code, pending)

	got, found := consumeCode(code)
	if !found {
		t.Fatal("consumeCode should find the stored code")
	}

	if got.ClientID != "testclient" {
		t.Errorf("unexpected client ID: %s", got.ClientID)
	}

	// Second consume should fail — single-use semantics.
	_, found = consumeCode(code)
	if found {
		t.Error("consumeCode should return false on second call (single-use)")
	}
}

func TestGenerateAndConsumeRefreshToken(t *testing.T) {
	token, err := generateRefreshToken("myclient", "bob", []string{"openid", "ego:read"})
	if err != nil {
		t.Fatalf("generateRefreshToken error: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty refresh token")
	}

	data, found := consumeRefreshToken(token)
	if !found {
		t.Fatal("consumeRefreshToken should find the stored token")
	}

	if data.ClientID != "myclient" {
		t.Errorf("unexpected client ID: %s", data.ClientID)
	}

	// Second consume should fail — single-use token rotation.
	_, found = consumeRefreshToken(token)
	if found {
		t.Error("consumeRefreshToken should return false on second call (rotation)")
	}
}

func TestVerifyPKCE_NoChallengeAlwaysPasses(t *testing.T) {
	pending := PendingAuthorization{}
	if err := verifyPKCE(pending, "anything"); err != nil {
		t.Errorf("expected no error when no PKCE was used: %v", err)
	}
}

func TestVerifyPKCE_S256Correct(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	pending := PendingAuthorization{
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	}

	if err := verifyPKCE(pending, verifier); err != nil {
		t.Errorf("unexpected PKCE error: %v", err)
	}
}

func TestVerifyPKCE_S256Wrong(t *testing.T) {
	pending := PendingAuthorization{
		CodeChallenge:       "wrongchallenge",
		CodeChallengeMethod: "S256",
	}

	if err := verifyPKCE(pending, "verifier"); err == nil {
		t.Error("expected PKCE verification failure for wrong verifier")
	}
}
