package debugger

import (
	"fmt"
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

// Run a context but allow the debugger to take control as
// needed.
func Run(c *bytecode.Context) error {
	return runFrom(c, 0)
}

func runFrom(c *bytecode.Context, pc int) error {
	var err error

	c.SetPC(pc)

	for err == nil {
		err = c.Resume()
		if errors.Equals(err, errors.ErrSignalDebugger) {
			err = Debugger(c)
		}

		if !c.IsRunning() || errors.Equals(err, errors.ErrStop) {
			return nil
		}
	}

	return err
}

// This is called on AtLine to offer the chance for the debugger to take control.
func Debugger(c *bytecode.Context) error {
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
				ui.Say("msg.debug.no.source")
			}
		}
	}

	// Are we in single-step mode?
	if c.SingleStep() {
		// Big hack here. Let's change the text of the "@entrypoint" directive
		// to be more easily read by the user when the debugger runs.
		if line < 0 {
			ui.Say("msg.debug.return")
		} else if strings.HasPrefix(text, "@entrypoint ") {
			// Strip of the directive and the parser's helpful trailing semicolon.
			entry := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(text, "@entrypoint")), ";"))
			ui.Say("msg.debug.start", map[string]any{
				"name": entry,
			})
		} else {
			fmt.Printf("%s:\n  %s %3d, %s\n", stepTo, data.SanitizeName(c.GetModuleName()), line, text)
		}

		prompt = true
	} else {
		prompt = evaluationBreakpoint(c)
	}

	for prompt {
		var tokens *tokenizer.Tokenizer

		for {
			cmd := getLine()
			if len(strings.TrimSpace(cmd)) == 0 {
				cmd = "step"
			}

			tokens = tokenizer.New(cmd, false)
			if !tokens.AtEnd() {
				break
			}
		}

		// We have a command now in the tokens buffer.
		if err == nil {
			t := tokens.Peek(1)
			switch t.Spelling() {
			case "help":
				_ = showHelp()

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
					// No action, this is the default step case

				default:
					prompt = true
					err = errors.ErrInvalidStepType.Context(tokens.Peek(2))

					c.SetSingleStep(false)
				}

			case "show":
				err = showCommand(s, tokens, line, c)

			case "set":
				err = runAfterFirstToken(s, tokens, false)

			case "call":
				err = runAfterFirstToken(s, tokens, true)

			case "print":
				text := strings.ReplaceAll("fmt.Println("+strings.Replace(tokens.GetSource(), "print", "", 1)+")", "\n", "")
				t2 := tokenizer.New(text, true)

				traceMode := ui.IsActive(ui.TraceLogger)
				ui.Active(ui.TraceLogger, false)

				err = compiler.Run("debugger", s, t2)
				if errors.Equals(err, errors.ErrStop) {
					err = nil
				}

				ui.Active(ui.TraceLogger, traceMode)

			case "break":
				err = breakCommand(tokens)

			case "exit":
				return errors.ErrStop

			default:
				err = errors.ErrInvalidDebugCommand.Context(t)
			}

			if err != nil && !errors.Equals(err, errors.ErrStop) && !errors.Equals(err, errors.ErrStepOver) {
				ui.Say("msg.debug.error", map[string]any{
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

func runAfterFirstToken(s *symbols.SymbolTable, t *tokenizer.Tokenizer, allowTrace bool) error {
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

// getLine reads a line of text from the console, and requires that it contain matching
// tick-quotes and braces.
func getLine() string {
	text := io.ReadConsoleText("debug> ")
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
			text = text + io.ReadConsoleText(".....> ")
			t = tokenizer.New(text, false)

			continue
		} else {
			break
		}
	}

	return text
}
