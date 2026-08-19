package dsns

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// TestFileServiceAuthDSNUnrestricted is DATA-SECURITY-2.md finding #6:
// fileService.AuthDSN used to skip the Restricted check entirely, so an
// unrestricted DSN was denied to any caller without an explicit Auth entry
// -- inverting the documented default that an unrestricted DSN "is not
// gated by Ego in any way."
func TestFileServiceAuthDSNUnrestricted(t *testing.T) {
	svc, err := NewFileService("memory")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.WriteDSN(0, "owner", defs.DSN{Name: "open", Restricted: false}); err != nil {
		t.Fatal(err)
	}

	if !svc.AuthDSN(0, "nobody", "open", DSNReadAction) {
		t.Fatal("expected unrestricted DSN to authorize a caller with no Auth entry at all")
	}
}

func TestFileServiceAuthDSNRestricted(t *testing.T) {
	svc, err := NewFileService("memory")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.WriteDSN(0, "owner", defs.DSN{Name: "locked", Restricted: true}); err != nil {
		t.Fatal(err)
	}

	if svc.AuthDSN(0, "nobody", "locked", DSNReadAction) {
		t.Fatal("expected restricted DSN to deny a caller with no grant")
	}

	if err := svc.GrantDSN(0, "somebody", "locked", DSNReadAction, true); err != nil {
		t.Fatal(err)
	}

	if !svc.AuthDSN(0, "somebody", "locked", DSNReadAction) {
		t.Fatal("expected restricted DSN to authorize a caller holding a matching grant")
	}

	if svc.AuthDSN(0, "somebody", "locked", DSNWriteAction) {
		t.Fatal("expected a read-only grant not to authorize a write action")
	}
}

// TestFileServiceGrantDSNNoSuchDSN is part of finding #6: GrantDSN used to
// create an orphaned Auth entry for a DSN name that was never created,
// instead of reporting the same error databaseService.GrantDSN reports.
func TestFileServiceGrantDSNNoSuchDSN(t *testing.T) {
	svc, err := NewFileService("memory")
	if err != nil {
		t.Fatal(err)
	}

	err = svc.GrantDSN(0, "someone", "does-not-exist", DSNReadAction, true)
	if err == nil {
		t.Fatal("expected an error granting access to a nonexistent DSN, got none")
	}

	if !errors.Equal(err, errors.ErrNoSuchDSN) {
		t.Fatalf("expected ErrNoSuchDSN, got %v", err)
	}
}

// TestFileServiceGrantDSNSetsRestricted is finding #6's second half:
// GrantDSN must flip Restricted to true on the first grant against a
// previously-unrestricted DSN, matching databaseService.GrantDSN.
func TestFileServiceGrantDSNSetsRestricted(t *testing.T) {
	svc, err := NewFileService("memory")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.WriteDSN(0, "owner", defs.DSN{Name: "wasopen", Restricted: false}); err != nil {
		t.Fatal(err)
	}

	if err := svc.GrantDSN(0, "somebody", "wasopen", DSNReadAction, true); err != nil {
		t.Fatal(err)
	}

	dsn, err := svc.ReadDSN(0, "owner", "wasopen", true)
	if err != nil {
		t.Fatal(err)
	}

	if !dsn.Restricted {
		t.Fatal("expected the first grant against an unrestricted DSN to mark it Restricted")
	}
}

// TestFileServiceGrantDSNPersists confirms the grant is actually written to
// disk. The original GrantDSN never set the service's dirty flag, so
// Flush() silently no-opped and every file-backed grant was lost on
// restart unless some unrelated write happened to flush it along the way.
func TestFileServiceGrantDSNPersists(t *testing.T) {
	fileName := "test-" + uuid.NewString() + ".json"
	defer os.Remove(fileName)

	svc, err := NewFileService(fileName)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.WriteDSN(0, "owner", defs.DSN{Name: "persisted", Restricted: true}); err != nil {
		t.Fatal(err)
	}

	if err := svc.GrantDSN(0, "somebody", "persisted", DSNReadAction, true); err != nil {
		t.Fatal(err)
	}

	reloaded, err := NewFileService(fileName)
	if err != nil {
		t.Fatal(err)
	}

	if !reloaded.AuthDSN(0, "somebody", "persisted", DSNReadAction) {
		t.Fatal("expected the grant to survive a reload from disk")
	}
}

// TestFileServiceDeleteDSNRemovesAllUsersGrants is finding #6: DeleteDSN
// used to remove only the calling user's own auth record, leaving every
// other user's grant behind. Recreating a DSN of the same name would then
// silently reactivate those stale grants.
func TestFileServiceDeleteDSNRemovesAllUsersGrants(t *testing.T) {
	svc, err := NewFileService("memory")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.WriteDSN(0, "owner", defs.DSN{Name: "shared", Restricted: true}); err != nil {
		t.Fatal(err)
	}

	if err := svc.GrantDSN(0, "alice", "shared", DSNReadAction, true); err != nil {
		t.Fatal(err)
	}

	if err := svc.GrantDSN(0, "bob", "shared", DSNReadAction, true); err != nil {
		t.Fatal(err)
	}

	// "owner" is the caller performing the delete, distinct from both
	// grantees, so a bug that only clears the caller's own entry would
	// leave alice's and bob's grants untouched.
	if err := svc.DeleteDSN(0, "owner", "shared"); err != nil {
		t.Fatal(err)
	}

	if err := svc.WriteDSN(0, "owner", defs.DSN{Name: "shared", Restricted: true}); err != nil {
		t.Fatal(err)
	}

	if svc.AuthDSN(0, "alice", "shared", DSNReadAction) {
		t.Fatal("expected alice's grant on the deleted DSN not to survive recreation")
	}

	if svc.AuthDSN(0, "bob", "shared", DSNReadAction) {
		t.Fatal("expected bob's grant on the deleted DSN not to survive recreation")
	}
}

// TestFileServiceWriteDSNAssignsID is finding #6: WriteDSN never assigned
// an ID on create, so every file-backed DSN had a permanently empty "id"
// in API responses, unlike databaseService.WriteDSN's uuid.NewString().
func TestFileServiceWriteDSNAssignsID(t *testing.T) {
	svc, err := NewFileService("memory")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.WriteDSN(0, "owner", defs.DSN{Name: "idtest"}); err != nil {
		t.Fatal(err)
	}

	dsn, err := svc.ReadDSN(0, "owner", "idtest", true)
	if err != nil {
		t.Fatal(err)
	}

	if dsn.ID == "" {
		t.Fatal("expected WriteDSN to assign a non-empty ID on create")
	}

	firstID := dsn.ID

	// An update (same name, already exists) must not reassign the ID.
	dsn.Host = "changed"
	if err := svc.WriteDSN(0, "owner", dsn); err != nil {
		t.Fatal(err)
	}

	reread, err := svc.ReadDSN(0, "owner", "idtest", true)
	if err != nil {
		t.Fatal(err)
	}

	if reread.ID != firstID {
		t.Fatalf("expected ID to remain stable across an update, got %q then %q", firstID, reread.ID)
	}
}

// TestFileServiceListDSNSReturnsCopy is finding #6: ListDSNS used to
// return the service's own internal Data map. ListDSNHandler
// (internal/server/dsns/handler.go) deletes entries from the map it gets
// back when filtering out DSNs a non-admin caller can't see -- against the
// live map, that delete silently and permanently removed the DSN from the
// store.
func TestFileServiceListDSNSReturnsCopy(t *testing.T) {
	svc, err := NewFileService("memory")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.WriteDSN(0, "owner", defs.DSN{Name: "a"}); err != nil {
		t.Fatal(err)
	}

	if err := svc.WriteDSN(0, "owner", defs.DSN{Name: "b"}); err != nil {
		t.Fatal(err)
	}

	list, err := svc.ListDSNS(0, "owner")
	if err != nil {
		t.Fatal(err)
	}

	delete(list, "a")

	if _, err := svc.ReadDSN(0, "owner", "a", true); err != nil {
		t.Fatal("deleting an entry from the returned list map must not delete it from the store")
	}
}

// TestFileServiceListDSNSRedactsPassword is finding #6: ListDSNS used to
// return DSN entries verbatim, including the stored (encrypted) password
// value, unlike databaseService.ListDSNS which redacts it.
func TestFileServiceListDSNSRedactsPassword(t *testing.T) {
	svc, err := NewFileService("memory")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.WriteDSN(0, "owner", defs.DSN{Name: "secret-holder", Password: "cipher-text"}); err != nil {
		t.Fatal(err)
	}

	list, err := svc.ListDSNS(0, "owner")
	if err != nil {
		t.Fatal(err)
	}

	if list["secret-holder"].Password != defs.ElidedPassword {
		t.Fatalf("expected ListDSNS to redact the password, got %q", list["secret-holder"].Password)
	}
}
