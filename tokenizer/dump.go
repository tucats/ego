package tokenizer

import (
	"fmt"
	"strings"
)

func (t *Tokenizer) DumpTokens(before, after int) {

	start := t.TokenP - before
	if start < 0 {
		start = 0
	}

	end := t.TokenP + after + 1
	if end >= len(t.Tokens) {
		end = len(t.Tokens)
	}

	tokens := strings.Builder{}
	pointer := strings.Builder{}

	for i := start; i < end; i++ {
		next := fmt.Sprintf("\"%v\" ", t.Tokens[i])
		tokens.WriteString(next)

		if i == t.TokenP {
			pointer.WriteString(" ^")
		} else {
			pointer.WriteString("  ")
		}
		for j := 3; j < len(next); j++ {
			pointer.WriteRune(' ')
		}
	}

	fmt.Printf("Token buffer: %s\n", tokens.String())
	fmt.Printf("               %s\n", pointer.String())
}
