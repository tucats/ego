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

// promptSentinel is a prefix written to the output channel to distinguish a
// "prompt for input" signal from ordinary output text. It is intentionally an
// unprintable sequence that cannot appear in real debugger output.
const promptSentinel = "\x00PROMPT\x00"

// Response is the value returned by Resume on each round-trip with the caller.
//
// Output contains text produced by the debugger itself (step notifications,
// breakpoint messages, show-command results, etc.) and should be displayed in
// a dedicated "Debugger" view.
//
// ProgramOutput contains text the running Ego program wrote to stdout
// (fmt.Println and friends). In interactive mode ProgramOutput is always
// empty because stdout is not captured; in API mode it is separated so the
// caller can display it in a distinct "Output" panel.
//
// Prompt is the debugger prompt string the caller should display when
// collecting the next command. When Done is true the debug session has ended;
// Err carries any non-normal termination error.
type Response struct {
	Output        string
	ProgramOutput string
	Prompt        string
	Done          bool
	Err           error
}

// session holds the I/O wiring for one debugger invocation. In interactive
// mode it reads from stdin and writes to stdout. In API mode a background
// goroutine runs the debugger loop; Resume exchanges data with it via
// channels.
type session struct {
	writer      io.Writer // where ordinary debugger output goes
	interactive bool
	// API-mode channels (nil in interactive mode)
	inputCh         chan string
	outputCh        chan string // debugger messages + promptSentinel
	programOutputCh chan string // captured program stdout (API mode only)
	doneCh          chan error
}

// printf writes a formatted string to the session's output writer.
func (s *session) printf(format string, args ...any) {
	fmt.Fprintf(s.writer, format, args...)
}

// println writes a string followed by a newline to the session's output writer.
func (s *session) println(text string) {
	fmt.Fprintln(s.writer, text)
}

// say translates a message key and writes the result to the session's output
// writer, mirroring the behavior of ui.Say.
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

// writeProgramOutput routes text that the Ego program wrote to stdout.
// In interactive mode it is written to the session writer (stdout).
// In API mode it is sent on programOutputCh so the caller can display it
// separately from debugger messages.
func (s *session) writeProgramOutput(text string) {
	if s.interactive || s.programOutputCh == nil {
		fmt.Fprint(s.writer, text)
	} else {
		s.programOutputCh <- text
	}
}

// readLine displays prompt and returns the next line of input.
//
// In interactive mode it delegates to the console reader (preserving the
// existing readline history behavior). In API mode it signals the caller
// by sending promptSentinel+prompt on outputCh, then blocks until a reply
// arrives on inputCh.
func (s *session) readLine(prompt string) string {
	if s.interactive {
		return readConsole(prompt)
	}

	// Signal the Resume caller that we need input.
	s.outputCh <- promptSentinel + prompt

	// Wait for the input the caller sends back.
	return <-s.inputCh
}

// channelWriter implements io.Writer and forwards each Write to outputCh as
// a plain string so Resume can accumulate them.
type channelWriter struct {
	ch chan string
}

func (w *channelWriter) Write(p []byte) (int, error) {
	if len(p) > 0 {
		w.ch <- string(p)
	}

	return len(p), nil
}

// sessions stores the active API-mode session for each live context so that
// Resume can route subsequent calls to the same background goroutine.
var (
	sessionsMu sync.Mutex
	sessions   = map[*bytecode.Context]*session{}
)

// Run executes the bytecode context under the interactive debugger, reading
// commands from stdin and writing output to stdout. This is the existing
// entry point used by the CLI; its behavior is unchanged.
func Run(c *bytecode.Context) error {
	sessionContext := &session{
		writer:      os.Stdout,
		interactive: true,
	}

	return runWithSession(c, sessionContext)
}

// Resume advances one step of an API-mode debug session: it delivers input to
// the waiting debugger goroutine (or starts the goroutine on the first call)
// and then collects all output until the goroutine next needs input.
//
// Pass an empty input string on the first call to run the program to its first
// debug stop and retrieve the initial state display and prompt. Subsequent
// calls should pass the command string supplied by the user.
//
// When Response.Done is true the debug session has ended and the context
// should be discarded.
func Resume(c *bytecode.Context, input string) Response {
	sessionsMu.Lock()
	sessionContext, exists := sessions[c]

	if !exists {
		// First call for this context — start the debugger goroutine.
		ch := make(chan string, 64)
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
			// normalize expected termination codes so callers see a clean Done.
			if errors.Equals(err, errors.ErrStop) {
				err = nil
			}
			
			sessionsMu.Lock()
			delete(sessions, c)
			sessionsMu.Unlock()
			sessionContext.doneCh <- err
		}()
	} else {
		sessionsMu.Unlock()
		// Deliver the caller's input to the waiting goroutine.
		sessionContext.inputCh <- input
	}

	return collectResponse(sessionContext)
}

// Close tears down an API-mode debug session that is no longer needed (for
// example, when the REST client disconnects). It is safe to call on an already
// finished session.
func Close(c *bytecode.Context) {
	sessionsMu.Lock()

	sessionContext, exists := sessions[c]
	if exists {
		delete(sessions, c)
	}

	sessionsMu.Unlock()

	if exists {
		// Unblock the goroutine if it is waiting for input, so it can exit.
		select {
		case sessionContext.inputCh <- "exit":
		default:
		}
	}
}

// collectResponse drains outputCh (debugger messages) and programOutputCh
// (program stdout) until a prompt sentinel or done signal arrives, then
// assembles and returns the Response.
func collectResponse(sessionContext *session) Response {
	var debugBuf, progBuf strings.Builder

	for {
		select {
		case msg := <-sessionContext.outputCh:
			if strings.HasPrefix(msg, promptSentinel) {
				// Drain any buffered program output before returning the prompt.
				drainProgramOutput(sessionContext, &progBuf)

				return Response{
					Output:        debugBuf.String(),
					ProgramOutput: progBuf.String(),
					Prompt:        strings.TrimPrefix(msg, promptSentinel),
				}
			}

			debugBuf.WriteString(msg)

		case msg := <-sessionContext.programOutputCh:
			progBuf.WriteString(msg)

		case err := <-sessionContext.doneCh:
			// Drain any output the goroutine wrote before exiting.
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
			}
			if err != nil && !errors.Equals(err, errors.ErrStop) {
				resp.Err = err
			}

			return resp
		}
	}
}

// drainProgramOutput drains all buffered messages from programOutputCh into buf
// without blocking.
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
