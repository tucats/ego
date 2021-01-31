package debugger

import (
	"fmt"
	"strconv"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

func Show(s *symbols.SymbolTable, tokens *tokenizer.Tokenizer, line int, c *bytecode.Context) error {
	t := tokens.Peek(2)
	tx := c.GetTokenizer()

	var err error
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
				depth, err = strconv.Atoi(tx)
			}
		}
		if err == nil {
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
			depth++
			fmt.Printf("\t%s:  %s, %d symbols\n", idx, syms.Name, len(syms.Symbols))
			syms = syms.Parent
		}

	case "source":
		start := 1
		end := len(tx.Source)
		tokens.Advance(2)
		if tokens.Peek(1) != tokenizer.EndOfTokens {
			start, err = strconv.Atoi(tokens.Next())
			_ = tokens.IsNext(":")
			if err == nil && tokens.Peek(1) != tokenizer.EndOfTokens {
				end, err = strconv.Atoi(tokens.Next())

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
		err = fmt.Errorf("unreognized show command: %s", t)
	}

	return err
}
