package debugger

import (
	"fmt"
	"strconv"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// showCommand implements the debugger's show command. This can be used to display information
// about the state of the running program or it's runtime environment.
func showCommand(s *symbols.SymbolTable, tokens *tokenizer.Tokenizer, line int, c *bytecode.Context) error {
	var err error

	t := tokens.Peek(2)
	tx := c.GetTokenizer()

	switch t.Spelling() {
	case "breaks", "breakpoints":
		showBreaks()

	case "symbols":
		if tokens.Peek(3) != tokenizer.EndOfTokens {
			return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(3))
		}

		fmt.Println(s.Format(true))

	case "line":
		if tokens.Peek(3) != tokenizer.EndOfTokens {
			return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(3))
		}

		text := tx.GetLine(line)

		fmt.Printf("%s:\n\t%5d, %s\n", stepTo, line, text)

	case "frames", "calls":
		depth := -1

		tx := tokens.Peek(3)
		if tx != tokenizer.EndOfTokens {
			if tokens.Peek(4) != tokenizer.EndOfTokens {
				return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(4))
			}

			if tx.Spelling() != "all" {
				var e2 error

				depth, e2 = strconv.Atoi(tx.Spelling())
				if e2 != nil {
					err = errors.NewError(e2)
				}
			}
		}

		if err == nil {
			fmt.Print(c.FormatFrames(depth))
		}

	case "scope":
		symbolTable := s
		depth := 0

		if tokens.Peek(3) != tokenizer.EndOfTokens {
			return errors.ErrUnexpectedTextAfterCommand.Context(tokens.Peek(3))
		}

		ui.Say("msg.debug.scope")

		for symbolTable != nil {
			idx := "local"

			if depth > 0 {
				idx = fmt.Sprintf("%5d", depth)
			}

			fmt.Printf("\t%s:  %s, %d %s\n", idx, symbolTable.Name, symbolTable.Size(), i18n.L("symbols"))

			depth++

			symbolTable = symbolTable.Parent()
		}

	case "source":
		start := 1
		end := len(tx.Source)

		tokens.Advance(2)

		if tokens.Peek(1) != tokenizer.EndOfTokens {
			var e2 error
			start, e2 = strconv.Atoi(tokens.NextText())
			_ = tokens.IsNext(tokenizer.ColonToken)

			if e2 == nil && tokens.Peek(1) != tokenizer.EndOfTokens {
				end, e2 = strconv.Atoi(tokens.NextText())
			}

			if e2 != nil {
				err = errors.NewError(e2)
			}
		}

		if err == nil {
			for i, t := range tx.Source {
				if i < start-1 || i > end-1 {
					continue
				}

				fmt.Printf("%-5d %s\n", i+1, t)
			}
		}

	default:
		err = errors.ErrInvalidDebugCommand.Context("show " + t.Spelling())
	}

	return err
}
