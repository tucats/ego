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

				bc, err := compiler.CompileString("print command", text)
				if err == nil {
					// Make a new child table so we can put the console writer in the temporary scope if this
					// isn't an interactive session. The local-most scope is discarded when this command finishes,
					// so the optional temporary addition of a stdout writer is discarded.
					printSymbolTable := symbols.NewChildSymbolTable("dbg print", c.GetSymbols())
					printCtx := bytecode.NewContext(printSymbolTable, bc).EnableConsoleOutput(sessionContext.interactive)

					err = printCtx.Run()
					if err == nil || errors.Equal(err, errors.ErrStop) {
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
