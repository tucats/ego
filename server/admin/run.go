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
// The dashboard editor POSTs this struct to ask the server to compile and run
// a snippet of Ego source code on the caller's behalf.
//
// Fields:
//   Code    — the Ego source text to compile and execute.
//   Trace   — when true the server temporarily enables the trace logger so
//             every VM instruction is recorded; useful for debugging.
//   Console — when true the server reuses a persistent symbol table across
//             successive calls (REPL / console mode).  When false (editor
//             mode) each run gets a fresh, empty symbol table.
//   Session — a browser-generated UUID that identifies which persistent
//             symbol table belongs to this caller.  Each open dashboard tab
//             gets its own UUID so concurrent users cannot see each other's
//             variables.
type codeRunRequest struct {
	Code    string `json:"code"`
	Trace   bool   `json:"trace,omitempty"`
	Console bool   `json:"console,omitempty"` // true → reuse the persistent symbol table (REPL mode)
	Session string `json:"session,omitempty"` // browser-generated UUID identifying the caller's symbol table
}

// codeRunResponse is the JSON body returned by POST /admin/run.
// Output carries everything the program wrote to stdout; Error is non-empty
// when compilation or execution failed.
type codeRunResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// symbolEntry holds a per-session persistent symbol table together with the
// time it was last used, so the reaper goroutine can evict idle entries.
//
// A symbol table in Ego is the runtime environment that stores all declared
// variables and imported packages for a running program.  In console (REPL)
// mode we keep one alive between requests so variables declared in one command
// are still accessible in the next.
type symbolEntry struct {
	table    *symbols.SymbolTable
	lastUsed time.Time
}

// symbolMap stores one symbolEntry per browser-session UUID.  Using a map
// keyed by UUID means each dashboard tab has its own isolated state.
//
// symbolMapLock is a mutual-exclusion lock (mutex).  In Go, maps are not safe
// for concurrent access, so every read or write to symbolMap must be done
// while holding this lock.  sync.Mutex has zero value that is ready to use —
// no initialisation needed.
var (
	symbolMap         = map[string]*symbolEntry{}
	symbolInitialized bool
	symbolMapLock     sync.Mutex
)

// runLock serializes code execution so stdout capture does not race when
// multiple dashboard users run code at the same time.
//
// os.Stdout is a process-wide global, so if two goroutines both redirect it
// simultaneously they would corrupt each other's output.  Holding runLock
// while executing ensures only one program runs at a time.
var runLock sync.Mutex

// initializeSymbolCleanup starts a background goroutine that removes symbol
// table entries that have not been used for more than one hour.  It runs on a
// five-minute ticker so the worst-case extra lifetime of an idle entry is
// 1 h 5 m.
//
// The function is guarded by symbolInitialized so it only ever starts one
// reaper goroutine, even if called from multiple goroutines concurrently.
func initializeSymbolCleanup() {
	if symbolInitialized {
		return
	}

	symbolInitialized = true

	// The "go func() { ... }()" idiom launches an anonymous function as a new
	// goroutine — a lightweight thread managed by the Go runtime.  The reaper
	// runs independently of the HTTP request lifecycle and never needs to be
	// stopped because the server runs until the process exits.
	go func() {
		// time.NewTicker returns a ticker that fires on its C channel every
		// five minutes.  defer ticker.Stop() releases the ticker's resources
		// when the goroutine eventually exits (in practice: never, but it is
		// good practice to always stop tickers).
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		// "for range ticker.C" blocks until the ticker fires, then runs the
		// body, then blocks again — repeating indefinitely.
		for range ticker.C {
			// Calculate the cutoff time: entries last used before this moment
			// are considered idle and will be removed.
			cutoff := time.Now().Add(-time.Hour)

			symbolMapLock.Lock()

			for id, entry := range symbolMap {
				if entry.lastUsed.Before(cutoff) {
					// delete removes the key from the map entirely.
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

// RunCodeHandler is the HTTP handler for POST /admin/run.
// It accepts Ego source code from the dashboard editor, compiles and runs it,
// then returns the captured stdout output (and any error) as a JSON object
// shaped like {"output":"...","error":"..."}.
func RunCodeHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var req codeRunRequest

	// json.NewDecoder wraps the request body in a streaming JSON decoder.
	// Decode fills req with the fields from the JSON body; if the body is
	// malformed it returns a non-nil error and we respond with 400 Bad Request.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	if ui.IsActive(ui.RestLogger) {
		b, _ := json.MarshalIndent(req, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	// Temporarily switch the trace logger to the state requested by the caller.
	// We save the current state first so we can restore it after the run,
	// leaving the server's logging configuration unchanged for other requests.
	savedTrace := ui.IsActive(ui.TraceLogger)
	ui.Active(ui.TraceLogger, req.Trace)

	output, runErr := executeAdminEgo(req.Code, req.Console, req.Session)

	// Restore the original trace state regardless of whether the run succeeded.
	ui.Active(ui.TraceLogger, savedTrace)

	// Build the response struct.  If the run produced an error, include its
	// text in the Error field so the dashboard can display it distinctly from
	// normal output.
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
//
// Capturing stdout requires temporarily replacing the os.Stdout file
// descriptor with a pipe.  The write end of the pipe is assigned to os.Stdout
// so the Ego runtime's print/fmt functions write into it; after the program
// finishes we close the write end and drain the read end into a buffer.
func executeAdminEgo(source string, console bool, uuid string) (string, error) {
	// runLock ensures only one program executes at a time so two concurrent
	// requests cannot interfere with the stdout pipe swap.
	runLock.Lock()
	defer runLock.Unlock()

	// Save the real stdout so we can restore it after the run.
	origStdout := os.Stdout

	// os.Pipe() creates a connected read/write pair of *os.File values.
	// Writes to w appear as reads from r.
	r, w, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("internal: could not create pipe: %w", err)
	}

	// Redirect all subsequent writes to os.Stdout into the pipe.
	os.Stdout = w

	runErr := runAdminEgo(source, console, uuid)

	// Close the write end of the pipe BEFORE reading from the read end.
	// If we read first, io.Copy would block forever waiting for more data
	// because the write end is still open.  Closing it sends EOF to the reader.
	w.Close()

	// Restore real stdout before reading the pipe so any server-side output
	// that happens after this point goes to the terminal, not into the buffer.
	os.Stdout = origStdout

	// Drain the captured output into buf.
	// The blank identifier _ discards the byte count; we only care about the string.
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	r.Close()

	return buf.String(), runErr
}

// runAdminEgo compiles and runs source using either the persistent symbol table
// for the given UUID (console == true, REPL mode) or a fresh child table
// (console == false, editor mode).
//
// Symbol table lifecycle:
//   • First call for a UUID: a root table (with the standard library imported)
//     and a "console" child table are created and stored in symbolMap.
//   • Subsequent calls for the same UUID: the existing entry is reused and its
//     lastUsed timestamp is refreshed so the reaper does not evict it.
//   • Editor (non-console) calls: a temporary child of the persistent table is
//     created for this run only and discarded afterwards, so each editor run
//     starts with a clean slate while still having access to the standard library.
func runAdminEgo(source string, console bool, uuid string) error {
	symbolMapLock.Lock()

	// Start the reaper goroutine on the very first execution if it has not
	// been started yet.  This is done inside the lock to avoid a race between
	// two goroutines both seeing symbolInitialized == false simultaneously.
	if !symbolInitialized {
		initializeSymbolCleanup()
	}

	entry, ok := symbolMap[uuid]
	if !ok {
		// First use for this UUID — build the full symbol table hierarchy.
		//
		// NewRootSymbolTable creates the top-level table that owns all
		// predeclared identifiers; NewChildSymbolTable creates a child that
		// inherits from the root but can also hold its own declarations.
		//
		// compiler.AddStandard populates root with the built-in functions and
		// types that every Ego program can use (fmt, strings, math, etc.).
		//
		// compiler.New builds a compiler instance; AutoImport scans the
		// standard-library packages and imports them into the table.
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
		// Existing entry — just update the last-used timestamp so the reaper
		// knows this session is still active.
		entry.lastUsed = time.Now()
	}

	// Take a local reference to the table before releasing the lock.
	// If we held the lock during compilation and execution (which can take
	// milliseconds to seconds) we would block every other goroutine from
	// looking up or creating symbol tables for the entire run.
	s := entry.table

	symbolMapLock.Unlock()

	// Editor runs use a fresh child table so each run starts clean.
	// Console runs reuse the persistent table so declared variables and
	// imported packages accumulate across successive commands.
	if !console {
		s = symbols.NewChildSymbolTable("editor", s)
	}

	// compiler.RunString compiles source in the context of symbol table s and
	// immediately executes it.  Any runtime or compile-time error is returned.
	return compiler.RunString("dashboard", s, source)
}
