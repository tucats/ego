package services

// Tests for the REST-3 audit's services-engine section (docs/issues/REST-3.md,
// section 6): a missing .ego file is now classified separately from a
// compile error (6.1), and a script calling os.exit() no longer shuts down
// the server (6.4).

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tucats/ego/internal/router"
)

// serviceTestRequest builds a minimal *http.Request and *router.Session
// suitable for calling ServiceHandler directly, bypassing the real router.
// filename is set directly on the session (router.Session.Filename), which
// getCachedService uses in preference to deriving a path from the endpoint,
// so a nonexistent path here deterministically exercises the "file missing"
// case without touching any real service file or global routing state.
func serviceTestRequest(t *testing.T, filename string) (*router.Session, *http.Request, *httptest.ResponseRecorder) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/services/rest3-test", nil)
	req.Header.Set("Accept", "application/json")

	session := &router.Session{
		ID:       1,
		Path:     "services/rest3-test",
		Filename: filename,
		URLParts: map[string]any{},
	}

	return session, req, httptest.NewRecorder()
}

func TestServiceHandler_MissingFile_ReturnsNotFound(t *testing.T) {
	session, req, w := serviceTestRequest(t, "/no/such/path/rest3-missing.ego")

	status := ServiceHandler(session, w, req)

	if status != http.StatusNotFound {
		t.Errorf("status = %d, want 404 -- body: %s", status, w.Body.String())
	}
}

func TestServiceHandler_CompileError_ReturnsInternalServerError(t *testing.T) {
	name := writeServiceFile(t, "func main() {;  this is not valid ego syntax !!!;}\n")

	session, req, w := serviceTestRequest(t, name)

	status := ServiceHandler(session, w, req)

	if status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 -- body: %s", status, w.Body.String())
	}
}

// TestServiceHandler_OSExit_DoesNotCrashProcess guards 6.4: a service script
// that calls os.Exit() must not terminate the real server process. There is
// no direct way to assert a process didn't call os.Exit() other than the
// test process still being alive to report a result at all -- which is
// exactly what this test demonstrates by completing normally. Before the
// fix, this call would have raced a background goroutine that, one second
// later, terminated the entire `go test` process (in the SERVER's mode; in
// child-service mode it was already harmless, since it only killed the
// forked subprocess) -- silently, since go test itself would just vanish.
// It also pins down the response the caller now sees: 500, since a service
// that calls os.exit() did something invalid for a service context, not
// something the (removed) "make 503 true via a real shutdown" logic was
// ever an honest description of.
func TestServiceHandler_OSExit_DoesNotCrashProcess(t *testing.T) {
	name := writeServiceFile(t, "func main() {\n\tos.Exit(0)\n}\n")

	session, req, w := serviceTestRequest(t, name)

	status := ServiceHandler(session, w, req)

	if status != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", status)
	}
}
