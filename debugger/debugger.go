package debugger

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

var stepTo = i18n.L("stepped.to")
var breakAt = i18n.L("break.at")

// Run a context but allow the debugger to take control as
// needed.
func Run(c *bytecode.Context) *errors.EgoError {
	return RunFrom(c, 0)
}

func RunFrom(c *bytecode.Context, pc int) *errors.EgoError {
	var err *errors.EgoError

	c.SetPC(pc)

	for errors.Nil(err) {
		err = c.Resume()
		if err.Is(errors.ErrSignalDebugger) {
			err = Debugger(c)
		}

		if !c.IsRunning() || err.Is(errors.ErrStop) {
			return nil
		}
	}

	return err
}

// This is called on AtLine to offer the chance for the debugger to take control.
func Debugger(c *bytecode.Context) *errors.EgoError {
	var err *errors.EgoError

	line := c.GetLine()
	text := ""

	if line > 0 {
		if tok := c.GetTokenizer(); tok != nil {
			text = tok.GetLine(line)
		} else {
			ui.Say("msg.debug.no.source")
		}
	}

	s := c.GetSymbols()
	prompt := false

	// Are we in single-step mode?
	if c.SingleStep() {
		// Big hack here. Let's change the text of the "@entrypoint" directive
		// to be more easily read by the user when the debugger runs.
		if line < 0 {
			ui.Say("msg.debug.return")
		} else if strings.HasPrefix(text, "@entrypoint ") {
			ui.Say("msg.debug.start", map[string]interface{}{
				"name": strings.TrimPrefix(text, "@entrypoint "),
			})
		} else {
			fmt.Printf("%s:\n  %s %3d, %s\n", stepTo, c.GetModuleName(), line, text)
		}

		prompt = true
	} else {
		prompt = EvaluateBreakpoint(c)
	}

	for prompt {
		var tokens *tokenizer.Tokenizer

		for {
			cmd := getLine()
			if len(strings.TrimSpace(cmd)) == 0 {
				cmd = "step"
			}

			tokens = tokenizer.New(cmd)
			if !tokens.AtEnd() {
				break
			}
		}

		// We have a command now in the tokens buffer.
		if errors.Nil(err) {
			t := tokens.Peek(1)
			switch t {
			case "help":
				_ = Help()

			case tokenizer.GoToken, tokenizer.ContinueToken:
				c.SetSingleStep(false)

				prompt = false

			case "step":
				c.SetSingleStep(true)
				c.SetStepOver(false)

				prompt = false

				switch tokens.Peek(2) {
				case "over":
					c.SetStepOver(true)

				case "into":
					c.SetStepOver(false)

				case tokenizer.ReturnToken:
					c.SetBreakOnReturn()
					c.SetSingleStep(false)

				case tokenizer.EndOfTokens:
					// No action, this is the default step case

				default:
					prompt = true
					err = errors.New(errors.ErrInvalidStepType).Context(tokens.Peek(2))

					c.SetSingleStep(false)
				}

			case "show":
				err = Show(s, tokens, line, c)

			case "set":
				err = runAfterFirstToken(s, tokens, false)

			case tokenizer.CallToken:
				err = runAfterFirstToken(s, tokens, true)

			case tokenizer.PrintToken:
				text := "fmt.Println(" + strings.Replace(tokens.GetSource(), tokenizer.PrintToken, "", 1) + ")"
				t2 := tokenizer.New(text)

				traceMode := ui.LoggerIsActive(ui.TraceLogger)
				ui.SetLogger(ui.TraceLogger, false)

				err = compiler.Run("debugger", s, t2)
				if err.Is(errors.ErrStop) {
					err = nil
				}

				ui.SetLogger(ui.TraceLogger, traceMode)

			case tokenizer.BreakToken:
				err = Break(c, tokens)

			case tokenizer.ExitToken:
				return errors.New(errors.ErrStop)

			default:
				err = errors.New(errors.ErrInvalidDebugCommand).Context(t)
			}

			if !errors.Nil(err) && !err.Is(errors.ErrStop) && !err.Is(errors.ErrStepOver) {
				ui.Say("msg.debug.error", map[string]interface{}{
					"err": err,
				})

				err = nil
			}

			if err.Is(errors.ErrStop) {
				err = nil
				prompt = false
			}
		}
	}

	return err
}

func runAfterFirstToken(s *symbols.SymbolTable, t *tokenizer.Tokenizer, allowTrace bool) *errors.EgoError {
	verb := t.GetTokens(0, 1, false)
	text := strings.TrimPrefix(strings.TrimSpace(t.GetSource()), verb)
	t2 := tokenizer.New(text)

	traceMode := ui.LoggerIsActive(ui.TraceLogger)

	defer ui.SetLogger(ui.TraceLogger, traceMode)
	ui.SetLogger(ui.TraceLogger, allowTrace)

	err := compiler.Run("debugger", s, t2)
	if err.Is(errors.ErrStop) {
		err = nil
	}

	return err
}

// getLine reads a line of text from the console, and requires that it contain matching
// tick-quotes and braces.
func getLine() string {
	text := runtime.ReadConsoleText("debug> ")
	if len(strings.TrimSpace(text)) == 0 {
		return ""
	}

	t := tokenizer.New(text)

	for {
		braceCount := 0
		parenCount := 0
		bracketCount := 0
		openTick := false

		lastToken := t.Tokens[len(t.Tokens)-1]
		if lastToken[0:1] == "`" && lastToken[len(lastToken)-1:] != "`" {
			openTick = true
		}

		if !openTick {
			for _, v := range t.Tokens {
				switch v {
				case "[":
					bracketCount++

				case "]":
					bracketCount--

				case "(":
					parenCount++

				case ")":
					parenCount--

				case tokenizer.DataBeginToken:
					braceCount++

				case tokenizer.DataEndToken:
					braceCount--
				}
			}
		}

		if braceCount > 0 || parenCount > 0 || bracketCount > 0 || openTick {
			text = text + runtime.ReadConsoleText(".....> ")
			t = tokenizer.New(text)

			continue
		} else {
			break
		}
	}

	return text
}
