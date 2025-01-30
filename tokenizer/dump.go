package tokenizer

import (
	"github.com/tucats/ego/app-cli/ui"
)

// DumpTokens dumps the current tokens and their positions in the source code to the
// Token logger. This is useful for debugging purposes, and requires that the TokenLogger
// be activated using the -l TOKENIZER command line flag. The tokens are only dumped if the
// compiler determines there was a compilation error.
func (t *Tokenizer) DumpTokens() {
	if ui.IsActive(ui.TokenLogger) {
		ui.WriteLog(ui.TokenLogger, "tokens.dump", nil)

		for index, token := range t.Tokens {
			ui.WriteLog(ui.TokenLogger, "tokens.token", ui.A{
				"line":  t.Line[index],
				"pos":   t.Pos[index],
				"token": token})
		}
	}
}
