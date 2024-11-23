package tokenizer

// Mark returns the current token position. This is used to "remember" where the
// current token was found, in case the compiler needs to backtrack and try to
// process a token stream a different way. This is used to support "lexical recovery"
// when the compiler tries to determine the symantic meaning of the next set of tokens.
func (t *Tokenizer) Mark() int {
	return t.TokenP
}

// Set sets the next token to the given marker. The marker was previouisly obtained
// by calling Mark().
func (t *Tokenizer) Set(mark int) {
	if mark < 0 {
		mark = 0
	} else if mark > len(t.Tokens) {
		mark = len(t.Tokens)
	}

	t.TokenP = mark
}

// Reset sets the tokenizer back to the start of the token stream. This is the same as
// having called Mark() before processing any tokens, and using that to Set() the current
// position back to the start.
func (t *Tokenizer) Reset() {
	t.TokenP = 0
}
