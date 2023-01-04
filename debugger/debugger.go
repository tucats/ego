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
func Run(c *bytecode.Context) error {
	return RunFrom(c, 0)
}

func RunFrom(c *bytecode.Context, pc int) error {
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
	var err error

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
		if err == nil {
			t := tokens.Peek(1)
			switch t.Spelling() {
			case "help":
				_ = Help()

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
					err = errors.EgoError(errors.ErrInvalidStepType).Context(tokens.Peek(2))

					c.SetSingleStep(false)
				}

			case "show":
				err = Show(s, tokens, line, c)

			case "set":
				err = runAfterFirstToken(s, tokens, false)

			case "call":
				err = runAfterFirstToken(s, tokens, true)

			case "print":
				text := "fmt.Println(" + strings.Replace(tokens.GetSource(), "print", "", 1) + ")"
				t2 := tokenizer.New(text)

				traceMode := ui.IsActive(ui.TraceLogger)
				ui.SetLogger(ui.TraceLogger, false)

				err = compiler.Run("debugger", s, t2)
				if errors.Equals(err, errors.ErrStop) {
					err = nil
				}

				ui.SetLogger(ui.TraceLogger, traceMode)

			case "break":
				err = Break(c, tokens)

			case "exit":
				return errors.EgoError(errors.ErrStop)

			default:
				err = errors.EgoError(errors.ErrInvalidDebugCommand).Context(t)
			}

			if err != nil && !errors.Equals(err, errors.ErrStop) && !errors.Equals(err, errors.ErrStepOver) {
				ui.Say("msg.debug.error", map[string]interface{}{
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
	t2 := tokenizer.New(text)

	traceMode := ui.IsActive(ui.TraceLogger)

	defer ui.SetLogger(ui.TraceLogger, traceMode)
	ui.SetLogger(ui.TraceLogger, allowTrace)

	err := compiler.Run("debugger", s, t2)
	if errors.Equals(err, errors.ErrStop) {
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
			text = text + runtime.ReadConsoleText(".....> ")
			t = tokenizer.New(text)

			continue
		} else {
			break
		}
	}

	return text
}
