package admin

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/debugger"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

const maxDebugSessions = 20
const maxCodeSessions = 20

// codeRunRequest is the JSON body expected by POST /admin/run.
//
// Fields:
//
//	Code       — the Ego source text to compile and execute.
//	Trace      — when true the server temporarily enables the trace logger.
//	Debug      — when true the server runs the code under the interactive
//	             debugger instead of executing it directly. The response will
//	             contain DebugOutput, DebugPrompt, and DebugWaiting instead of
//	             the normal Output field.
//	DebugInput — a command string to deliver to an already-running debug
//	             session. Only used when Debug is true and a session already
//	             exists for the given Session UUID.
//	Console    — when true the server reuses a persistent symbol table across
//	             successive calls (REPL mode).
//	Session    — a browser-generated UUID identifying the caller's symbol table
//	             and, in debug mode, the active debugger session.
type codeRunRequest struct {
	Code       string `json:"code"`
	Trace      bool   `json:"trace,omitempty"`
	Debug      bool   `json:"debug,omitempty"`
	DebugInput string `json:"debugInput,omitempty"`
	Console    bool   `json:"console,omitempty"`
	Session    string `json:"session,omitempty"`
}

// codeRunResponse is the JSON body returned by POST /admin/run.
//
// Normal (non-debug) mode:
//
//	Output — everything the program wrote to stdout.
//	Error  — non-empty when compilation or execution failed.
//
// Debug mode:
//
//	DebugOutput   — text produced by the debugger itself since the previous
//	                call: step notifications, breakpoint messages, show-command
//	                results, etc.  Display in a dedicated "Debugger" panel.
//	ProgramOutput — text the running Ego program wrote to stdout (fmt.Println,
//	                etc.) since the previous call.  Display in the "Output" pane.
//	DebugPrompt   — the prompt string the debugger is currently showing; the
//	                caller should display this and collect the next command.
//	DebugWaiting  — true when the debugger is paused waiting for the next
//	                command.  False when the session has ended.
//	Error         — non-empty if the debug session ended with an error.
//
// Line           - last line of code successfully executed. This will only be
//                  valid when debug mode is active.
type codeRunResponse struct {
	Output        string `json:"output,omitempty"`
	Error         string `json:"error,omitempty"`
	DebugOutput   string `json:"debugOutput,omitempty"`
	ProgramOutput string `json:"programOutput,omitempty"`
	DebugPrompt   string `json:"debugPrompt,omitempty"`
	DebugWaiting  bool   `json:"debugWaiting,omitempty"`
	Line          int    `json:"line"`
}

// codeSessionEntry holds a per-session persistent symbol table together with the
// time it was last used, so the reaper goroutine can evict idle entries.
type codeSessionEntry struct {
	table    *symbols.SymbolTable
	lastUsed time.Time
}

// debugSession holds the live bytecode context for an active debug session
// along with the time it was last used.
type debugSession struct {
	ctx      *bytecode.Context
	lastUsed time.Time
}

// codeSessions stores one symbolEntry per browser-session UUID. codeSessionLock
// serializes all reads and writes to the map.
var (
	codeSessions            = map[string]*codeSessionEntry{}
	codeSessionsInitialized bool
	codeSessionLock         sync.Mutex
)

// debugSessions stores one debugEntry per browser-session UUID while a debug
// session is in progress. debugSessionLock serializes all reads and writes.
var (
	debugSessions    = map[string]*debugSession{}
	debugSessionLock sync.Mutex
)

// initializeSessionCleanup starts a background goroutine that removes code
// session entries that have not been used for more than one hour. It runs on a
// five-minute ticker. It also reaps idle debug sessions (15-minute timeout) to
// avoid leaking goroutines when a browser tab is closed mid-debug.
func initializeSessionCleanup() {
	if codeSessionsInitialized {
		return
	}

	codeSessionsInitialized = true

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			// Reap idle symbol tables.
			cutoff := time.Now().Add(-time.Hour)

			codeSessionLock.Lock()
			for id, entry := range codeSessions {
				if entry.lastUsed.Before(cutoff) {
					delete(codeSessions, id)
					ui.Log(ui.ServerLogger, "admin.run.session.reaped", ui.A{"session": id})
				}
			}
			codeSessionLock.Unlock()

			// Reap idle debug sessions.
			debugCutoff := time.Now().Add(-15 * time.Minute)

			debugSessionLock.Lock()
			for id, entry := range debugSessions {
				if entry.lastUsed.Before(debugCutoff) {
					debugger.Close(entry.ctx)
					delete(debugSessions, id)
					ui.Log(ui.ServerLogger, "admin.run.debug.session.reaped", ui.A{"id": id})
				}
			}
			debugSessionLock.Unlock()
		}
	}()
}

// RunCodeHandler is the HTTP handler for POST /admin/run.
func RunCodeHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var req codeRunRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	if ui.IsActive(ui.RestLogger) {
		b, _ := json.MarshalIndent(req, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    string(b),
		})
	}

	var resp codeRunResponse

	if req.Debug {
		// A bit of a hack, but you can turn on the trace flag which will force tracing
		// of the output. Note this turns on ALL tracing, so concurrent program runs
		// will commingle their trace output.
		savedTrace := ui.IsActive(ui.TraceLogger)
		ui.Active(ui.TraceLogger, req.Trace)

		resp = executeAdminDebug(req.Code, req.DebugInput, req.Session)

		ui.Active(ui.TraceLogger, savedTrace)
	} else {
		savedTrace := ui.IsActive(ui.TraceLogger)
		ui.Active(ui.TraceLogger, req.Trace)

		output, runErr := executeAdminEgo(req.Code, req.Console, req.Session)

		ui.Active(ui.TraceLogger, savedTrace)

		resp = codeRunResponse{Output: output}
		if runErr != nil {
			resp.Error = runErr.Error()
		}
	}

	w.Header().Set("Content-Type", "application/json")

	b := util.WriteJSON(w, resp, &session.ResponseLength)
	ui.Log(ui.RestLogger, "rest.response.payload", ui.A{
		"session": session.ID,
		"body":    string(b),
	})

	return http.StatusOK
}

// executeAdminDebug manages one round-trip with the debugger for the given
// session UUID.
//
//   - If no debug session exists for uuid and code is non-empty, it compiles
//     the code and starts a new debug session.
//   - If a session already exists and debugInput is non-empty, it delivers
//     the command to the running debugger.
//   - If a session exists but debugInput is empty, it returns the current
//     wait state without sending any input (re-poll / page refresh).
func executeAdminDebug(code, debugInput, uuid string) codeRunResponse {
	var debugCount int

	debugSessionLock.Lock()

	debugCount = len(debugSessions)
	entry, exists := debugSessions[uuid]

	debugSessionLock.Unlock()

	var ctx *bytecode.Context

	if exists {
		debugSessionLock.Lock()
		entry.lastUsed = time.Now()
		debugSessionLock.Unlock()

		ctx = entry.ctx

		if debugInput == "" {
			// Nothing to deliver; return waiting state so the caller can re-show the prompt.
			return codeRunResponse{
				DebugWaiting: true,
				DebugPrompt:  "debug> ",
			}
		}
	} else {
		// Do we already have too many sessions?
		if debugCount >= maxDebugSessions {
			return codeRunResponse{Error: errors.ErrMaxDebugSessions.Context(debugCount).Error()}
		}

		// No existing session — compile code and create a new debug context.
		if code == "" {
			return codeRunResponse{Error: "no active debug session and no code provided"}
		}

		s, err := getOrCreateSymbolTable(uuid)
		if err != nil {
			return codeRunResponse{Error: err.Error()}
		}

		// Editor debug runs use a fresh child table so each run starts clean.
		s = symbols.NewChildSymbolTable("debug-editor", s)

		bc, compileErr := compiler.CompileString("dashboard", code)
		if compileErr != nil {
			return codeRunResponse{Error: compileErr.Error()}
		}

		ctx = bytecode.NewContext(s, bc).
			SetDebug(true).
			EnableConsoleOutput(false) // capture program output into the session channelWriter

		debugSessionLock.Lock()
		debugSessions[uuid] = &debugSession{ctx: ctx, lastUsed: time.Now()}
		debugSessionLock.Unlock()
	}

	// First call passes "" to start the goroutine; subsequent calls pass the command.
	dbResp := debugger.Resume(ctx, debugInput)

	if dbResp.Done {
		debugSessionLock.Lock()
		delete(debugSessions, uuid)
		debugSessionLock.Unlock()

		resp := codeRunResponse{
			DebugOutput:   dbResp.Output,
			ProgramOutput: dbResp.ProgramOutput,
		}
		if dbResp.Err != nil && !errors.Equals(dbResp.Err, errors.ErrStop) {
			resp.Error = dbResp.Err.Error()
		}

		return resp
	}

	return codeRunResponse{
		DebugOutput:   dbResp.Output,
		ProgramOutput: dbResp.ProgramOutput,
		DebugPrompt:   dbResp.Prompt,
		DebugWaiting:  true,
		Line:          dbResp.Line,
	}
}

// getOrCreateSymbolTable returns the persistent console symbol table for the
// given UUID, creating it (and starting the reaper) on first use.
func getOrCreateSymbolTable(uuid string) (*symbols.SymbolTable, error) {
	var sessionCount int

	// This is going to make multiple references into the symbolMap, so lock it
	// while we're here. This runs fairly briefly.
	codeSessionLock.Lock()
	defer codeSessionLock.Unlock()

	// If we are here the first time, fire off a go routine that handles cleanup
	// of expired symbol table sessions.
	if !codeSessionsInitialized {
		initializeSessionCleanup()
	}

	sessionCount = len(codeSessions)

	entry, ok := codeSessions[uuid]
	if !ok {
		// Have we exceeded the maximum number of Code sessions?
		if sessionCount >= maxCodeSessions {
			return nil, errors.ErrMaxCodeSessions.Context(sessionCount)
		}

		// No existing session — create a new console symbol table.
		ui.Log(ui.ServerLogger, "admin.run.session.created", ui.A{"session": uuid})

		entry = &codeSessionEntry{table: symbols.NewRootSymbolTable("dashboard"), lastUsed: time.Now()}
		codeSessions[uuid] = entry

		root := symbols.NewRootSymbolTable("dashboard")
		consoleTable := symbols.NewChildSymbolTable("console", root)

		compiler.AddStandard(root)

		comp := compiler.New("dashboard").
			SetExtensionsEnabled(true).
			SetRoot(consoleTable)

		if err := comp.AutoImport(true, consoleTable); err != nil {
			return nil, err
		}

		entry = &codeSessionEntry{table: consoleTable, lastUsed: time.Now()}
		codeSessions[uuid] = entry

		ui.Log(ui.ServerLogger, "admin.run.session.created", ui.A{"session": uuid})
	} else {
		entry.lastUsed = time.Now()
	}

	return entry.table, nil
}

// executeAdminEgo compiles and runs the given Ego source code, capturing
// program output via the bytecode context output buffer and returning it as a
// string. No global stdout redirection is performed, so concurrent requests do
// not interfere with each other.
func executeAdminEgo(source string, console bool, uuid string) (string, error) {
	s, err := getOrCreateSymbolTable(uuid)
	if err != nil {
		return "", err
	}

	if !console {
		s = symbols.NewChildSymbolTable("editor", s)
	}

	bc, err := compiler.CompileString("dashboard", source)
	if err != nil {
		return "", err
	}

	// Just to be sure we can never run off the end during a debug session,
	// add an extra stop instruction at the end.
	bc.Emit(bytecode.Stop)

	// Let's run this code!
	ctx := bytecode.NewContext(s, bc).EnableConsoleOutput(false)

	runErr := ctx.Run()

	return ctx.GetOutput(), runErr
}
