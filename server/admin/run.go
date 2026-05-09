package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/debugger"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Maximum number of active code or debug sessions that can exist at one time.
// This is to limit memory consumption from DoS attack against run endpoints.
const maxDebugSessions = 20
const maxCodeSessions = 20

// Maximum size of the POST /admin/run request body and Code field (256 KiB).
// Enforced before JSON decoding to prevent memory exhaustion from large payloads.
const maxRunBodyBytes = 256 << 10
const maxRunCodeBytes = 256 << 10

// traceRunMu serializes requests that enable trace logging so that the global
// TraceLogger state is not concurrently mutated by multiple handlers.
var traceRunMu sync.Mutex

// elapsedString formats a duration for display in the dashboard. Go's standard
// time.Duration.String() uses the Unicode µ character for sub-millisecond values
// (e.g. "586.75µs"), which renders incorrectly in some browser fonts. Instead,
// sub-millisecond durations are expressed as a fractional millisecond value.
func elapsedString(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.4fms", float64(d)/float64(time.Millisecond))
	}

	return d.String()
}

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
//
//	valid when debug mode is active.
type codeRunResponse struct {
	Output        string `json:"output,omitempty"`
	Error         string `json:"error,omitempty"`
	DebugOutput   string `json:"debugOutput,omitempty"`
	ProgramOutput string `json:"programOutput,omitempty"`
	DebugPrompt   string `json:"debugPrompt,omitempty"`
	DebugWaiting  bool   `json:"debugWaiting,omitempty"`
	Line          int    `json:"line"`
	Elapsed       string `json:"elapsed"`
}

// codeSessionEntry holds a per-session persistent symbol table together with
// the authenticated username that created it.
type codeSessionEntry struct {
	owner string
	table *symbols.SymbolTable
}

// debugSession holds the live bytecode context for an active debug session
// along with the authenticated username that created it.
type debugSession struct {
	owner string
	ctx   *bytecode.Context
}

func init() {
	// Symbol table sessions expire after one hour of inactivity.
	caches.SetExpiration(caches.SymbolTableCache, "1h")

	// Debug sessions expire after 15 minutes of inactivity.
	caches.SetExpiration(caches.DebugSessionCache, "15m")
}

// RunCodeHandler is the HTTP handler for POST /admin/run.
func RunCodeHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var req codeRunRequest

	// Limit body size before decoding to prevent memory exhaustion (CODE-M1).
	r.Body = http.MaxBytesReader(w, r.Body, maxRunBodyBytes)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		status := http.StatusBadRequest
		if _, ok := err.(*http.MaxBytesError); ok {
			status = http.StatusRequestEntityTooLarge
		}

		return util.ErrorResponse(w, session.ID, err.Error(), status)
	}

	if len(req.Code) > maxRunCodeBytes {
		return util.ErrorResponse(w, session.ID, "code payload too large", http.StatusRequestEntityTooLarge)
	}

	// Validate the caller-supplied session UUID before using it as a map key.
	// This prevents log injection and rejects attempts to reference sessions by
	// arbitrary strings (CODE-M3).
	if req.Session != "" {
		if _, err := uuid.Parse(req.Session); err != nil {
			return util.ErrorResponse(w, session.ID, "invalid session id", http.StatusBadRequest)
		}
	}

	if ui.IsActive(ui.RestLogger) {
		// Truncate the Code field so that large scripts are not written verbatim
		// to the server log (CODE-L1).
		logReq := req
		if len(logReq.Code) > 120 {
			logReq.Code = logReq.Code[:120] + "..."
		}

		b, _ := json.MarshalIndent(logReq, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    string(b),
		})
	}

	startTime := time.Now()

	var resp codeRunResponse

	if req.Session == "" {
		req.Session = uuid.New().String()
	}

	if req.Debug {
		resp = executeAdminDebug(session.ID, session.User, req.Code, req.DebugInput, req.Trace, req.Session)
	} else {
		if req.Trace {
			// Trace logging mutates the global TraceLogger state. Serialize
			// trace-enabled requests so concurrent executions do not interfere
			// with each other's saved trace state (CODE-M2).
			traceRunMu.Lock()
			ui.Active(ui.TraceLogger, true)

			defer func() {
				ui.Active(ui.TraceLogger, false)
				traceRunMu.Unlock()
			}()
		}

		output, runErr := executeAdminEgo(session, req.Code, req.Console, req.Trace, req.Session)

		resp = codeRunResponse{Output: output, Elapsed: elapsedString(time.Since(startTime))}
		if runErr != nil {
			resp.Error = runErr.Error()
		}
	}

	w.Header().Set("Content-Type", "application/json")

	_ = util.WriteJSON(w, resp, &session.ResponseLength)

	// Prepare the body to be logged as well. If the text is longer than
	// 120 characters, let's truncate it. We only do this if the logger
	// is active, just to be a teeny bit more efficient.
	if ui.IsActive(ui.RestLogger) {
		body := resp.Output
		text := strings.Builder{}

		for i, ch := range body {
			if i > 117 {
				text.WriteString("...")

				break
			}

			text.WriteRune(ch)
		}

		resp.Output = text.String()
		b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		ui.Log(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b),
		})
	}

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
func executeAdminDebug(session int, user, code, debugInput string, tracing bool, uuid string) codeRunResponse {
	startTime := time.Now()

	var ctx *bytecode.Context

	if v, found := caches.Find(caches.DebugSessionCache, uuid); found {
		entry := v.(*debugSession)

		// Reject requests from a different user than the one who created the
		// session. Return a generic error to avoid confirming that the session
		// exists (CODE-M3).
		if entry.owner != user {
			return codeRunResponse{Error: i18n.T("msg.run.not.found")}
		}

		ctx = entry.ctx
		ctx.SetTrace(tracing).Sandboxed(true)

		if debugInput == "" {
			// Nothing to deliver; return waiting state so the caller can re-show the prompt.
			return codeRunResponse{
				DebugWaiting: true,
				DebugPrompt:  "debug> ",
				Elapsed:      elapsedString(time.Since(startTime)),
			}
		}
	} else {
		// Do we already have too many sessions?
		if caches.Size(caches.DebugSessionCache) >= maxDebugSessions {
			return codeRunResponse{Error: errors.ErrMaxDebugSessions.Context(caches.Size(caches.DebugSessionCache)).Error()}
		}

		// No existing session — compile code and create a new debug context.
		if code == "" {
			return codeRunResponse{Error: i18n.T("msg.run.no.session")}
		}

		s, err := getOrCreateSymbolTable(session, user, uuid)
		if err != nil {
			return codeRunResponse{Error: err.Error()}
		}

		// Editor debug runs use a fresh child table so each run starts clean.
		s = symbols.NewChildSymbolTable("debug-editor", s)

		bc, compileErr := compiler.CompileString("dashboard", code)
		if compileErr != nil {
			return codeRunResponse{
				ProgramOutput: compileErr.Error(),
				Error:         compileErr.Error(),
				Elapsed:       elapsedString(time.Since(startTime)),
			}
		}

		ctx = bytecode.NewContext(s, bc).
			SetDebug(true).
			Sandboxed(true).
			SetTrace(tracing).
			EnableConsoleOutput(false) // capture program output into the session channelWriter

		caches.Add(caches.DebugSessionCache, uuid, &debugSession{owner: user, ctx: ctx})
	}

	// First call passes "" to start the goroutine; subsequent calls pass the command.
	dbResp := debugger.Resume(ctx, debugInput)

	if dbResp.Done {
		caches.Delete(caches.DebugSessionCache, uuid)

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
		Elapsed:       elapsedString(time.Since(startTime)),
	}
}

// getOrCreateSymbolTable returns the persistent console symbol table for the
// given UUID, creating it on first use. Returns an error if the session exists
// but was created by a different user (CODE-M3).
func getOrCreateSymbolTable(session int, user, uuid string) (*symbols.SymbolTable, error) {
	if v, found := caches.Find(caches.SymbolTableCache, uuid); found {
		entry := v.(*codeSessionEntry)

		// Reject access from a different user to prevent session fixation (CODE-M3).
		if entry.owner != user {
			return nil, errors.ErrNoPrivilegeForOperation.Context("session")
		}

		return entry.table, nil
	}

	// Have we exceeded the maximum number of Code sessions?
	if caches.Size(caches.SymbolTableCache) >= maxCodeSessions {
		return nil, errors.ErrMaxCodeSessions.Context(caches.Size(caches.SymbolTableCache))
	}

	// No existing session — create a new console symbol table.
	ui.Log(ui.ServerLogger, "admin.run.session.created", ui.A{
		"session": session,
		"id":      uuid})

	root := symbols.NewRootSymbolTable("dashboard")
	consoleTable := symbols.NewChildSymbolTable("console", root)

	compiler.AddStandard(root)

	comp := compiler.New("dashboard").
		SetExtensionsEnabled(true).
		SetRoot(consoleTable)

	if err := comp.AutoImport(true, consoleTable); err != nil {
		return nil, err
	}

	caches.Add(caches.SymbolTableCache, uuid, &codeSessionEntry{owner: user, table: consoleTable})

	return consoleTable, nil
}

// executeAdminEgo compiles and runs the given Ego source code, capturing
// program output via the bytecode context output buffer and returning it as a
// string. No global stdout redirection is performed, so concurrent requests do
// not interfere with each other.
func executeAdminEgo(session *server.Session, source string, console bool, trace bool, uuid string) (string, error) {
	s, err := getOrCreateSymbolTable(session.ID, session.User, uuid)
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
	ctx := bytecode.NewContext(s, bc).Sandboxed(!session.Admin).EnableConsoleOutput(false).SetTrace(trace)

	runErr := ctx.Run()

	return ctx.GetOutput(), runErr
}
