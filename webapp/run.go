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

	output, runErr := executeEgo(req.Code, req.Console)

	ui.Active(ui.TraceLogger, savedTrace)

	resp := runResponse{Output: output}
	if runErr != nil {
		resp.Error = runErr.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// executeEgo compiles and runs the given Ego source code, returning whatever
// was written to stdout and any execution error. When console is true the
// persistent symbol table is reused (REPL mode); when false a fresh table
// is used for each run (editor mode).
func executeEgo(source string, console bool) (string, error) {
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
	runErr := runEgo(source, console)

	// Restore stdout before reading the pipe (avoid deadlock on large output).
	w.Close()

	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	r.Close()

	return buf.String(), runErr
}

// playgroundSymbols is the persistent symbol table shared across all runs in
// this webapp session. It is created and seeded with the standard library on
// the first call to runEgo, then reused so that variables, functions, and
// imported packages defined in one run are available in subsequent runs.
var playgroundSymbols *symbols.SymbolTable

// runEgo compiles and runs source in either the persistent symbol table
// (console == true, REPL mode) or a fresh one (console == false, editor mode).
// The persistent table is initialized on first use and retained for the
// lifetime of the server process.
func runEgo(source string, console bool) error {
	// Ensure the persistent symbol table exists.
	if playgroundSymbols == nil {
		playgroundRootSymbols := symbols.NewRootSymbolTable("playground")
		playgroundSymbols = symbols.NewChildSymbolTable("console", playgroundRootSymbols)

		compiler.AddStandard(playgroundRootSymbols)

		comp := compiler.New("playground").
			SetExtensionsEnabled(true).
			SetRoot(playgroundSymbols)

		if err := comp.AutoImport(true, playgroundSymbols); err != nil {
			// If initialization fails, discard the partial table so the
			// next call retries rather than operating on a broken state.
			playgroundSymbols = nil

			return err
		}
	}

	// Editor runs get a fresh child table so each run starts clean.
	// Console runs reuse the persistent table so state accumulates.
	s := playgroundSymbols
	if !console {
		s = symbols.NewChildSymbolTable("editor", playgroundSymbols)
	}

	return compiler.RunString("playground", s, source)
}
