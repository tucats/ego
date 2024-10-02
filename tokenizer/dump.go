package tokenizer

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
)

func (t *Tokenizer) DumpTokens() {
	if ui.IsActive(ui.TokenLogger) {
		ui.WriteLog(ui.TokenLogger, "Tokenizer contents:")

		for index, token := range t.Tokens {
			ui.WriteLog(ui.TokenLogger, "  [%2d:%2d] %v", t.Line[index], t.Pos[index], token)
		}
	}
}

func (t *Tokenizer) DumpTokenRange(before, after int) {
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
