package authserver

import (
	"path/filepath"
	"testing"
	"time"
)

func setupTestKey(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := loadOrGenerateKey(filepath.Join(dir, "test.pem")); err != nil {
		t.Fatalf("key setup failed: %v", err)
	}
}

func TestCreateAccessToken_ReturnsValidJWT(t *testing.T) {
	setupTestKey(t)

	cfg := asConfig{
		Issuer:          "https://ego.test",
		TokenExpiration: time.Hour,
	}

	token, jti, err := createAccessToken(cfg, "myclient", "alice", "myclient", "openid ego:read")
	if err != nil {
		t.Fatalf("createAccessToken failed: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token string")
	}

	if jti == "" {
		t.Error("expected non-empty JTI")
	}
}

func TestParseToken_ValidToken(t *testing.T) {
	setupTestKey(t)

	cfg := asConfig{
		Issuer:          "https://ego.test",
		TokenExpiration: time.Hour,
	}

	tokenStr, _, err := createAccessToken(cfg, "myclient", "alice", "myclient", "openid ego:read")
	if err != nil {
		t.Fatalf("createAccessToken failed: %v", err)
	}

	claims, err := parseToken(tokenStr)
	if err != nil {
		t.Fatalf("parseToken failed: %v", err)
	}

	if claims.Subject != "alice" {
		t.Errorf("expected sub=alice, got %q", claims.Subject)
	}

	if claims.Issuer != "https://ego.test" {
		t.Errorf("expected iss=https://ego.test, got %q", claims.Issuer)
	}

	if claims.Scope != "openid ego:read" {
		t.Errorf("expected scope=openid ego:read, got %q", claims.Scope)
	}
}

func TestParseToken_Invalid(t *testing.T) {
	setupTestKey(t)

	_, err := parseToken("not.a.jwt")
	if err == nil {
		t.Error("expected error parsing invalid token")
	}
}

func TestCreateIDToken_ContainsSubject(t *testing.T) {
	setupTestKey(t)

	cfg := asConfig{
		Issuer:          "https://ego.test",
		TokenExpiration: time.Hour,
	}

	idToken, err := createIDToken(cfg, "myclient", "alice")
	if err != nil {
		t.Fatalf("createIDToken failed: %v", err)
	}

	if idToken == "" {
		t.Error("expected non-empty ID token")
	}
}
