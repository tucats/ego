package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

// TestNewStateGeneratesUniqueValues verifies that consecutive calls to newState
// produce different state and code_verifier strings.
func TestNewStateGeneratesUniqueValues(t *testing.T) {
	s1, v1, err1 := newState()
	if err1 != nil {
		t.Fatalf("newState() error: %v", err1)
	}

	s2, v2, err2 := newState()
	if err2 != nil {
		t.Fatalf("newState() error: %v", err2)
	}

	if s1 == s2 {
		t.Error("newState() returned the same state string twice")
	}

	if v1 == v2 {
		t.Error("newState() returned the same code_verifier twice")
	}
}

// TestNewStateBase64URLFormat verifies that the generated state and verifier
// are valid base64url-encoded strings (no padding characters).
func TestNewStateBase64URLFormat(t *testing.T) {
	state, verifier, err := newState()
	if err != nil {
		t.Fatalf("newState() error: %v", err)
	}

	// base64url without padding: no '+', '/', or '=' characters.
	for _, ch := range []byte(state) {
		if ch == '+' || ch == '/' || ch == '=' {
			t.Errorf("state contains non-base64url character %q", ch)
		}
	}

	for _, ch := range []byte(verifier) {
		if ch == '+' || ch == '/' || ch == '=' {
			t.Errorf("verifier contains non-base64url character %q", ch)
		}
	}

	// 32 random bytes → 43 characters in base64url (ceil(32*8/6) = 43).
	if len(state) != 43 {
		t.Errorf("state length = %d, want 43", len(state))
	}

	if len(verifier) != 43 {
		t.Errorf("verifier length = %d, want 43", len(verifier))
	}
}

// TestValidateStateSucceeds verifies that a freshly generated state can be
// validated and that the stored code_verifier is returned correctly.
func TestValidateStateSucceeds(t *testing.T) {
	state, verifier, err := newState()
	if err != nil {
		t.Fatalf("newState() error: %v", err)
	}

	ps, err := validateState(state)
	if err != nil {
		t.Fatalf("validateState() unexpected error: %v", err)
	}

	if ps.CodeVerifier != verifier {
		t.Errorf("CodeVerifier = %q, want %q", ps.CodeVerifier, verifier)
	}
}

// TestValidateStateSingleUse verifies that a state can only be validated once.
func TestValidateStateSingleUse(t *testing.T) {
	state, _, err := newState()
	if err != nil {
		t.Fatalf("newState() error: %v", err)
	}

	// First validation should succeed.
	if _, err := validateState(state); err != nil {
		t.Fatalf("first validateState() error: %v", err)
	}

	// Second validation of the same state must fail.
	if _, err := validateState(state); err == nil {
		t.Error("second validateState() should have returned an error (single-use)")
	}
}

// TestValidateStateUnknown verifies that validating an unknown state returns an error.
func TestValidateStateUnknown(t *testing.T) {
	_, err := validateState("totally-unknown-state-string")
	if err == nil {
		t.Error("validateState() with unknown state should return an error")
	}
}

// TestValidateStateExpired verifies that a state older than stateMaxAge is rejected.
func TestValidateStateExpired(t *testing.T) {
	state, _, err := newState()
	if err != nil {
		t.Fatalf("newState() error: %v", err)
	}

	// Backdate the entry to simulate an expired state.
	stateStore.mu.Lock()
	stateStore.items[state].CreatedAt = time.Now().Add(-(stateMaxAge + time.Second))
	stateStore.mu.Unlock()

	_, err = validateState(state)
	if err == nil {
		t.Error("validateState() with expired state should return an error")
	}

	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error message should mention 'expired', got: %v", err)
	}
}

// TestPurgeExpiredStates verifies that purgeExpiredStates removes old entries and
// leaves fresh entries intact.
func TestPurgeExpiredStates(t *testing.T) {
	freshState, _, err := newState()
	if err != nil {
		t.Fatalf("newState() fresh error: %v", err)
	}

	expiredState, _, err := newState()
	if err != nil {
		t.Fatalf("newState() expired error: %v", err)
	}

	// Backdate only the expired entry.
	stateStore.mu.Lock()
	stateStore.items[expiredState].CreatedAt = time.Now().Add(-(stateMaxAge + time.Second))
	stateStore.mu.Unlock()

	purgeExpiredStates()

	// The expired entry should be gone.
	stateStore.mu.Lock()
	_, hasExpired := stateStore.items[expiredState]
	_, hasFresh := stateStore.items[freshState]
	stateStore.mu.Unlock()

	if hasExpired {
		t.Error("purgeExpiredStates() should have removed the expired entry")
	}

	if !hasFresh {
		t.Error("purgeExpiredStates() should have kept the fresh entry")
	}

	// Clean up the fresh entry so it does not affect other tests.
	validateState(freshState) //nolint:errcheck
}

// TestCodeChallenge verifies that codeChallenge produces the correct S256 output.
// The expected value is computed independently using the RFC 7636 formula:
//
//	BASE64URL(SHA256(ASCII(code_verifier)))
func TestCodeChallenge(t *testing.T) {
	// Use a well-known verifier from RFC 7636 Appendix B.
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

	// Compute the expected challenge independently.
	h := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])

	got := codeChallenge(verifier)
	if got != expected {
		t.Errorf("codeChallenge(%q) = %q, want %q", verifier, got, expected)
	}
}
