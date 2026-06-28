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

// ─────────────────────────────────────────────────────────────────────────────
// L1: content-derived JWKS kid
// ─────────────────────────────────────────────────────────────────────────────

// TestBuildJWKS_KidIsNotStatic verifies that the "kid" field in the JWKS
// document is NOT the static string "1" (L1).
//
// Why the static "1" was a problem:
//
//	When the signing key is rotated (e.g., the key file is deleted so a new one
//	is generated on the next restart), a Resource Server that has cached the old
//	JWKS cannot detect the rotation: both the old and new JWKS documents contain
//	kid="1".  The RS keeps verifying new JWTs against the old public key, causing
//	401 errors until the RS JWKS cache expires (up to 1 hour by default).
//
//	With a content-derived kid, the new key has a different kid value.  RS
//	libraries see the new kid in the JWT header, find it missing from their
//	cached JWKS, and immediately re-fetch the updated key set.
func TestBuildJWKS_KidIsNotStatic(t *testing.T) {
	dir := t.TempDir()
	if err := loadOrGenerateKey(filepath.Join(dir, "test.pem")); err != nil {
		t.Fatalf("setup: %v", err)
	}

	kid := extractKid(t, jwksJSON)

	// Before the L1 fix the kid was always "1".  After the fix it is a hex
	// fingerprint of the public key.  Either the length or the format will
	// differ from "1".
	if kid == "1" {
		t.Error("L1: JWKS kid must not be the static string '1'; it should be derived from the public key fingerprint")
	}
}

// TestBuildJWKS_KidIsHexFingerprint verifies that the kid is exactly 16
// lowercase hexadecimal characters — the first 8 bytes of SHA-256(DER public
// key) encoded as hex (L1).
//
// 16 hex chars = 8 bytes = 64 bits of the fingerprint.  This provides
// ample uniqueness for identifying a single key pair while keeping the
// kid short enough to be human-readable in logs.
func TestBuildJWKS_KidIsHexFingerprint(t *testing.T) {
	dir := t.TempDir()
	if err := loadOrGenerateKey(filepath.Join(dir, "test.pem")); err != nil {
		t.Fatalf("setup: %v", err)
	}

	kid := extractKid(t, jwksJSON)

	// Must be exactly 16 characters (8 bytes × 2 hex digits each).
	if len(kid) != 16 {
		t.Errorf("L1: kid length = %d, want 16 (hex fingerprint of first 8 SHA-256 bytes); kid = %q", len(kid), kid)
	}

	// Must be valid lowercase hex.
	for i, c := range kid {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("L1: kid[%d] = %q is not a lowercase hex digit; kid = %q", i, c, kid)
		}
	}
}

// TestBuildJWKS_DifferentKeysProduceDifferentKids verifies that two distinct
// key pairs produce different kid values (L1 collision resistance).
//
// If two different keys produced the same kid, a Resource Server would fail
// to distinguish them during key rotation, defeating the purpose of the fix.
func TestBuildJWKS_DifferentKeysProduceDifferentKids(t *testing.T) {
	dir1 := t.TempDir()
	if err := loadOrGenerateKey(filepath.Join(dir1, "key1.pem")); err != nil {
		t.Fatalf("key 1 setup: %v", err)
	}

	kid1 := extractKid(t, jwksJSON)

	dir2 := t.TempDir()
	if err := loadOrGenerateKey(filepath.Join(dir2, "key2.pem")); err != nil {
		t.Fatalf("key 2 setup: %v", err)
	}

	kid2 := extractKid(t, jwksJSON)

	// Two independently generated P-256 keys must produce different SHA-256
	// fingerprints.  A collision here would indicate a bug in key generation
	// or fingerprinting logic.
	if kid1 == kid2 {
		t.Errorf("L1: two independently generated keys produced the same kid %q — possible fingerprint bug", kid1)
	}
}

// TestBuildJWKS_SameKeyProducesSameKid verifies that reloading the same key
// from disk produces the same kid (L1 stability across restarts).
//
// If the kid changed on every restart (even when the key file is the same),
// Resource Servers would see an unexpected kid on every restart and re-fetch
// the JWKS on every single JWT — eliminating the caching benefit entirely.
func TestBuildJWKS_SameKeyProducesSameKid(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "stable.pem")

	// Generate and record the kid.
	if err := loadOrGenerateKey(keyFile); err != nil {
		t.Fatalf("first load: %v", err)
	}

	kid1 := extractKid(t, jwksJSON)

	// Reload the same key file and record the kid again.
	if err := loadOrGenerateKey(keyFile); err != nil {
		t.Fatalf("second load: %v", err)
	}

	kid2 := extractKid(t, jwksJSON)

	if kid1 != kid2 {
		t.Errorf("L1: kid changed between reloads of the same key file: %q → %q", kid1, kid2)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// L2: atomic key file write
// ─────────────────────────────────────────────────────────────────────────────

// TestLoadOrGenerateKey_WritesCorrectPermissions verifies that the PEM file
// written by loadOrGenerateKey has mode 0600 (owner read/write only) (L2).
//
// The L2 fix uses os.Chmod before the os.Rename, so the file is never
// world-readable even transiently.  This test confirms the final permission
// is correct after both steps.
func TestLoadOrGenerateKey_WritesCorrectPermissions(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "perms.pem")

	if err := loadOrGenerateKey(keyFile); err != nil {
		t.Fatalf("loadOrGenerateKey: %v", err)
	}

	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}

	// Only the lower 9 permission bits matter.
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("L2: key file permissions = %04o, want 0600", perm)
	}
}

// TestLoadOrGenerateKey_NoTempFileLeftBehind verifies that the atomic write
// process (write temp file → rename → cleanup) leaves no orphan temp files
// in the key directory after a successful write (L2).
//
// os.Rename atomically replaces the target, so the temp file path disappears.
// The deferred os.Remove in loadOrGenerateKey is a no-op in the success case
// but this test confirms the directory is clean regardless.
func TestLoadOrGenerateKey_NoTempFileLeftBehind(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "clean.pem")

	if err := loadOrGenerateKey(keyFile); err != nil {
		t.Fatalf("loadOrGenerateKey: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	for _, e := range entries {
		name := e.Name()

		// The only file allowed is the final PEM.  Any ".tmp" file would be
		// an orphan left by a failed or incomplete atomic write.
		if name != filepath.Base(keyFile) {
			t.Errorf("L2: unexpected file in key directory after write: %q (possible temp-file leak)", name)
		}
	}
}

// extractKid is a test helper that unmarshals a JWKS JSON byte slice and
// returns the "kid" field of the first key entry.  It calls t.Fatal if the
// JSON is malformed or the expected fields are missing.
func extractKid(t *testing.T, jwks []byte) string {
	t.Helper()

	var keySet map[string]any
	if err := json.Unmarshal(jwks, &keySet); err != nil {
		t.Fatalf("extractKid: JWKS is not valid JSON: %v", err)
	}

	keys, ok := keySet["keys"].([]any)
	if !ok || len(keys) == 0 {
		t.Fatal("extractKid: JWKS missing 'keys' array")
	}

	key, ok := keys[0].(map[string]any)
	if !ok {
		t.Fatal("extractKid: first key entry is not an object")
	}

	kid, ok := key["kid"].(string)
	if !ok || kid == "" {
		t.Fatalf("extractKid: 'kid' field is missing or not a string; key = %v", key)
	}

	return kid
}
