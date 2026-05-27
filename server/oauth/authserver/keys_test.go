package authserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrGenerateKey_GeneratesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "test.pem")

	// Key file does not exist yet — should be generated.
	if err := loadOrGenerateKey(keyFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if signingKey == nil {
		t.Fatal("signingKey should not be nil after generation")
	}

	if jwksJSON == nil {
		t.Fatal("jwksJSON should not be nil after generation")
	}

	// The PEM file should have been written to disk.
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		t.Fatal("expected PEM file to be written to disk")
	}
}

func TestLoadOrGenerateKey_LoadsExisting(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "test.pem")

	// Generate first.
	if err := loadOrGenerateKey(keyFile); err != nil {
		t.Fatalf("generation error: %v", err)
	}

	firstKey := signingKey

	// Load again from disk — should read the same key.
	if err := loadOrGenerateKey(keyFile); err != nil {
		t.Fatalf("load error: %v", err)
	}

	// Compare the public key X coordinate — if they match the same key was loaded.
	if signingKey.PublicKey.X.Cmp(firstKey.PublicKey.X) != 0 {
		t.Error("reloaded key differs from generated key")
	}
}

func TestBuildJWKS_CorrectStructure(t *testing.T) {
	dir := t.TempDir()
	if err := loadOrGenerateKey(filepath.Join(dir, "test.pem")); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	var keySet map[string]any
	if err := json.Unmarshal(jwksJSON, &keySet); err != nil {
		t.Fatalf("JWKS is not valid JSON: %v", err)
	}

	keys, ok := keySet["keys"].([]any)
	if !ok || len(keys) == 0 {
		t.Fatal("JWKS missing 'keys' array")
	}

	key, ok := keys[0].(map[string]any)
	if !ok {
		t.Fatal("first key entry is not an object")
	}

	for _, field := range []string{"kty", "crv", "use", "alg", "kid", "x", "y"} {
		if _, ok := key[field]; !ok {
			t.Errorf("JWKS key missing field %q", field)
		}
	}

	if key["kty"] != "EC" {
		t.Errorf("expected kty=EC, got %v", key["kty"])
	}

	if key["crv"] != "P-256" {
		t.Errorf("expected crv=P-256, got %v", key["crv"])
	}

	if key["alg"] != "ES256" {
		t.Errorf("expected alg=ES256, got %v", key["alg"])
	}
}
