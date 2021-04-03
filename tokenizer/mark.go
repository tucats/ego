package tokenizer

// Mark returns a location marker in the token stream.
func (t *Tokenizer) Mark() int {
	return t.TokenP
}

// Set sets the next token to the given marker.
func (t *Tokenizer) Set(mark int) {
	if mark < 0 {
		mark = 0
	} else if mark > len(t.Tokens) {
		mark = len(t.Tokens)
	}

	t.TokenP = mark
}

// Reset sets the tokenizer back to the start of the token stream.
func (t *Tokenizer) Reset() {
	t.TokenP = 0
}
