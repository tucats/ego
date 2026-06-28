package debugger

import (
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/compiler"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/runtime/io"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

// stepTo and breakAt are i18n message strings looked up once at package
// initialisation.  They are used as prefixes in the step and breakpoint
// notification messages printed to the user.
var (
	stepTo  = i18n.L("stepped.to")
	breakAt = i18n.L("break.at")
)

// runWithSession is the shared entry point used by both Run (interactive) and
// Resume (API mode).  It delegates to runFrom starting at program counter 0,
// which means execution begins from the very first instruction in the context.
func runWithSession(c *bytecode.Context, sessionContext *session) error {
	return runFrom(c, 0, sessionContext)
}

// runFrom sets the program counter to pc and then drives the bytecode run
// loop under debugger control.
//
// The loop calls c.Resume() which executes instructions until one of:
//
//   - ErrSignalDebugger — the AtLine opcode fired (start of a new source line
//     while the context is in debug mode).  debuggerPrompt is called to decide
//     whether to stop.
//   - ErrStop or context !IsRunning() — the program has finished normally.
//   - Any other error — a runtime error; return it to the caller.
//
// Between every c.Resume() call, captured program output (text the Ego
// program printed via fmt.Println etc.) is flushed through the session so it
// appears before the next debugger prompt.  This only has a visible effect in
// API mode — in interactive mode GetOutput() always returns "".
func runFrom(c *bytecode.Context, pc int, sessionContext *session) error {
	var err error

	c.SetPC(pc)

	for err == nil {
		err = c.Resume()

		// Flush any output the Ego program produced since the last stop.
		// In API mode the context captures stdout; in interactive mode this
		// is always a no-op.
		if out := c.GetOutput(); out != "" {
			sessionContext.writeProgramOutput(out)
			c.ClearOutput()
		}

		if errors.Equals(err, errors.ErrSignalDebugger) {
			// The run loop stopped at a new source line.  Ask the debugger
			// whether it wants to take control (breakpoint hit or single-step).
			sessionContext.line = c.GetLine()
			err = debuggerPrompt(c, sessionContext)
		}

		if !c.IsRunning() || errors.Equals(err, errors.ErrStop) {
			// The program has finished (or been exited by the user).  Return
			// nil so the caller sees a clean completion.
			return nil
		}
	}

	return err
}

// runAfterFirstToken is a helper used by the "set" and "call" debugger
// commands.  Both commands take the form:
//
//	set  variable = expression
//	call expression
//
// The leading verb (set / call) is stripped, and the remainder is compiled
// and run immediately in the current symbol-table scope so the user can
// assign variables or invoke functions from the debugger prompt.
//
// allowTrace controls whether the TRACE logger is active during execution:
//   - "call" passes true so each instruction is logged, giving the user
//     visibility into what the function does.
//   - "set" passes false to keep assignment output quiet.
//
// The trace state is saved and restored via a deferred call so that a panic
// inside compiler.Run cannot permanently change the logger state.
func runAfterFirstToken(s *symbols.SymbolTable, t *tokenizer.Tokenizer, allowTrace bool, sessionContext *session) error {
	_ = sessionContext // output routing for compiler.Run output is not yet implemented

	// Strip the leading verb by reading its spelling and then rebuilding the
	// tokenizer from everything that follows it in the source string.
	verb := t.GetTokens(0, 1, false)
	text := strings.TrimPrefix(strings.TrimSpace(t.GetSource()), verb)
	t2 := tokenizer.New(text, false)

	// Save and conditionally enable the trace logger for the duration of the
	// call.  The deferred restore ensures the logger state is never leaked
	// even if compiler.Run panics or returns early.
	traceMode := ui.IsActive(ui.TraceLogger)

	defer ui.Active(ui.TraceLogger, traceMode)
	ui.Active(ui.TraceLogger, allowTrace)

	err := compiler.Run("debugger", s, t2)
	if errors.Equals(err, errors.ErrStop) {
		err = nil
	}

	return err
}

// getLine reads a complete debugger command from the session, automatically
// accumulating continuation lines when the input has unclosed delimiters.
//
// A complete command is one where braces `{}`, parentheses `()`, and square
// brackets `[]` are all balanced, and no backtick string is still open.  For
// example, if the user types:
//
//	print map[string]int{
//
// the prompt changes to ".....>" and getLine reads another line before
// returning, because the opening `{` has not yet been closed.
//
// In interactive mode the underlying readLine calls use the readline library
// so the user gets history and line editing.  In API mode each readLine call
// blocks until Resume delivers the next command string.
func getLine(sessionContext *session) string {
	text := sessionContext.readLine("debug> ")
	if len(strings.TrimSpace(text)) == 0 {
		return ""
	}

	t := tokenizer.New(text, false)

	for {
		braceCount := 0
		parenCount := 0
		bracketCount := 0
		openTick := false

		// Check whether the last token is an unclosed backtick string.
		// A backtick string starts with "`" and ends with "`"; if the last
		// token starts with "`" but does not also end with "`", the string is
		// still open.
		lastToken := t.Tokens[len(t.Tokens)-1].Spelling()
		if lastToken[0:1] == "`" && lastToken[len(lastToken)-1:] != "`" {
			openTick = true
		}

		if !openTick {
			// Count unmatched openers and closers across the whole token stream.
			for _, v := range t.Tokens {
				switch v.Spelling() {
				case "[":
					bracketCount++
				case "]":
					bracketCount--
				case "(":
					parenCount++
				case ")":
					parenCount--
				case "{":
					braceCount++
				case "}":
					braceCount--
				}
			}
		}

		if braceCount > 0 || parenCount > 0 || bracketCount > 0 || openTick {
			// The command is not yet complete — read another line and re-tokenize
			// the combined text.
			text = text + sessionContext.readLine(".....> ")
			t = tokenizer.New(text, false)
		} else {
			// All delimiters are balanced — the command is complete.
			break
		}
	}

	return text
}

// readConsole reads a line of text from the user's terminal using the readline
// library (which provides history and line editing).  It is called only in
// interactive mode; API mode uses channel I/O instead.
func readConsole(prompt string) string {
	return io.ReadConsoleText(prompt)
}
