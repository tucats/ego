package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

var (
	testFile         string = filepath.Join(os.TempDir(), fmt.Sprintf("ego_test_auth-%s.json", uuid.New().String()))
	savedAuthService userIOService
)

func setupTestAuthService(t *testing.T) {
	var err error

	savedAuthService = AuthService

	AuthService, err = NewFileService(testFile, "admin", "password")
	if err != nil {
		t.Fatalf("Failed to create auth service: %v", err)
	}

	// Seed the database with users
	
	_ = AuthService.WriteUser(defs.User{
		Name:        "payroll",
		Password:    HashString("payroll1"),
		Permissions: []string{"root", "checks"},
	})

	_ = AuthService.WriteUser(defs.User{
		Name:        "staff",
		Password:    HashString("quidditch"),
		Permissions: []string{"logon", "tables"},
	})

	_ = AuthService.WriteUser(defs.User{
		Name:        "bogus",
		Password:    HashString("zork"),
		Permissions: []string{"employees"},
	})
}

// tear down the testing authorization service. If ignoreErrors is true,
// any error is deleting the file is ignored. Note that this is needed when
// the test does not ever flush the database to the file system.
func teardownTestAuthService(t *testing.T, ignoreErrors bool) {
	AuthService = savedAuthService

	err := os.Remove(testFile)
	if !ignoreErrors && err != nil {
		t.Fatalf("Failed to remove test file: %v", err)
	}
}

func TestValidatePassword_EmptyUser(t *testing.T) {
	// Arrange
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	user := ""
	pass := "password123"

	// Act
	result := ValidatePassword(user, pass)

	// Assert
	if result {
		t.Error("Expected ValidatePassword to return false for empty user input")
	}
}

func TestValidatePassword_UserDoesNotExist(t *testing.T) {
	// Arrange
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	user := "nonexistentUser"
	pass := "password123"

	// Act
	result := ValidatePassword(user, pass)

	// Assert
	if result {
		t.Error("Expected ValidatePassword to return false for a user that does not exist")
	}
}

func TestValidatePassword_EmptyPassword(t *testing.T) {
	// Arrange
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	user := "payroll"
	pass := ""

	// Act
	result := ValidatePassword(user, pass)

	// Assert
	if result {
		t.Error("Expected ValidatePassword to return false for an empty password")
	}
}

func TestValidatePassword_InvalidPassword(t *testing.T) {
	// Arrange
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	// Act - wrong password for payroll user
	result := ValidatePassword("payroll", "zorp")

	// Assert
	if result {
		t.Error("Expected ValidatePassword to return false for an empty password")
	}

	// Act - user "bogus" does not allow logons, so password check is always false
	result = ValidatePassword("bogus", "zork")

	// Assert
	if result {
		t.Error("Expected ValidatePassword to return false for an empty password")
	}
}

func TestValidatePassword_ValidPasswordRoot(t *testing.T) {
	// Arrange
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	// Act
	result := ValidatePassword("payroll", "payroll1")

	// Assert
	if !result {
		t.Error("Expected ValidPassword to return true for a valid password")
	}

	// Act
	result = ValidatePassword("staff", "quidditch")

	// Assert
	if !result {
		t.Error("Expected ValidPassword to return true for a valid password")
	}
}

func TestValidatePermission_Allowed(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	s := symbols.NewSymbolTable("validate")

	result, err := Permission(s, data.NewList("payroll", "checks"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if b, ok := result.(bool); !ok || !b {
		t.Error("Expected Permission to return true for a valid permission")
	}
}

func TestValidatePermission_Disallowed(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	s := symbols.NewSymbolTable("validate")

	// Disallowed because permission is not in the user's list
	result, err := Permission(s, data.NewList("payroll", "reports"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if b, ok := result.(bool); !ok || b {
		t.Error("Expected Permission to return false for an invalid permission")
	}
}

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

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := HashString(test.in)
			if got != test.want {
				t.Errorf("HashString(%s) = %s, want %s", test.in, got, test.want)
			}
		})
	}
}
