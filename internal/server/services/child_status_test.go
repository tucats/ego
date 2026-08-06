package services

// Tests for the child-service-mode counterparts of the REST-3 audit fixes
// (docs/issues/REST-3.md, section 6): 6.2 (a computed status that never
// reached the caller because it was never written to the response file)
// and 6.4 (os.exit() in a service script no longer terminates a process --
// here, the forked child process was always the only thing it could kill,
// but the dead code implying otherwise is gone too).

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ego/internal/cli/ui"
)

// runChildService writes a minimal ChildServiceRequest naming filename as
// the service to run, invokes ChildService against it, and returns the
// decoded ChildServiceResponse it wrote back.
func runChildService(t *testing.T, filename string) ChildServiceResponse {
	t.Helper()

	dir := t.TempDir()

	saved := ChildTempDir
	ChildTempDir = dir
	t.Cleanup(func() { ChildTempDir = saved })

	const serverID = "rest3-test-server"

	sessionID := 1

	req := ChildServiceRequest{
		SessionID: sessionID,
		ServerID:  serverID,
		Method:    http.MethodGet,
		Path:      "/services/rest3-test",
		Filename:  filename,
	}

	b, err := json.MarshalIndent(req, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	requestFileName := filepath.Join(dir, "ego-request-"+serverID+"-1.json")
	if err := os.WriteFile(requestFileName, b, 0o644); err != nil {
		t.Fatalf("write request file: %v", err)
	}

	// ChildService's own error return only covers "could not produce a
	// response file at all" (a marshal or os.Create failure) -- a service
	// failure proper is reported through the response file's Status field,
	// not through this return value, so it is deliberately not asserted on
	// zero here.
	_ = ChildService(requestFileName)

	responseFileName := filepath.Join(dir, "ego-response-"+serverID+"-1.json")

	respBytes, err := os.ReadFile(responseFileName)
	if err != nil {
		t.Fatalf("ChildService did not write a response file: %v", err)
	}

	var resp ChildServiceResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("response file did not contain valid JSON: %v\nbody: %s", err, string(respBytes))
	}

	return resp
}

func TestChildService_MissingFile_ReturnsNotFound(t *testing.T) {
	resp := runChildService(t, "/no/such/path/rest3-missing.ego")

	if resp.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want 404 -- Message: %q", resp.Status, resp.Message)
	}
}

func TestChildService_CompileError_ReturnsInternalServerError(t *testing.T) {
	name := writeServiceFile(t, "func main() {;  this is not valid ego syntax !!!;}\n")

	resp := runChildService(t, name)

	if resp.Status != http.StatusInternalServerError {
		t.Errorf("Status = %d, want 500 -- Message: %q", resp.Status, resp.Message)
	}
}

// TestChildService_OSExit_ReportsInternalServerError guards 6.4 for the
// child-mode path: a script calling os.Exit() must not prevent the child
// process's response from reaching the parent (the pre-fix childError()
// wrote to stdout, which the parent only reads as log output, so the
// response file was never written and the parent hardcoded 500 for an
// unrelated reason -- an *exec.ExitError from the child's nonzero exit,
// not this status). Here that means the response file must exist and carry
// the status this function actually chose.
func TestChildService_OSExit_ReportsInternalServerError(t *testing.T) {
	name := writeServiceFile(t, "func main() {\n\tos.Exit(0)\n}\n")

	resp := runChildService(t, name)

	if resp.Status != http.StatusInternalServerError {
		t.Errorf("Status = %d, want 500 -- Message: %q", resp.Status, resp.Message)
	}
}
