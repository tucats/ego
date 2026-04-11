package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
)

var (
	testFile         string = filepath.Join(os.TempDir(), fmt.Sprintf("ego_test_auth-%s.json", uuid.New().String()))
	savedAuthService userIOService
)

// setupTestAuthService creates a temporary file-backed auth service and seeds it
// with a mix of hash formats so both the legacy migration path and the current
// bcrypt path are exercised by the tests.
//
//   - "payroll"  — SHA-256 hash (legacy format, should trigger migration on login)
//   - "staff"    — bcrypt hash   (current format)
//   - "bogus"    — SHA-256 hash, but lacks logon/root permission (always rejected)
func setupTestAuthService(t *testing.T) {
	t.Helper()

	var err error

	savedAuthService = AuthService

	AuthService, err = NewFileService(testFile, defs.DefaultAdminUsername, defs.DefaultAdminPassword)
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	// payroll: legacy SHA-256 hash — exercises the migration path.
	_ = AuthService.WriteUser(0, defs.User{
		Name:        "payroll",
		Password:    egostrings.HashString("payroll1"),
		Permissions: []string{defs.RootPermission, "checks"},
	})

	// staff: current bcrypt hash — exercises the bcrypt validation path.
	staffHash, err := HashPassword("quidditch")
	if err != nil {
		t.Fatalf("Failed to hash staff password: %v", err)
	}

	_ = AuthService.WriteUser(0, defs.User{
		Name:        "staff",
		Password:    staffHash,
		Permissions: []string{defs.LogonPermission, "tables"},
	})

	// bogus: has no logon/root permission — always rejected regardless of password.
	_ = AuthService.WriteUser(0, defs.User{
		Name:        "bogus",
		Password:    egostrings.HashString("zork"),
		Permissions: []string{"employees"},
	})
}

// teardown restores the original AuthService and removes the temporary file.
func teardownTestAuthService(t *testing.T, ignoreErrors bool) {
	t.Helper()

	AuthService = savedAuthService

	err := os.Remove(testFile)
	if !ignoreErrors && err != nil {
		t.Fatalf("Failed to remove test file: %v", err)
	}
}

// --- Reject cases ---

func TestValidatePassword_EmptyUser(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	if ValidatePassword(0, "", "password123") {
		t.Error("expected false for empty username")
	}
}

func TestValidatePassword_UserDoesNotExist(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	if ValidatePassword(0, "nonexistent", "password123") {
		t.Error("expected false for unknown user")
	}
}

func TestValidatePassword_EmptyPassword(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	if ValidatePassword(0, "payroll", "") {
		t.Error("expected false for empty password")
	}
}

func TestValidatePassword_InvalidPassword(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	if ValidatePassword(0, "payroll", "wrongpassword") {
		t.Error("expected false for wrong password (SHA-256 user)")
	}

	if ValidatePassword(0, "staff", "wrongpassword") {
		t.Error("expected false for wrong password (bcrypt user)")
	}

	// bogus has no logon/root permission, so even the correct password is rejected.
	if ValidatePassword(0, "bogus", "zork") {
		t.Error("expected false for user without logon permission")
	}
}

// --- Accept cases (bcrypt path) ---

func TestValidatePassword_BcryptUser(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	if !ValidatePassword(0, "staff", "quidditch") {
		t.Error("expected true for valid bcrypt-hashed password")
	}
}

// --- Accept cases (legacy SHA-256 path + migration) ---

func TestValidatePassword_LegacySHA256User(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	if !ValidatePassword(0, "payroll", "payroll1") {
		t.Error("expected true for valid legacy SHA-256 password")
	}
}

// TestValidatePassword_MigrationUpgradesToBcrypt verifies that a successful
// login with a legacy SHA-256 hash rewrites the stored hash to bcrypt, so the
// user's next login goes through the bcrypt path.
func TestValidatePassword_MigrationUpgradesToBcrypt(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	// Confirm the stored hash is NOT bcrypt before login.
	before, err := AuthService.ReadUser(0, "payroll", true)
	if err != nil {
		t.Fatalf("ReadUser before migration: %v", err)
	}

	if IsBcryptHash(before.Password) {
		t.Fatal("precondition failed: payroll password should be SHA-256 before first login")
	}

	// Login succeeds using the legacy path.
	if !ValidatePassword(0, "payroll", "payroll1") {
		t.Fatal("expected login to succeed for legacy user")
	}

	// After login the stored hash must have been upgraded to bcrypt.
	after, err := AuthService.ReadUser(0, "payroll", true)
	if err != nil {
		t.Fatalf("ReadUser after migration: %v", err)
	}

	if !IsBcryptHash(after.Password) {
		t.Errorf("expected bcrypt hash after migration, got %q", after.Password)
	}

	// The migrated hash must still validate the same password.
	if !ValidatePassword(0, "payroll", "payroll1") {
		t.Error("expected login to succeed after migration to bcrypt")
	}
}

// TestValidatePassword_LegacyQuotedFormat verifies that passwords stored in
// the legacy {quoted} format are accepted and migrated.
func TestValidatePassword_LegacyQuotedFormat(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	// Store the password in the legacy {quoted} format.
	_ = AuthService.WriteUser(0, defs.User{
		Name:        "quoted",
		Password:    "{hunter2}",
		Permissions: []string{defs.LogonPermission},
	})

	if !ValidatePassword(0, "quoted", "hunter2") {
		t.Error("expected true for valid {quoted} format password")
	}

	// Confirm the hash was upgraded to bcrypt.
	after, err := AuthService.ReadUser(0, "quoted", true)
	if err != nil {
		t.Fatalf("ReadUser after migration: %v", err)
	}

	if !IsBcryptHash(after.Password) {
		t.Errorf("expected bcrypt hash after {quoted} migration, got %q", after.Password)
	}
}

// --- IsBcryptHash unit tests ---

func TestIsBcryptHash(t *testing.T) {
	tests := []struct {
		hash string
		want bool
	}{
		{"$2a$12$somehashvalue", true},
		{"$2b$12$somehashvalue", true},
		{"$2y$12$somehashvalue", true},
		{"e3b0c44298fc1c149afbf4c8996fb924", false}, // SHA-256 hex
		{"{plaintext}", false},
		{"", false},
	}

	for _, tc := range tests {
		if got := IsBcryptHash(tc.hash); got != tc.want {
			t.Errorf("IsBcryptHash(%q) = %v, want %v", tc.hash, got, tc.want)
		}
	}
}

// --- HashString (SHA-256) regression tests ---
// These ensure the existing SHA-256 function remains correct, since it is
// still used for non-password purposes throughout the codebase.

func TestHashString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "Empty string",
			in:   "",
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "Single character",
			in:   "a",
			want: "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
		},
		{
			name: "Special characters",
			in:   "!@#$%^&*()_+=-{}[]|:;<>,.?/~",
			want: "a6e7f1154ddc33c92e25e5dc439968a9520a1fe18f602e486be98cbd0af05ce9",
		},
		{
			name: "Long string",
			in:   "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+=-{}[]|:;<>,.?/~",
			want: "f61ea696fa12f59c34928b73c33335a710628f15dfd6f36eeb7d24074d5d3d91",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := egostrings.HashString(tc.in)
			if got != tc.want {
				t.Errorf("HashString(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
