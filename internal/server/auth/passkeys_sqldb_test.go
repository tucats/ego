package auth

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/resources"
)

// legacyCredential builds a minimal webauthn.Credential for test fixtures.
func legacyCredential(id byte, signCount uint32) webauthn.Credential {
	return webauthn.Credential{
		ID:        []byte{id},
		PublicKey: []byte{id, id, id},
		Authenticator: webauthn.Authenticator{
			SignCount: signCount,
		},
	}
}

// TestNewDatabaseServiceMigratesLegacyPasskeys covers migrateLegacyPasskeys:
// opening a "credentials" table whose "passkeys" column still holds a JSON
// array of credentials (the pre-migration shape) must move each credential
// into its own row of the new "passkeys" table, clear the legacy column, and
// leave every other row (including a user with no passkeys) untouched.
func TestNewDatabaseServiceMigratesLegacyPasskeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "premigration.db")
	connStr := "sqlite://" + path

	oldHandle, err := resources.Open(defs.User{}, "credentials", connStr)
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}

	oldHandle.SetDefaultPrimaryKey()

	if err := oldHandle.Create(); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	migratedUserID := uuid.New()

	creds := []webauthn.Credential{legacyCredential(1, 5), legacyCredential(2, 9)}

	raw, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := oldHandle.Begin().Insert(defs.User{
		Name:     "withpasskeys",
		ID:       migratedUserID,
		Password: "hash",
		Passkeys: raw,
	}); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	// A user with no passkeys at all must survive untouched.
	if err := oldHandle.Begin().Insert(defs.User{
		Name:     "nopasskeys",
		ID:       uuid.New(),
		Password: "hash",
	}); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	oldHandle.Close()

	// Opening it the normal way must migrate the legacy column rather than
	// failing or leaving the credentials stranded where nothing reads them.
	svc, err := NewDatabaseService(connStr, "", "")
	if err != nil {
		t.Fatalf("NewDatabaseService did not migrate the pre-existing table: %v", err)
	}

	t.Cleanup(func() { _ = svc.Close() })

	migratedUser, err := svc.ReadUser(0, "withpasskeys", true)
	if err != nil {
		t.Fatalf("ReadUser after migration: %v", err)
	}

	if len(migratedUser.Passkeys) != 0 {
		t.Errorf("credentials.passkeys = %q, want empty (migration should have cleared it)", migratedUser.Passkeys)
	}

	got, err := svc.ListPasskeys(migratedUser)
	if err != nil {
		t.Fatalf("ListPasskeys after migration: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("ListPasskeys after migration returned %d credentials, want 2", len(got))
	}

	foundSignCounts := map[uint32]bool{}
	for _, c := range got {
		foundSignCounts[c.Authenticator.SignCount] = true
	}

	if !foundSignCounts[5] || !foundSignCounts[9] {
		t.Errorf("migrated credentials = %+v, want sign counts 5 and 9 present", got)
	}

	untouchedUser, err := svc.ReadUser(0, "nopasskeys", true)
	if err != nil {
		t.Fatalf("ReadUser(nopasskeys) after migration: %v", err)
	}

	count, err := svc.CountPasskeys(untouchedUser)
	if err != nil {
		t.Fatalf("CountPasskeys(nopasskeys): %v", err)
	}

	if count != 0 {
		t.Errorf("CountPasskeys(nopasskeys) = %d, want 0", count)
	}
}

// TestNewDatabaseServiceMigrationIsIdempotent confirms opening the service a
// second time -- the common case for every startup after the first one
// following an upgrade -- neither errors nor duplicates already-migrated
// rows, since the legacy column is cleared as part of the first migration.
func TestNewDatabaseServiceMigrationIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "already-migrated.db")
	connStr := "sqlite://" + path

	first, err := NewDatabaseService(connStr, "root", "password")
	if err != nil {
		t.Fatalf("first open: %v", err)
	}

	user, err := first.ReadUser(0, "root", true)
	if err != nil {
		t.Fatalf("ReadUser(root): %v", err)
	}

	if err := first.AddPasskey(user, legacyCredential(3, 1)); err != nil {
		t.Fatalf("AddPasskey: %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	second, err := NewDatabaseService(connStr, "root", "password")
	if err != nil {
		t.Fatalf("second open: %v", err)
	}

	t.Cleanup(func() { _ = second.Close() })

	user, err = second.ReadUser(0, "root", true)
	if err != nil {
		t.Fatalf("ReadUser(root) after reopen: %v", err)
	}

	count, err := second.CountPasskeys(user)
	if err != nil {
		t.Fatalf("CountPasskeys after reopen: %v", err)
	}

	if count != 1 {
		t.Errorf("CountPasskeys after reopen = %d, want 1 (migration must not run twice)", count)
	}
}

// TestPasskeyCRUD exercises AddPasskey, ListPasskeys, UpdatePasskeySignCount,
// and DeletePasskeys against a fresh (non-migrated) database.
func TestPasskeyCRUD(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crud.db")
	connStr := "sqlite://" + path

	svc, err := NewDatabaseService(connStr, "root", "password")
	if err != nil {
		t.Fatalf("NewDatabaseService: %v", err)
	}

	t.Cleanup(func() { _ = svc.Close() })

	user, err := svc.ReadUser(0, "root", true)
	if err != nil {
		t.Fatalf("ReadUser: %v", err)
	}

	if err := svc.AddPasskey(user, legacyCredential(10, 1)); err != nil {
		t.Fatalf("AddPasskey #1: %v", err)
	}

	if err := svc.AddPasskey(user, legacyCredential(20, 1)); err != nil {
		t.Fatalf("AddPasskey #2: %v", err)
	}

	count, err := svc.CountPasskeys(user)
	if err != nil {
		t.Fatalf("CountPasskeys: %v", err)
	}

	if count != 2 {
		t.Fatalf("CountPasskeys = %d, want 2", count)
	}

	if err := svc.UpdatePasskeySignCount(user, []byte{10}, 42); err != nil {
		t.Fatalf("UpdatePasskeySignCount: %v", err)
	}

	creds, err := svc.ListPasskeys(user)
	if err != nil {
		t.Fatalf("ListPasskeys: %v", err)
	}

	found := false

	for _, c := range creds {
		if len(c.ID) == 1 && c.ID[0] == 10 {
			found = true

			if c.Authenticator.SignCount != 42 {
				t.Errorf("SignCount after update = %d, want 42", c.Authenticator.SignCount)
			}
		}
	}

	if !found {
		t.Fatal("credential with ID [10] not found after update")
	}

	if err := svc.DeletePasskeys(user); err != nil {
		t.Fatalf("DeletePasskeys: %v", err)
	}

	count, err = svc.CountPasskeys(user)
	if err != nil {
		t.Fatalf("CountPasskeys after delete: %v", err)
	}

	if count != 0 {
		t.Errorf("CountPasskeys after delete = %d, want 0", count)
	}
}

// oldPasskeyRowFixture is the passkeys table shape before the CredentialID
// column was added -- used to simulate a pre-existing deployment's table
// for TestNewDatabaseServiceBackfillsCredentialID.
type oldPasskeyRowFixture struct {
	ID         string
	UserID     string
	Credential json.RawMessage
}

// TestNewDatabaseServiceBackfillsCredentialID covers
// addCredentialIDColumnIfMissing and backfillCredentialIDs: opening a
// passkeys table that predates the CredentialID column must add the column
// and populate it for every pre-existing row, not just rows written after
// the migration. UpdatePasskeySignCount's column-based lookup is used here
// as the observable proof the backfill actually happened -- it would find
// nothing (errors.ErrNotFound) if CredentialID were still blank.
func TestNewDatabaseServiceBackfillsCredentialID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentialid-backfill.db")
	connStr := "sqlite://" + path

	oldHandle, err := resources.Open(oldPasskeyRowFixture{}, passkeysTable, connStr)
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}

	oldHandle.SetPrimaryKey("ID")

	if err := oldHandle.Create(); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	userID := uuid.New()

	raw, err := json.Marshal(legacyCredential(7, 3))
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}

	if err := oldHandle.Begin().Insert(oldPasskeyRowFixture{
		ID:         uuid.New().String(),
		UserID:     userID.String(),
		Credential: raw,
	}); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	oldHandle.Close()

	svc, err := NewDatabaseService(connStr, "", "")
	if err != nil {
		t.Fatalf("NewDatabaseService did not migrate the pre-existing passkeys table: %v", err)
	}

	t.Cleanup(func() { _ = svc.Close() })

	user := defs.User{ID: userID}

	if err := svc.UpdatePasskeySignCount(user, []byte{7}, 99); err != nil {
		t.Fatalf("UpdatePasskeySignCount after backfill: %v (CredentialID was likely left blank)", err)
	}

	creds, err := svc.ListPasskeys(user)
	if err != nil {
		t.Fatalf("ListPasskeys after backfill: %v", err)
	}

	if len(creds) != 1 || creds[0].Authenticator.SignCount != 99 {
		t.Errorf("credentials after backfill + update = %+v, want one credential with SignCount 99", creds)
	}
}

// TestDeleteUserRemovesPasskeys confirms that deleting a user also removes
// their rows from the passkeys table, so a future user reusing the same
// name (with a new ID) never starts out with someone else's stale rows.
func TestDeleteUserRemovesPasskeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delete-user.db")
	connStr := "sqlite://" + path

	svc, err := NewDatabaseService(connStr, "root", "password")
	if err != nil {
		t.Fatalf("NewDatabaseService: %v", err)
	}

	t.Cleanup(func() { _ = svc.Close() })

	if err := svc.WriteUser(0, defs.User{Name: "temp", ID: uuid.New(), Password: "hash"}); err != nil {
		t.Fatalf("WriteUser: %v", err)
	}

	user, err := svc.ReadUser(0, "temp", true)
	if err != nil {
		t.Fatalf("ReadUser: %v", err)
	}

	if err := svc.AddPasskey(user, legacyCredential(1, 1)); err != nil {
		t.Fatalf("AddPasskey: %v", err)
	}

	if err := svc.DeleteUser(0, "temp"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	count, err := svc.CountPasskeys(user)
	if err != nil {
		t.Fatalf("CountPasskeys after delete user: %v", err)
	}

	if count != 0 {
		t.Errorf("CountPasskeys after delete user = %d, want 0 (orphaned passkey rows)", count)
	}
}
