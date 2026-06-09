package debugger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

// promptSentinel is a special prefix written to the output channel to signal
// that the debugger needs input from the caller (rather than sending ordinary
// output text).  The sequence uses NUL bytes that cannot appear in normal
// debugger messages, so the receiver can reliably distinguish it.
const promptSentinel = "\x00PROMPT\x00"

// Response is the value returned by Resume on each round-trip with the caller.
//
// In API mode the debugger runs in a background goroutine.  Each Resume call
// delivers one command to that goroutine and then collects all output
// produced until the next input prompt (or until the session ends).
//
// Fields:
//
//   - Output:        Text written by the debugger itself (step notifications,
//     breakpoint messages, show-command results, etc.).
//     Display this in a "Debugger" panel.
//   - ProgramOutput: Text the running Ego program wrote to stdout.  In
//     interactive mode this is always empty because stdout
//     is not captured; in API mode it is kept separate so the
//     caller can display it in a distinct "Output" panel.
//   - Prompt:        The prompt string to show when collecting the next
//     command.  Empty when Done is true.
//   - Done:          True when the debug session has ended.  After this, the
//     context should be discarded.
//   - Line:          The source-line number the program is currently stopped at.
//   - Err:           Any non-normal termination error.  Nil for a clean exit.
type Response struct {
	Output        string
	ProgramOutput string
	Prompt        string
	Done          bool
	Line          int
	Err           error
}

// session holds the I/O wiring for one debugger invocation.
//
// In interactive mode (interactive == true) it reads from stdin and writes
// to stdout, exactly like a traditional command-line debugger.
//
// In API mode a background goroutine drives the debugger loop.  The caller
// exchanges data with that goroutine through three channels:
//
//   - outputCh:        receives all debugger output (text + promptSentinel messages)
//   - programOutputCh: receives text the running Ego program wrote to stdout
//   - inputCh:         the caller delivers the user's next command here
//   - doneCh:          the goroutine sends a final error (or nil) here when it exits
type session struct {
	writer      io.Writer // where ordinary debugger output goes
	interactive bool
	// API-mode channels — all nil in interactive mode.
	inputCh         chan string
	outputCh        chan string // debugger messages and promptSentinel signals
	programOutputCh chan string // captured Ego program stdout (API mode only)
	doneCh          chan error
	line            int // last source line number successfully executed
}

// printf writes a formatted string to the session's output destination.
// It is the low-level write primitive used by all other output helpers.
func (s *session) printf(format string, args ...any) {
	fmt.Fprintf(s.writer, format, args...)
}

// println writes a string followed by a newline to the session's output
// destination.
func (s *session) println(text string) {
	fmt.Fprintln(s.writer, text)
}

// say looks up msgID in the i18n message table, substitutes any named
// arguments from the optional args map, and writes the result to the
// session's output destination.  It mirrors ui.Say but routes output
// through the session abstraction so API-mode output is captured correctly.
func (s *session) say(msgID string, args ...map[string]any) {
	var text string
	if len(args) > 0 {
		text = i18n.T(msgID, args[0])
	} else {
		text = i18n.T(msgID)
	}

	if text != "" {
		fmt.Fprintln(s.writer, text)
	}
}

// writeProgramOutput routes text that the running Ego program produced
// (via fmt.Println, etc.) to the appropriate destination.
//
//   - Interactive mode: written directly to the session writer (stdout) so
//     it appears inline with debugger output.
//   - API mode: sent on programOutputCh so the caller can display it in a
//     separate panel from debugger messages.
func (s *session) writeProgramOutput(text string) {
	if s.interactive || s.programOutputCh == nil {
		fmt.Fprint(s.writer, text)
	} else {
		s.programOutputCh <- text
	}
}

// readLine displays prompt and returns the next line of user input.
//
//   - Interactive mode: delegates to readConsole which uses the readline
//     library for history and line editing.
//   - API mode: sends promptSentinel+prompt on outputCh to signal to the
//     Resume caller that input is needed, then blocks on inputCh until the
//     caller delivers the next command string.
func (s *session) readLine(prompt string) string {
	if s.interactive {
		return readConsole(prompt)
	}

	// Notify the Resume caller that the debugger is waiting for input.
	s.outputCh <- promptSentinel + prompt

	// Block until Resume delivers the user's command.
	return <-s.inputCh
}

// channelWriter implements io.Writer so that the bytecode context's output
// capture mechanism (which expects an io.Writer) can forward each Write to
// the outputCh channel in API mode.  Resume's collectResponse drains this
// channel.
type channelWriter struct {
	ch chan string
}

// Write converts the byte slice to a string and sends it on the channel.
// It is safe to call from a single goroutine; no locking is needed because
// only the debugger goroutine writes to outputCh.
func (w *channelWriter) Write(p []byte) (int, error) {
	if len(p) > 0 {
		w.ch <- string(p)
	}

	return len(p), nil
}

// sessions is the registry of all live API-mode debug sessions, keyed by the
// bytecode.Context pointer.  When Resume is called for a context the first
// time, a new entry is created; on subsequent calls the existing session is
// found and the user's input is delivered to it.
//
// sessionsMu protects concurrent access to the map — Close and Resume may be
// called from different goroutines.
var (
	sessionsMu sync.Mutex
	sessions   = map[*bytecode.Context]*session{}
)

// Run executes the bytecode context under the interactive debugger.
//
// It reads commands from stdin and writes output to stdout.  This is the
// entry point used by the CLI ("ego debug file.ego"); it blocks until the
// program finishes or the user types "exit".
func Run(c *bytecode.Context) error {
	sessionContext := &session{
		writer:      os.Stdout,
		interactive: true,
	}

	return runWithSession(c, sessionContext)
}

// Resume advances one step of an API-mode debug session.
//
// On the first call for a given context, Resume starts the debugger in a
// background goroutine and returns the initial output (the first debug stop
// or program completion).  On subsequent calls, it delivers the user's input
// string to the waiting goroutine and returns all output produced until the
// next prompt.
//
// When Response.Done is true the session is over and the context should be
// discarded.  Pass an empty string as input on the very first call.
//
// Typical API-mode usage:
//
//	resp := debugger.Resume(ctx, "")        // start: run to first stop
//	for !resp.Done {
//	    userCmd := getUserInput(resp.Prompt) // show resp.Output and prompt
//	    resp = debugger.Resume(ctx, userCmd) // send command, collect next output
//	}
func Resume(c *bytecode.Context, input string) Response {
	sessionsMu.Lock()
	sessionContext, exists := sessions[c]

	if !exists {
		// First call for this context.  Set up the channel infrastructure and
		// launch the debugger goroutine.
		ch := make(chan string, 64) // buffered so the goroutine rarely blocks on writes
		sessionContext = &session{
			writer:          &channelWriter{ch: ch},
			interactive:     false,
			inputCh:         make(chan string, 1),
			outputCh:        ch,
			programOutputCh: make(chan string, 64),
			doneCh:          make(chan error, 1),
		}

		sessions[c] = sessionContext
		sessionsMu.Unlock()

		go func() {
			err := runWithSession(c, sessionContext)

			// Normalise ErrStop to nil so the caller always sees a clean Done.
			if errors.Equals(err, errors.ErrStop) {
				err = nil
			}

			// Remove the session from the registry before signalling done, so
			// that a concurrent Resume call for the same context does not find
			// an already-finished session.
			sessionsMu.Lock()
			delete(sessions, c)
			sessionsMu.Unlock()

			sessionContext.doneCh <- err
		}()
	} else {
		sessionsMu.Unlock()

		// Subsequent call — deliver the user's command to the waiting goroutine.
		sessionContext.inputCh <- input
	}

	return collectResponse(sessionContext)
}

// Close tears down an API-mode debug session that is no longer needed — for
// example, when a REST client disconnects mid-session.
//
// It removes the session from the registry and attempts to unblock the
// debugger goroutine by sending an "exit" command to its input channel.
// If the goroutine is currently executing (not waiting for input) and the
// input channel is already full, the send is dropped; the goroutine will
// still read "exit" on its next readLine call provided the channel had room.
//
// It is safe to call Close on an already-finished session — the lookup will
// find nothing and the function returns without error.
func Close(c *bytecode.Context) {
	sessionsMu.Lock()

	sessionContext, exists := sessions[c]
	if exists {
		delete(sessions, c)
	}

	sessionsMu.Unlock()

	if exists {
		// Non-blocking send: if the goroutine is not waiting for input right now
		// (it's busy executing bytecode), inputCh has capacity 1 so the "exit"
		// is buffered and will be consumed on the next readLine call.  The
		// default case protects against the rare race where inputCh is already
		// full (e.g., a concurrent Resume sent a command just before Close ran).
		select {
		case sessionContext.inputCh <- "exit":
		default:
		}
	}
}

// collectResponse drains the debugger's output and program-output channels
// until a prompt sentinel (the goroutine needs input) or the done signal
// (the goroutine has exited) arrives.  It then assembles and returns a
// Response for the Resume caller.
//
// The two output channels (outputCh and programOutputCh) are drained
// concurrently using a select.  A message is classified as debugger output
// or program output based on which channel it arrives on.
func collectResponse(sessionContext *session) Response {
	var debugBuf, progBuf strings.Builder

	for {
		select {
		case msg := <-sessionContext.outputCh:
			if strings.HasPrefix(msg, promptSentinel) {
				// The goroutine is waiting for input — drain any buffered
				// program output and return the prompt response.
				drainProgramOutput(sessionContext, &progBuf)

				return Response{
					Output:        debugBuf.String(),
					ProgramOutput: progBuf.String(),
					Prompt:        strings.TrimPrefix(msg, promptSentinel),
					Line:          sessionContext.line,
				}
			}

			debugBuf.WriteString(msg)

		case msg := <-sessionContext.programOutputCh:
			progBuf.WriteString(msg)

		case err := <-sessionContext.doneCh:
			// The goroutine has finished.  Do a final non-blocking drain of both
			// output channels to capture any messages written just before exit.
		drainDebug:
			for {
				select {
				case msg := <-sessionContext.outputCh:
					if !strings.HasPrefix(msg, promptSentinel) {
						debugBuf.WriteString(msg)
					}
				default:
					break drainDebug
				}
			}

			drainProgramOutput(sessionContext, &progBuf)

			resp := Response{
				Output:        debugBuf.String(),
				ProgramOutput: progBuf.String(),
				Done:          true,
				Line:          sessionContext.line,
			}

			// Propagate runtime errors; ignore expected stop conditions.
			if err != nil && !errors.Equals(err, errors.ErrStop) {
				resp.Err = err
			}

			return resp
		}
	}
}

// drainProgramOutput performs a non-blocking drain of programOutputCh into
// buf, accumulating any buffered program stdout that arrived before the
// current prompt or done event.
func drainProgramOutput(sessionContext *session, buf *strings.Builder) {
	if sessionContext.programOutputCh == nil {
		return
	}

	for {
		select {
		case msg := <-sessionContext.programOutputCh:
			buf.WriteString(msg)
		default:
			return
		}
	}
}
