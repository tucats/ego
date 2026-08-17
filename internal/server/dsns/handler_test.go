package dsns

// Tests for the status-code fixes made to this package's handlers as part of
// the REST-3 audit (docs/issues/REST-3.md, section 3):
//
//   - CreateDSNHandler must report 409, not 400, for a duplicate name.
//   - DSNPermissionsHandler must not fall through to a misleading 200 after
//     already sending a 400 for a malformed body.
//   - ListDSNPermHandler must still report 404 for a missing DSN now that it
//     goes through the shared dberrors classifier instead of a hardcoded
//     status.
//
// Each test installs a temporary in-memory DSN service (mirroring the
// technique used in server/admin/users/users_test.go), uses
// net/http/httptest to capture responses, and asserts on status codes and
// decoded JSON bodies.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tucats/ego/internal/defs"
	egodsns "github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/router"
)

// setupTestDSNService replaces the global egodsns.DSNService with a fresh
// in-memory file-backed service.
func setupTestDSNService(t *testing.T) {
	t.Helper()

	svc, err := egodsns.NewFileService("memory")
	if err != nil {
		t.Fatalf("create test DSN service: %v", err)
	}

	egodsns.DSNService = svc
}

func makeDSNSession(urlParts map[string]any) *router.Session {
	return &router.Session{
		ID:       1,
		User:     "admin",
		// Admin must be set explicitly -- it's a separate field from User,
		// not implied by the username. These tests call handlers directly,
		// bypassing the router's own authentication/authorization layer
		// entirely, so DATA-SECURITY.md §3.6's in-handler admin check
		// (added to DeleteDSNHandler/DSNPermissionsHandler/
		// ListDSNPermHandler) is the first thing here that actually reads
		// this field; every test in this package is about handler
		// behavior, not authorization, so an admin session is the right
		// default throughout.
		Admin:    true,
		URLParts: urlParts,
	}
}

func newDSNRequest(t *testing.T, method string, body any) *http.Request {
	t.Helper()

	var bodyBytes []byte

	if body != nil {
		var err error

		bodyBytes, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}

	req, err := http.NewRequest(method, "/dsns", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	return req
}

func TestCreateDSNHandler_DuplicateName_ReturnsConflict(t *testing.T) {
	setupTestDSNService(t)

	dsn := defs.DSN{Name: "d1", Provider: defs.SqliteProvider, Database: "d1.db"}

	rr := httptest.NewRecorder()
	status := CreateDSNHandler(makeDSNSession(nil), rr, newDSNRequest(t, http.MethodPost, dsn))

	if status != http.StatusCreated {
		t.Fatalf("initial create: expected 201, got %d -- body: %s", status, rr.Body.String())
	}

	if got, want := rr.Header().Get(defs.LocationHeader), "/dsns/d1"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}

	// Same name again -- must be rejected as a conflict, not silently
	// applied as an overwrite.
	rr2 := httptest.NewRecorder()
	status2 := CreateDSNHandler(makeDSNSession(nil), rr2, newDSNRequest(t, http.MethodPost, dsn))

	if status2 != http.StatusConflict {
		t.Errorf("duplicate create: expected 409, got %d -- body: %s", status2, rr2.Body.String())
	}
}

// TestDSNPermissionsHandler_MalformedBody_SingleResponse guards against a
// missing "return" that let execution fall through from the malformed-body
// 400 path to the success path below, writing a second, misleading 200
// response (with Count: 0) on top of the 400 already sent.
func TestDSNPermissionsHandler_MalformedBody_SingleResponse(t *testing.T) {
	setupTestDSNService(t)

	req, err := http.NewRequest(http.MethodPost, "/dsns/perm", bytes.NewBufferString("not json at all"))
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	rr := httptest.NewRecorder()
	status := DSNPermissionsHandler(makeDSNSession(nil), rr, req)

	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d -- body: %s", status, rr.Body.String())
	}

	var body defs.RestStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body did not parse as a single JSON document (want exactly one response, not a 400 followed by a second 200): %v\nbody: %s", err, rr.Body.String())
	}

	if body.Status != http.StatusBadRequest {
		t.Errorf("body.Status = %d, want %d", body.Status, http.StatusBadRequest)
	}
}

func TestListDSNPermHandler_MissingDSN_ReturnsNotFound(t *testing.T) {
	setupTestDSNService(t)

	rr := httptest.NewRecorder()
	status := ListDSNPermHandler(
		makeDSNSession(map[string]any{"dsn": "nosuchdsn"}),
		rr,
		newDSNRequest(t, http.MethodGet, nil),
	)

	if status != http.StatusNotFound {
		t.Errorf("expected 404, got %d -- body: %s", status, rr.Body.String())
	}
}
