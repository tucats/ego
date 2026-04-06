package debugger

import (
	"fmt"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// showCommand implements the debugger's show command. This can be used to display information
// about the state of the running program or its runtime environment. All output is written
// through sessionContext.
func showCommand(s *symbols.SymbolTable, tokens *tokenizer.Tokenizer, line int, c *bytecode.Context, sessionContext *session) error {
	var (
		err error
		t   = tokens.Peek(2)
		tx  = c.GetTokenizer()
	)

	switch t.Spelling() {
	case "breaks", "breakpoints":
		showBreaks(sessionContext)

	case "symbols":
		if tokens.Peek(3).IsNot(tokenizer.EndOfTokens) {
			return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(3))
		}

		sessionContext.println(s.Format(true))

	case "line":
		if tokens.Peek(3).IsNot(tokenizer.EndOfTokens) {
			return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(3))
		}

		text := tx.GetLine(line)
		sessionContext.printf("%s:\n\t%5d, %s\n", stepTo, line, text)

	case "frames", "calls":
		depth := bytecode.ShowAllCallFrames

		tx := tokens.Peek(3)
		if tx.IsNot(tokenizer.EndOfTokens) {
			if tokens.Peek(4).IsNot(tokenizer.EndOfTokens) {
				return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(4))
			}

			if tx.Spelling() != "all" {
				var e2 error

				depth, e2 = egostrings.Atoi(tx.Spelling())
				if e2 != nil {
					err = errors.ErrInvalidInteger
				}
			}
		}

		if err == nil {
			sessionContext.printf("%s", c.FormatFrames(depth))
		}

	case "scope":
		symbolTable := s
		depth := 0

		if tokens.Peek(3).IsNot(tokenizer.EndOfTokens) {
			return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(3))
		}

		sessionContext.say("msg.debug.scope")

		for symbolTable != nil {
			idx := "local"

			if depth > 0 {
				idx = fmt.Sprintf("%5d", depth)
			}

			sessionContext.printf("\t%s:  %s, %d %s\n", idx, symbolTable.Name, symbolTable.Size(), i18n.L("symbols"))

			depth++
			symbolTable = symbolTable.Parent()
		}

	case "source":
		showSource(tx, tokens, err, sessionContext)

	default:
		err = errors.ErrInvalidDebugCommand.Context("show " + t.Spelling())
	}

	return err
}
