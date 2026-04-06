package debugger

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
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
			sessionContext.line = c.GetLine()
			err = debuggerPrompt(c, sessionContext)
		}

		if !c.IsRunning() || errors.Equals(err, errors.ErrStop) {
			return nil
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
