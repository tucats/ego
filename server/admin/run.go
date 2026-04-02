package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/symbols"
)

// codeRunRequest is the JSON body expected by POST /admin/run.
type codeRunRequest struct {
	Code    string `json:"code"`
	Trace   bool   `json:"trace,omitempty"`
	Console bool   `json:"console,omitempty"` // true → reuse the persistent symbol table (REPL mode)
}

// codeRunResponse is the JSON body returned by POST /admin/run.
type codeRunResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// runMu serializes code execution so stdout capture doesn't race when multiple
// dashboard users run code concurrently.
var runMu sync.Mutex

// adminSymbols is the persistent symbol table shared across console runs in the
// dashboard Code tab. It is initialized on first use and retained for the
// lifetime of the server process, giving the console REPL-like behavior.
var adminSymbols *symbols.SymbolTable

// RunCodeHandler handles POST /admin/run.
// It accepts Ego source code, compiles and runs it, then returns the captured
// output (or any error) as JSON with the shape {"output":"...","error":"..."}.
func RunCodeHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var req codeRunRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)

		return http.StatusBadRequest
	}

	// Temporarily enable/disable the trace logger as requested, then restore it.
	savedTrace := ui.IsActive(ui.TraceLogger)
	ui.Active(ui.TraceLogger, req.Trace)

	output, runErr := executeAdminEgo(req.Code, req.Console)

	ui.Active(ui.TraceLogger, savedTrace)

	resp := codeRunResponse{Output: output}
	if runErr != nil {
		resp.Error = runErr.Error()
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return http.StatusInternalServerError
	}

	return http.StatusOK
}

// executeAdminEgo compiles and runs the given Ego source code, capturing
// everything written to os.Stdout and returning it as a string.
func executeAdminEgo(source string, console bool) (string, error) {
	runMu.Lock()
	defer runMu.Unlock()

	// Redirect os.Stdout so we capture everything the Ego fmt package writes.
	origStdout := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("internal: could not create pipe: %w", err)
	}

	os.Stdout = w

	// Run the code.
	runErr := runAdminEgo(source, console)

	// Restore stdout before reading the pipe (avoid deadlock on large output).
	w.Close()

	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	r.Close()

	return buf.String(), runErr
}

// runAdminEgo compiles and runs source using either the persistent dashboard
// symbol table (console == true, REPL mode) or a fresh child table
// (console == false, editor mode).
func runAdminEgo(source string, console bool) error {
	// Ensure the persistent symbol table exists.
	if adminSymbols == nil {
		root := symbols.NewRootSymbolTable("dashboard")
		adminSymbols = symbols.NewChildSymbolTable("console", root)

		compiler.AddStandard(root)

		comp := compiler.New("dashboard").
			SetExtensionsEnabled(true).
			SetRoot(adminSymbols)

		if err := comp.AutoImport(true, adminSymbols); err != nil {
			// Discard the partial table so the next call retries cleanly.
			adminSymbols = nil

			return err
		}
	}

	// Editor runs get a fresh child table so each run starts clean.
	// Console runs reuse the persistent table so state accumulates.
	s := adminSymbols
	if !console {
		s = symbols.NewChildSymbolTable("editor", adminSymbols)
	}

	return compiler.RunString("dashboard", s, source)
}
