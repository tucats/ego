package webapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/symbols"
)

// handleRun receives Ego source code, compiles and runs it, then returns
// the captured output (or any error) as JSON.
func handleRun(w http.ResponseWriter, r *http.Request) {
	var req runRequest

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)

		return
	}

	// Save and apply the trace logger state, then restore it after execution.
	savedTrace := ui.IsActive(ui.TraceLogger)
	ui.Active(ui.TraceLogger, req.Trace)

	output, runErr := executeEgo(req.Code)

	ui.Active(ui.TraceLogger, savedTrace)

	resp := runResponse{Output: output}
	if runErr != nil {
		resp.Error = runErr.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// executeEgo compiles and runs the given Ego source code, returning whatever
// was written to stdout and any execution error.
func executeEgo(source string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	// Redirect os.Stdout so we capture everything the Ego fmt package writes.
	origStdout := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("internal: could not create pipe: %w", err)
	}

	os.Stdout = w

	// Run the code.
	runErr := runEgo(source)

	// Restore stdout before reading the pipe (avoid deadlock on large output).
	w.Close()

	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	r.Close()

	return buf.String(), runErr
}

// runEgo sets up an Ego execution environment and runs the given source string.
func runEgo(source string) error {
	s := symbols.NewRootSymbolTable("playground")

	compiler.AddStandard(s)

	comp := compiler.New("playground").
		SetExtensionsEnabled(true).
		SetRoot(s)

	if err := comp.AutoImport(true, s); err != nil {
		return err
	}

	return compiler.RunString("playground", s, source)
}
