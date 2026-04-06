package debugger

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/io"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

var (
	stepTo  = i18n.L("stepped.to")
	breakAt = i18n.L("break.at")
)

// runWithSession is the common implementation shared by Run and Resume. It
// runs the bytecode context under the debugger, using sessionContext for all I/O.
func runWithSession(c *bytecode.Context, sessionContext *session) error {
	return runFrom(c, 0, sessionContext)
}

func runFrom(c *bytecode.Context, pc int, sessionContext *session) error {
	var err error

	c.SetPC(pc)

	for err == nil {
		err = c.Resume()

		// Flush any output the Ego program wrote between debugger stops so it
		// appears in the session stream before the next prompt. This only has
		// an effect when the context is in capture mode (EnableConsoleOutput(false));
		// in interactive mode GetOutput() always returns "".
		if out := c.GetOutput(); out != "" {
			sessionContext.writeProgramOutput(out)
			c.ClearOutput()
		}

		if errors.Equals(err, errors.ErrSignalDebugger) {
			err = debuggerPrompt(c, sessionContext)
		}

		if !c.IsRunning() || errors.Equals(err, errors.ErrStop) {
			return nil
		}
	}

	return err
}

// debuggerPrompt is called on AtLine to offer the debugger a chance to take
// control. It replaces the old exported Debugger function. All output is
// written through sessionContext; all input is read through sessionContext.
func debuggerPrompt(c *bytecode.Context, sessionContext *session) error {
	var (
		err    error
		text   string
		prompt bool
		line   = c.GetLine()
		s      = c.GetSymbols()
	)

	if line > 0 {
		if text = c.GetSource(); text == "" {
			if tok := c.GetTokenizer(); tok != nil {
				text = tok.GetLine(line)
			} else {
				sessionContext.say("msg.debug.no.source")
			}
		}
	}

	// Are we in single-step mode?
	if c.SingleStep() {
		if line < 0 {
			// line < 0 means the debugger was signalled at a return-from-entrypoint
			// marker — the program has no more source lines to execute.  Write the
			// "return from entrypoint" message and signal completion to the caller
			// rather than prompting for another command.
			sessionContext.say("msg.debug.return")

			return errors.ErrStop
		} else if strings.HasPrefix(text, "@entrypoint ") {
			entry := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(text, "@entrypoint")), ";"))
			sessionContext.say("msg.debug.start", map[string]any{
				"name": entry,
			})
		} else {
			sessionContext.printf("%s:\n  %s %3d, %s\n", stepTo, data.SanitizeName(c.GetModuleName()), line, text)
		}

		prompt = true
	} else {
		prompt = evaluationBreakpoint(c, sessionContext)
	}

	for prompt {
		var tokens *tokenizer.Tokenizer

		// Read a complete command, looping for continuation lines if needed.
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

		// Process the command.
		if err == nil {
			t := tokens.Peek(1)
			switch t.Spelling() {
			case "help":
				_ = showHelp(sessionContext)

			case "go", "continue":
				c.SetSingleStep(false)
				prompt = false

			case "step":
				c.SetSingleStep(true)
				c.SetStepOver(false)
				prompt = false

				switch tokens.PeekText(2) {
				case "over":
					c.SetStepOver(true)

				case "into":
					c.SetStepOver(false)

				case "return":
					c.SetBreakOnReturn()
					c.SetSingleStep(false)

				case "":
					// Default step — no extra action.

				default:
					prompt = true
					err = errors.ErrInvalidStepType.Context(tokens.Peek(2))

					c.SetSingleStep(false)
				}

			case "show":
				err = showCommand(s, tokens, line, c, sessionContext)

			case "set":
				err = runAfterFirstToken(s, tokens, false, sessionContext)

			case "call":
				err = runAfterFirstToken(s, tokens, true, sessionContext)

			case "print":
				text := strings.ReplaceAll("fmt.Println("+strings.Replace(tokens.GetSource(), "print", "", 1)+")", "\n", "")
				traceMode := ui.IsActive(ui.TraceLogger)
				ui.Active(ui.TraceLogger, false)

				bc, err := compiler.CompileString("debugger", text)
				if err == nil {
					printCtx := bytecode.NewContext(c.GetSymbols(), bc).EnableConsoleOutput(false)

					err = printCtx.Run()
					if err == nil || errors.Equal(err, errors.ErrStop) {
						output := printCtx.GetOutput()
						output = strings.TrimSuffix(output, "\n")
						sessionContext.println(output)
					}
				} else {
					sessionContext.say("msg.debug.error", map[string]any{
						"err": err,
					})
				}

				ui.Active(ui.TraceLogger, traceMode)

			case "break":
				err = breakCommand(tokens, sessionContext)

			case "exit":
				return errors.ErrStop

			default:
				err = errors.ErrInvalidDebugCommand.Context(t)
			}

			if err != nil && !errors.Equals(err, errors.ErrStop) && !errors.Equals(err, errors.ErrStepOver) {
				sessionContext.say("msg.debug.error", map[string]any{
					"err": err,
				})

				err = nil
			}

			if errors.Equals(err, errors.ErrStop) {
				err = nil
				prompt = false
			}
		}
	}

	return err
}

func runAfterFirstToken(s *symbols.SymbolTable, t *tokenizer.Tokenizer, allowTrace bool, sessionContext *session) error {
	_ = sessionContext // reserved for future output routing of compiler.Run results
	verb := t.GetTokens(0, 1, false)
	text := strings.TrimPrefix(strings.TrimSpace(t.GetSource()), verb)
	t2 := tokenizer.New(text, false)

	traceMode := ui.IsActive(ui.TraceLogger)

	defer ui.Active(ui.TraceLogger, traceMode)
	ui.Active(ui.TraceLogger, allowTrace)

	err := compiler.Run("debugger", s, t2)
	if errors.Equals(err, errors.ErrStop) {
		err = nil
	}

	return err
}

// getLine reads a complete command from the session, handling continuation
// lines for unbalanced braces, parentheses, and backtick strings. In
// interactive mode it uses the readline-capable console reader; in API mode
// it reads from the session channel.
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

		lastToken := t.Tokens[len(t.Tokens)-1].Spelling()
		if lastToken[0:1] == "`" && lastToken[len(lastToken)-1:] != "`" {
			openTick = true
		}

		if !openTick {
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
			text = text + sessionContext.readLine(".....> ")
			t = tokenizer.New(text, false)
		} else {
			break
		}
	}

	return text
}

// readConsole reads a line of text from the user's console using the readline
// library. It is used only in interactive mode.
func readConsole(prompt string) string {
	return io.ReadConsoleText(prompt)
}
