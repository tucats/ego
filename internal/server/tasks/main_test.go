package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/server/auth"
)

// authTestFile is the temporary user-database file backing the minimal auth
// service used by dispatch tests that exercise a real router.ServeHTTP call.
var authTestFile = filepath.Join(os.TempDir(), fmt.Sprintf("ego_tasks_auth_test-%s.json", uuid.New().String()))

// TestMain initializes a minimal file-backed auth service so that
// auth.GetPermissions calls inside router.Authenticate don't panic on a nil
// AuthService when a dispatch test's minted token is validated, and sets
// defs.InstanceID (required by tokens.New) to a valid UUID -- both of
// which a real server's startup sequence would normally have done first.
// This mirrors internal/router/auth_test.go's own TestMain.
func TestMain(m *testing.M) {
	svc, err := auth.NewFileService(authTestFile, defs.DefaultAdminUsername, defs.DefaultAdminPassword)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tasks_test: failed to create test auth service: %v\n", err)
		os.Exit(1)
	}

	auth.AuthService = svc
	defs.InstanceID = uuid.New().String()

	code := m.Run()

	_ = os.Remove(authTestFile)
	os.Exit(code)
}
