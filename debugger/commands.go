package debugger

import (
	"strings"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// debuggerPrompt is called every time the bytecode run loop emits
// ErrSignalDebugger — once per source line when the context is in debug mode.
//
// It decides whether to stop and show a prompt (single-step mode or a
// breakpoint is hit) or simply return so execution continues.  When it does
// stop, it loops reading and dispatching commands until the user enters a
// command that resumes execution (step, continue, go) or stops the program
// (exit).
//
// All output goes through sessionContext so the function works identically
// in interactive mode (stdout) and API mode (channel-based I/O).
func debuggerPrompt(c *bytecode.Context, sessionContext *session) error {
	var (
		err    error
		text   string
		prompt bool
		line   = c.GetLine()
		s      = c.GetSymbols()
	)

	// Retrieve the source text for the current line.  The context may have
	// a cached copy (c.GetSource); if not, fall back to the tokenizer's own
	// line table.  If neither is available, announce that source is missing.
	if line > 0 {
		if text = c.GetSource(); text == "" {
			if tok := c.GetTokenizer(); tok != nil {
				text = tok.GetLine(line)
			} else {
				sessionContext.say("msg.debug.no.source")
			}
		}
	}

	// Decide whether to enter the interactive prompt this stop.
	if c.SingleStep() {
		if line < 0 {
			// A negative line number is a special marker emitted at the end of
			// the top-level entrypoint — there is no more user code to execute.
			// Signal the caller to stop the debug session cleanly.
			sessionContext.say("msg.debug.return")

			return errors.ErrStop
		} else if strings.HasPrefix(text, "@entrypoint ") {
			// An @entrypoint directive marks the start of a named function body.
			// Show the function name but do not count this as a steppable line.
			entry := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(text, "@entrypoint")), ";"))
			sessionContext.say("msg.debug.start", map[string]any{
				"name": entry,
			})
		} else {
			// Normal single-step: print the module name, line number, and source text.
			sessionContext.printf("%s:\n  %s %3d, %s\n", stepTo, data.SanitizeName(c.GetModuleName()), line, text)
		}

		prompt = true
	} else {
		// Not in single-step mode — check whether any breakpoint fires here.
		prompt = evaluationBreakpoint(c, sessionContext)
	}

	// Interactive command loop.  Each iteration reads one complete command
	// (with automatic continuation for unbalanced braces) and dispatches it.
	for prompt {
		var tokens *tokenizer.Tokenizer

		// Keep reading until a non-empty command is assembled.  An empty
		// command line is treated as "step" (same as pressing Enter in GDB).
		for {
			cmd := getLine(sessionContext)
			if len(strings.TrimSpace(cmd)) == 0 {
				cmd = "step"
			}

			tokens = tokenizer.New(cmd, false)
			if !tokens.AtEnd() {
				break
			}
		}

		// Dispatch on the first token of the command.
		if err == nil {
			t := tokens.Peek(1)
			switch t.Spelling() {
			case "help":
				// Display the command reference table.
				_ = showHelp(sessionContext)

			case "go", "continue":
				// Resume execution without stopping at every line.
				c.SetSingleStep(false)
				prompt = false

			case "step":
				// Execute exactly one source line.  Optional sub-words qualify
				// the step mode.
				c.SetSingleStep(true)
				c.SetStepOver(false)
				prompt = false

				switch tokens.PeekText(2) {
				case "over":
					// Step over: execute calls as a single unit without entering them.
					c.SetStepOver(true)

				case "into":
					// Step into: descend into function calls (the default).
					c.SetStepOver(false)

				case "return":
					// Step return: run until the current function returns, then stop.
					c.SetBreakOnReturn()
					c.SetSingleStep(false)

				case "":
					// Plain "step" with no qualifier — default behaviour.

				default:
					// Unrecognised qualifier — report the error and stay in the prompt.
					prompt = true
					err = errors.ErrInvalidStepType.Context(tokens.Peek(2))

					c.SetSingleStep(false)
				}

			case "show":
				// Display information about current program state.
				err = showCommand(s, tokens, line, c, sessionContext)

			case "set":
				// Assign a value to a variable in the current scope.
				// runAfterFirstToken strips the "set" keyword and compiles/runs
				// the remainder as a statement.
				err = runAfterFirstToken(s, tokens, false, sessionContext)

			case "call":
				// Invoke a function expression with tracing enabled so the user
				// can see each step inside the called function.
				err = runAfterFirstToken(s, tokens, true, sessionContext)

			case "print":
				// Evaluate and print an expression.  We wrap the expression in a
				// fmt.Println() call so the result is always displayed, even for
				// values that would not ordinarily be printed.
				//
				// A fresh child symbol table is created so that any temporary
				// variables used by the print expression do not leak into the
				// program's scope.  Console-output capture is controlled by
				// whether this is an interactive or API-mode session.
				text := strings.ReplaceAll("fmt.Println("+strings.Replace(tokens.GetSource(), "print", "", 1)+")", "\n", "")

				bc, err := compiler.CompileString("print command", text)
				if err == nil {
					// Make a new child table so we can put the console writer in
					// the temporary scope for non-interactive sessions.  The local
					// scope is discarded when this command finishes.
					printSymbolTable := symbols.NewChildSymbolTable("dbg print", c.GetSymbols())
					printCtx := bytecode.NewContext(printSymbolTable, bc).EnableConsoleOutput(sessionContext.interactive)

					err = printCtx.Run()

					// DEBUGGER-COMMANDS-1: Use errors.Equals (key-based comparison)
					// rather than errors.Equal (full-string comparison).
					//
					// errors.Equal compares the formatted .Error() strings of both
					// sides.  When the child context exits via ErrStop it often
					// carries context (e.g. ErrStop.Context("line 5")), whose
					// .Error() string is "stop: line 5" — which does NOT match the
					// bare "stop" string from errors.ErrStop.Error().  That would
					// cause the print output to be silently discarded and replaced
					// with a spurious error message.
					//
					// errors.Equals delegates to (*Error).Is(), which compares only
					// the underlying i18n key and ignores any .Context(...) suffix,
					// so any flavour of ErrStop is correctly treated as a clean stop.
					if err == nil || errors.Equals(err, errors.ErrStop) {
						output := strings.TrimSuffix(printCtx.GetOutput(), "\n")
						sessionContext.println(output)
					} else {
						sessionContext.say("msg.debug.error", map[string]any{
							"err": err,
						})
					}
				} else {
					sessionContext.say("msg.debug.error", map[string]any{
						"err": err,
					})
				}

			case "break":
				// Add, remove, save, or load a breakpoint.
				err = breakCommand(tokens, sessionContext)

			case "exit":
				// Stop the program immediately.
				return errors.ErrStop

			default:
				// Unrecognised command.
				err = errors.ErrInvalidDebugCommand.Context(t)
			}

			// If the command produced an error that is not a stop or step-over
			// signal, report it to the user and clear it — we stay in the prompt
			// so the user can try again rather than crashing the debug session.
			if err != nil && !errors.Equals(err, errors.ErrStop) && !errors.Equals(err, errors.ErrStepOver) {
				sessionContext.say("msg.debug.error", map[string]any{
					"err": err,
				})

				err = nil
			}

			// A clean ErrStop means "exit the prompt and stop execution".
			if errors.Equals(err, errors.ErrStop) {
				err = nil
				prompt = false
			}
		}
	}

	return err
}
