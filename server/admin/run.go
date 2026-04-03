package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// codeRunRequest is the JSON body expected by POST /admin/run.
type codeRunRequest struct {
	Code    string `json:"code"`
	Trace   bool   `json:"trace,omitempty"`
	Console bool   `json:"console,omitempty"` // true → reuse the persistent symbol table (REPL mode)
	Session string `json:"session,omitempty"` // browser-generated UUID identifying the caller's symbol table
}

// codeRunResponse is the JSON body returned by POST /admin/run.
type codeRunResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// symbolEntry holds a per-session persistent symbol table together with the
// time it was last used, so the reaper goroutine can evict idle entries.
type symbolEntry struct {
	table    *symbols.SymbolTable
	lastUsed time.Time
}

// symbolMap stores one symbolEntry per browser session UUID.  All accesses
// are serialized through symbolMapLock.
var (
	symbolMap         = map[string]*symbolEntry{}
	symbolInitialized bool
	symbolMapLock     sync.Mutex
)

// runLock serializes code execution so stdout capture doesn't race when multiple
// dashboard users run code concurrently.
var runLock sync.Mutex

// initializeSymbolCleanup starts a background goroutine that removes symbol table entries that
// have not been used for more than one hour.  It runs every five minutes so
// the worst-case extra lifetime of an idle entry is 1 h 5 m.
func initializeSymbolCleanup() {
	if symbolInitialized {
		return
	}

	symbolInitialized = true

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			cutoff := time.Now().Add(-time.Hour)

			symbolMapLock.Lock()

			for id, entry := range symbolMap {
				if entry.lastUsed.Before(cutoff) {
					delete(symbolMap, id)
					ui.Log(ui.ServerLogger, "admin.run.session.reaped", ui.A{
						"session": id,
					})
				}
			}

			symbolMapLock.Unlock()
		}
	}()
}

// RunCodeHandler handles POST /admin/run.
// It accepts Ego source code, compiles and runs it, then returns the captured
// output (or any error) as JSON with the shape {"output":"...","error":"..."}.
func RunCodeHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var req codeRunRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	if ui.IsActive(ui.RestLogger) {
		b, _ := json.MarshalIndent(req, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	// Temporarily enable/disable the trace logger as requested, then restore it.
	savedTrace := ui.IsActive(ui.TraceLogger)
	ui.Active(ui.TraceLogger, req.Trace)

	output, runErr := executeAdminEgo(req.Code, req.Console, req.Session)

	ui.Active(ui.TraceLogger, savedTrace)

	resp := codeRunResponse{Output: output}
	if runErr != nil {
		resp.Error = runErr.Error()
	}

	w.Header().Set("Content-Type", "application/json")

	b := util.WriteJSON(w, resp, &session.ResponseLength)
	ui.Log(ui.RestLogger, "rest.response.payload", ui.A{
		"session": session.ID,
		"body":    string(b)})

	return http.StatusOK
}

// executeAdminEgo compiles and runs the given Ego source code, capturing
// everything written to os.Stdout and returning it as a string.
func executeAdminEgo(source string, console bool, uuid string) (string, error) {
	runLock.Lock()
	defer runLock.Unlock()

	// Redirect os.Stdout so we capture everything the Ego fmt package writes.
	origStdout := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("internal: could not create pipe: %w", err)
	}

	os.Stdout = w

	// Run the code.
	runErr := runAdminEgo(source, console, uuid)

	// Restore stdout before reading the pipe (avoid deadlock on large output).
	w.Close()

	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	r.Close()

	return buf.String(), runErr
}

// runAdminEgo compiles and runs source using either the persistent symbol table
// for the given UUID (console == true, REPL mode) or a fresh child table
// (console == false, editor mode).
//
// Each browser session identified by uuid gets its own isolated symbol table so
// concurrent dashboard users cannot see each other's state.  If uuid is empty
// (e.g. a legacy client that does not send the field) the call falls back to a
// shared entry keyed by the empty string, which preserves the old behavior.
func runAdminEgo(source string, console bool, uuid string) error {
	symbolMapLock.Lock()

	// If this is the first time we do this, spin off a thread that will handle
	// cleanup of the persistent symbol table entries.
	if !symbolInitialized {
		initializeSymbolCleanup()
	}

	entry, ok := symbolMap[uuid]
	if !ok {
		// First use for this UUID — initialize a root table and a persistent
		// child ("console") that accumulates REPL state across calls.
		root := symbols.NewRootSymbolTable("dashboard")
		consoleTable := symbols.NewChildSymbolTable("console", root)

		compiler.AddStandard(root)

		comp := compiler.New("dashboard").
			SetExtensionsEnabled(true).
			SetRoot(consoleTable)

		if err := comp.AutoImport(true, consoleTable); err != nil {
			symbolMapLock.Unlock()

			return err
		}

		entry = &symbolEntry{table: consoleTable, lastUsed: time.Now()}
		symbolMap[uuid] = entry

		ui.Log(ui.ServerLogger, "admin.run.session.created", ui.A{
			"session": uuid,
		})
	} else {
		entry.lastUsed = time.Now()
	}

	// Take a local reference before releasing the lock so the reaper cannot
	// delete the entry while we are executing code with its table.
	s := entry.table

	symbolMapLock.Unlock()

	// Editor runs get a fresh child table so each run starts clean.
	// Console runs reuse the persistent table so state accumulates.
	if !console {
		s = symbols.NewChildSymbolTable("editor", s)
	}

	return compiler.RunString("dashboard", s, source)
}
