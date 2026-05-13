package users

// Tests for the five HTTP handlers in the server/admin/users package:
//
//   GetUserHandler     – GET    /admin/users/{name}
//   ListUsersHandler   – GET    /admin/users
//   CreateUserHandler  – POST   /admin/users
//   UpdateUserHandler  – PATCH  /admin/users/{name}
//   DeleteUserHandler  – DELETE /admin/users/{name}
//
// Additionally, getUserFromBody (the shared request-body parser) is tested
// directly.
//
// Each test installs a temporary file-based auth service (same technique as
// server/auth/validate_test.go), uses net/http/httptest to capture responses,
// and asserts on status codes and decoded JSON bodies.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/auth"
)

// ---------------------------------------------------------------------------
// Test infrastructure
// ---------------------------------------------------------------------------

var testFile = filepath.Join(os.TempDir(), fmt.Sprintf("ego_test_users-%s.json", uuid.New().String()))
var userName1 = "alice" + strconv.Itoa(rand.IntN(100))
var userName2 = "bob" + strconv.Itoa(rand.IntN(100))

// setupTestAuthService replaces the global auth.AuthService with a fresh
// file-based service backed by a temporary file and seeds it with a handful
// of known users.
func setupTestAuthService(t *testing.T) {
	t.Helper()

	var err error

	auth.AuthService, err = auth.NewFileService(testFile, defs.DefaultAdminUsername, defs.DefaultAdminPassword)
	if err != nil {
		t.Fatalf("create test auth service: %v", err)
	}

	mustWrite := func(u defs.User) {
		if err := auth.AuthService.WriteUser(0, u); err != nil {
			t.Fatalf("seed user %q: %v", u.Name, err)
		}
	}

	mustWrite(defs.User{
		Name:        userName1,
		Password:    egostrings.HashString("secret"),
		Permissions: []string{defs.LogonPermission},
	})

	mustWrite(defs.User{
		Name:        userName2,
		Password:    egostrings.HashString("pass"),
		Permissions: []string{defs.LogonPermission, "custom.tables"},
	})
}

// teardownTestAuthService sets auth.AuthService back to nil (its zero value
// before any server is started) and removes the temporary database file.
func teardownTestAuthService(t *testing.T, ignoreErrors bool) {
	t.Helper()

	auth.AuthService = nil

	if err := os.Remove(testFile); !ignoreErrors && err != nil {
		t.Fatalf("remove test file: %v", err)
	}
}

// makeSession builds a minimal *router.Session for use in handler calls.
//   - urlParts is the map of named path segments (e.g. {"name": testUserName}).
//   - params   is the map of query-string parameters (may be nil).
func makeSession(urlParts map[string]any, params map[string][]string) *router.Session {
	if params == nil {
		params = map[string][]string{}
	}

	if urlParts == nil {
		urlParts = map[string]any{}
	}

	return &router.Session{
		ID:         1,
		URLParts:   urlParts,
		Parameters: params,
	}
}

// newRequest creates an *http.Request with an optional JSON body.
// Pass nil for body on requests that carry no payload (GET, DELETE).
func newRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()

	var bodyBytes []byte

	if body != nil {
		var err error

		bodyBytes, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}

	req, err := http.NewRequest(method, path, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	return req
}

// decodeUserResponse decodes the JSON written to rr into a defs.UserResponse.
func decodeUserResponse(t *testing.T, rr *httptest.ResponseRecorder) defs.UserResponse {
	t.Helper()

	var resp defs.UserResponse

	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode UserResponse: %v\nbody: %s", err, rr.Body.String())
	}

	return resp
}

// decodeUserCollection decodes the JSON written to rr into a defs.UserCollection.
func decodeUserCollection(t *testing.T, rr *httptest.ResponseRecorder) defs.UserCollection {
	t.Helper()

	var coll defs.UserCollection

	if err := json.Unmarshal(rr.Body.Bytes(), &coll); err != nil {
		t.Fatalf("decode UserCollection: %v\nbody: %s", err, rr.Body.String())
	}

	return coll
}

// ---------------------------------------------------------------------------
// getUserFromBody tests
// ---------------------------------------------------------------------------

func TestGetUserFromBody_ValidJSON(t *testing.T) {
	body := defs.User{Name: userName1, Password: "secret"}
	req := newRequest(t, http.MethodPost, "/admin/users", body)

	u, err := getUserFromBody(req, makeSession(nil, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if u.Name != userName1 {
		t.Errorf("expected name 'alice', got %q", u.Name)
	}

	if u.Password != "secret" {
		t.Errorf("expected password 'secret', got %q", u.Password)
	}
}

func TestGetUserFromBody_InvalidJSON(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "/admin/users", bytes.NewBufferString("not json"))

	_, err := getUserFromBody(req, makeSession(nil, nil))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestGetUserFromBody_EmptyBody(t *testing.T) {
	// An empty body is valid JSON for an empty struct — should parse without error.
	req := newRequest(t, http.MethodPost, "/admin/users", defs.User{})

	u, err := getUserFromBody(req, makeSession(nil, nil))
	if err != nil {
		t.Fatalf("unexpected error for empty struct body: %v", err)
	}

	if u == nil {
		t.Fatal("expected non-nil user")
	}
}

func TestGetUserFromBody_PermissionsInitialized(t *testing.T) {
	// Even when no permissions are in the body, the returned slice must be
	// non-nil so callers can safely call len() on it.
	req := newRequest(t, http.MethodPost, "/admin/users", defs.User{Name: "carol"})

	u, err := getUserFromBody(req, makeSession(nil, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if u.Permissions == nil {
		t.Error("expected non-nil Permissions slice")
	}
}

// ---------------------------------------------------------------------------
// GetUserHandler tests
// ---------------------------------------------------------------------------

func TestGetUserHandler_ExistingUser(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	session := makeSession(map[string]any{"name": userName1}, nil)
	status := GetUserHandler(session, rr, newRequest(t, http.MethodGet, "/admin/users/alice", nil))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}

	resp := decodeUserResponse(t, rr)
	if resp.Name != userName1 {
		t.Errorf("expected name 'alice', got %q", resp.Name)
	}
}

func TestGetUserHandler_PasswordElided(t *testing.T) {
	// The password field in the response must never contain the real hash.
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	GetUserHandler(makeSession(map[string]any{"name": userName1}, nil), rr,
		newRequest(t, http.MethodGet, "/admin/users/alice", nil))

	resp := decodeUserResponse(t, rr)
	if resp.Password == egostrings.HashString("secret") {
		t.Error("response must not contain the password hash")
	}

	if resp.Password != defs.ElidedPassword && resp.Password != "" {
		t.Errorf("expected elided password, got %q", resp.Password)
	}
}

func TestGetUserHandler_NonexistentUser(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	status := GetUserHandler(
		makeSession(map[string]any{"name": "nobody"}, nil),
		rr,
		newRequest(t, http.MethodGet, "/admin/users/nobody", nil),
	)

	if status != http.StatusNotFound {
		t.Errorf("expected 404, got %d", status)
	}
}

// ---------------------------------------------------------------------------
// ListUsersHandler tests
// ---------------------------------------------------------------------------

func TestListUsersHandler_ReturnsOK(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	status := ListUsersHandler(makeSession(nil, nil), rr, newRequest(t, http.MethodGet, "/admin/users", nil))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}
}

func TestListUsersHandler_ContainsSeededUsers(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	ListUsersHandler(makeSession(nil, nil), rr, newRequest(t, http.MethodGet, "/admin/users", nil))

	coll := decodeUserCollection(t, rr)

	names := map[string]bool{}
	for _, u := range coll.Items {
		names[u.Name] = true
	}

	for _, expected := range []string{userName1, userName2} {
		if !names[expected] {
			t.Errorf("expected user %q in list", expected)
		}
	}
}

func TestListUsersHandler_SortedByName(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	ListUsersHandler(makeSession(nil, nil), rr, newRequest(t, http.MethodGet, "/admin/users", nil))

	coll := decodeUserCollection(t, rr)

	for i := 1; i < len(coll.Items); i++ {
		if coll.Items[i-1].Name > coll.Items[i].Name {
			t.Errorf("items not sorted: %q > %q", coll.Items[i-1].Name, coll.Items[i].Name)
		}
	}
}

func TestListUsersHandler_CountMatchesItems(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	ListUsersHandler(makeSession(nil, nil), rr, newRequest(t, http.MethodGet, "/admin/users", nil))

	coll := decodeUserCollection(t, rr)

	if coll.Count != len(coll.Items) {
		t.Errorf("Count %d does not match len(Items) %d", coll.Count, len(coll.Items))
	}
}

func TestListUsersHandler_PasswordsNotReturned(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	ListUsersHandler(makeSession(nil, nil), rr, newRequest(t, http.MethodGet, "/admin/users", nil))

	coll := decodeUserCollection(t, rr)

	for _, u := range coll.Items {
		if u.Password != "" {
			t.Errorf("user %q: expected empty password, got %q", u.Name, u.Password)
		}
	}
}

// ---------------------------------------------------------------------------
// CreateUserHandler tests
// ---------------------------------------------------------------------------

func TestCreateUserHandler_CreatesUser(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	body := defs.User{Name: "carol", Password: "pass123", Permissions: []string{}}
	rr := httptest.NewRecorder()
	status := CreateUserHandler(makeSession(nil, nil), rr, newRequest(t, http.MethodPost, "/admin/users", body))

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", status, rr.Body.String())
	}

	// Verify the user now exists in the store.
	_, err := auth.AuthService.ReadUser(0, "carol", false)
	if err != nil {
		t.Errorf("user 'carol' not found after creation: %v", err)
	}
}

func TestCreateUserHandler_ResponsePasswordElided(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	body := defs.User{Name: "dave", Password: "hunter2", Permissions: []string{}}
	rr := httptest.NewRecorder()
	CreateUserHandler(makeSession(nil, nil), rr, newRequest(t, http.MethodPost, "/admin/users", body))

	resp := decodeUserResponse(t, rr)
	if resp.Password == egostrings.HashString("hunter2") {
		t.Error("response must not contain the password hash")
	}
}

func TestCreateUserHandler_InvalidPermission(t *testing.T) {
	// A permission that begins with "ego." but is not in AllPermissions should
	// produce a 400 Bad Request.
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	body := defs.User{Name: "eve", Password: "x", Permissions: []string{"ego.nonexistent"}}
	rr := httptest.NewRecorder()
	status := CreateUserHandler(makeSession(nil, nil), rr, newRequest(t, http.MethodPost, "/admin/users", body))

	if status != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid permission, got %d", status)
	}
}

func TestCreateUserHandler_AmbiguousPermission(t *testing.T) {
	// A permission without the "ego." prefix that matches a known permission
	// after adding it is ambiguous and should return 400.
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	// "logon" would match "ego.logon" — this is the ambiguous case.
	body := defs.User{Name: "frank", Password: "x", Permissions: []string{"logon"}}
	rr := httptest.NewRecorder()
	status := CreateUserHandler(makeSession(nil, nil), rr, newRequest(t, http.MethodPost, "/admin/users", body))

	if status != http.StatusBadRequest {
		t.Errorf("expected 400 for ambiguous permission, got %d", status)
	}
}

func TestCreateUserHandler_InvalidBody(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	req, _ := http.NewRequest(http.MethodPost, "/admin/users", bytes.NewBufferString("{bad json"))
	rr := httptest.NewRecorder()
	status := CreateUserHandler(makeSession(nil, nil), rr, req)

	if status != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", status)
	}
}

// ---------------------------------------------------------------------------
// UpdateUserHandler tests
// ---------------------------------------------------------------------------

func TestUpdateUserHandler_UpdatesPassword(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	body := defs.User{Name: userName1, Password: "newPass"}
	rr := httptest.NewRecorder()
	status := UpdateUserHandler(
		makeSession(map[string]any{"name": userName1}, nil),
		rr,
		newRequest(t, http.MethodPatch, "/admin/users/alice", body),
	)

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", status, rr.Body.String())
	}

	// Confirm the new password is accepted by ValidatePassword.
	if !auth.ValidatePassword(0, userName1, "newPass") {
		t.Error("new password 'newPass' was not accepted after update")
	}
}

func TestUpdateUserHandler_AddPermission(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	// Use the "+perm" prefix to add a custom (non-ego.) permission.
	body := defs.User{Name: userName1, Permissions: []string{"+custom.perm"}}
	rr := httptest.NewRecorder()
	status := UpdateUserHandler(
		makeSession(map[string]any{"name": userName1}, nil),
		rr,
		newRequest(t, http.MethodPatch, "/admin/users/alice", body),
	)

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", status, rr.Body.String())
	}

	u, _ := auth.AuthService.ReadUser(0, userName1, false)

	found := false

	for _, p := range u.Permissions {
		if p == "custom.perm" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected 'custom.perm' in alice's permissions after +custom.perm update; got %v", u.Permissions)
	}
}

func TestUpdateUserHandler_RemovePermission(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	// userName2 starts with "custom.tables" — remove it with the "-" prefix.
	body := defs.User{Name: userName2, Permissions: []string{"-custom.tables"}}
	rr := httptest.NewRecorder()
	status := UpdateUserHandler(
		makeSession(map[string]any{"name": userName2}, nil),
		rr,
		newRequest(t, http.MethodPatch, "/admin/users/bob", body),
	)

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", status, rr.Body.String())
	}

	u, _ := auth.AuthService.ReadUser(0, userName2, false)

	for _, p := range u.Permissions {
		if p == "custom.tables" {
			t.Error("expected 'custom.tables' to be removed from bob's permissions")
		}
	}
}

func TestUpdateUserHandler_CannotRename(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	// Supplying a different name in the body should be rejected with 400.
	body := defs.User{Name: "different"}
	rr := httptest.NewRecorder()
	status := UpdateUserHandler(
		makeSession(map[string]any{"name": userName1}, nil),
		rr,
		newRequest(t, http.MethodPatch, "/admin/users/alice", body),
	)

	if status != http.StatusBadRequest {
		t.Errorf("expected 400 when trying to rename user, got %d", status)
	}
}

func TestUpdateUserHandler_NonexistentUser(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	body := defs.User{Name: "nobody", Password: "x"}
	rr := httptest.NewRecorder()
	status := UpdateUserHandler(
		makeSession(map[string]any{"name": "nobody"}, nil),
		rr,
		newRequest(t, http.MethodPatch, "/admin/users/nobody", body),
	)

	if status != http.StatusNotFound {
		t.Errorf("expected 404, got %d", status)
	}
}

func TestUpdateUserHandler_InvalidPermission(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	body := defs.User{Name: userName1, Permissions: []string{"ego.doesNotExist"}}
	rr := httptest.NewRecorder()
	status := UpdateUserHandler(
		makeSession(map[string]any{"name": userName1}, nil),
		rr,
		newRequest(t, http.MethodPatch, "/admin/users/alice", body),
	)

	if status != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid permission, got %d", status)
	}
}

func TestUpdateUserHandler_NoChangeIsOK(t *testing.T) {
	// An update with no password and no permissions is a no-op, but should
	// still return 200 (the handler skips the write but succeeds).
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	body := defs.User{Name: userName1}
	rr := httptest.NewRecorder()
	status := UpdateUserHandler(
		makeSession(map[string]any{"name": userName1}, nil),
		rr,
		newRequest(t, http.MethodPatch, "/admin/users/alice", body),
	)

	if status != http.StatusOK {
		t.Errorf("expected 200 for no-op update, got %d", status)
	}
}

// ---------------------------------------------------------------------------
// DeleteUserHandler tests
// ---------------------------------------------------------------------------

func TestDeleteUserHandler_DeletesUser(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	status := DeleteUserHandler(
		makeSession(map[string]any{"name": userName1}, nil),
		rr,
		newRequest(t, http.MethodDelete, "/admin/users/alice", nil),
	)

	if status != http.StatusOK {
		t.Errorf("expected 200, got %d — body: %s", status, rr.Body.String())
	}

	// Verify the user is truly gone.
	_, err := auth.AuthService.ReadUser(0, userName1, false)
	if err == nil {
		t.Error("user 'alice' still exists after deletion")
	}
}

func TestDeleteUserHandler_ReturnsDeletedRecord(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	DeleteUserHandler(
		makeSession(map[string]any{"name": userName1}, nil),
		rr,
		newRequest(t, http.MethodDelete, "/admin/users/alice", nil),
	)

	resp := decodeUserResponse(t, rr)
	if resp.Name != userName1 {
		t.Errorf("expected deleted user name 'alice' in response, got %q", resp.Name)
	}
}

func TestDeleteUserHandler_PasswordElided(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	DeleteUserHandler(
		makeSession(map[string]any{"name": userName1}, nil),
		rr,
		newRequest(t, http.MethodDelete, "/admin/users/alice", nil),
	)

	resp := decodeUserResponse(t, rr)
	if resp.Password == egostrings.HashString("secret") {
		t.Error("response must not contain the password hash")
	}
}

func TestDeleteUserHandler_NonexistentUser(t *testing.T) {
	setupTestAuthService(t)
	defer teardownTestAuthService(t, true)

	rr := httptest.NewRecorder()
	status := DeleteUserHandler(
		makeSession(map[string]any{"name": "nobody"}, nil),
		rr,
		newRequest(t, http.MethodDelete, "/admin/users/nobody", nil),
	)

	if status != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent user, got %d", status)
	}
}
