package debugger

import (
	"fmt"
	"strconv"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// Show implements the debugger's show command. This can be used to display information
// about the state of the running program or it's runtime environment.
func Show(s *symbols.SymbolTable, tokens *tokenizer.Tokenizer, line int, c *bytecode.Context) *errors.EgoError {
	var err *errors.EgoError

	t := tokens.Peek(2)
	tx := c.GetTokenizer()

	switch t {
	case "breaks", "breakpoints":
		ShowBreaks()

	case "symbols", "syms":
		fmt.Println(s.Format(true))

	case "line":
		text := tx.GetLine(line)

		fmt.Printf("%s:\n\t%5d, %s\n", stepTo, line, text)

	case "frames", "calls":
		depth := -1

		tx := tokens.Peek(3)
		if tx != tokenizer.EndOfTokens {
			if tx != "all" {
				var e2 error
				depth, e2 = strconv.Atoi(tx)
				err = errors.New(e2)
			}
		}

		if errors.Nil(err) {
			fmt.Print(c.FormatFrames(depth))
		}

	case "scope":
		syms := s
		depth := 0

		fmt.Printf("Symbol table scope:\n")

		for syms != nil {
			idx := "local"

			if depth > 0 {
				idx = fmt.Sprintf("%5d", depth)
			}

			fmt.Printf("\t%s:  %s, %d symbols\n", idx, syms.Name, len(syms.Symbols))

			depth++

			syms = syms.Parent
		}

	case "source":
		start := 1
		end := len(tx.Source)

		tokens.Advance(2)

		if tokens.Peek(1) != tokenizer.EndOfTokens {
			var e2 error
			start, e2 = strconv.Atoi(tokens.Next())
			_ = tokens.IsNext(":")

			if e2 == nil && tokens.Peek(1) != tokenizer.EndOfTokens {
				end, e2 = strconv.Atoi(tokens.Next())
			}

			err = errors.New(e2)
		}

		if errors.Nil(err) {
			for i, t := range tx.Source {
				if i < start-1 || i > end-1 {
					continue
				}

				fmt.Printf("%-5d %s\n", i+1, t)
			}
		}

	default:
		err = errors.New(errors.InvalidDebugCommandError).Context("show " + t)
	}

	return err
}
