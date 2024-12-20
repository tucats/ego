package debugger

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// How many spaces to indent source code formatting.
const indent = 3

func showSource(tx *tokenizer.Tokenizer, tokens *tokenizer.Tokenizer, err error) {
	start := 1
	end := len(tx.Source)
	nesting := 0

	tokens.Advance(2)

	if tokens.Peek(1) != tokenizer.EndOfTokens {
		var e2 error
		start, e2 = strconv.Atoi(tokens.NextText())
		_ = tokens.IsNext(tokenizer.ColonToken)

		if e2 == nil && tokens.Peek(1) != tokenizer.EndOfTokens {
			end, e2 = strconv.Atoi(tokens.NextText())
		}

		if e2 != nil {
			err = errors.New(errors.ErrInvalidInteger)
		}
	}

	if err == nil {
		for i, t := range tx.Source {
			if i < start-1 || i > end-1 {
				continue
			}

			// Simplistic formatting. Strip the trailing line break token,
			// and then indent the string based on the nexting of braces.
			t = strings.TrimSpace(strings.TrimSuffix(t, ";"))

			opened := strings.Count(t, "{") + strings.Count(t, "(")
			closed := strings.Count(t, "}") + strings.Count(t, ")")

			if opened > closed {
				t = strings.Repeat(" ", nesting*indent) + t
				nesting++
			} else if closed > opened {
				nesting--
				if nesting < 0 {
					nesting = 0
				}

				t = strings.Repeat(" ", nesting*indent) + t
			} else {
				t = strings.Repeat(" ", nesting*indent) + t
			}

			fmt.Printf("%-5d %s\n", i+1, t)
		}
	}
}
