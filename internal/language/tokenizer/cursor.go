package tokenizer

// Next gets the next token in the tokenizer, and advances the "current" position.
// If there are no more tokens, it returns EndOfTokens.
func (t *Tokenizer) Next() Token {
	if t.TokenP >= len(t.Tokens) {
		return EndOfTokens
	}

	token := t.Tokens[t.TokenP]
	t.TokenP++

	return token
}

// NextText gets the next token in the tokenizer and returns it's
// text value as a string.
func (t *Tokenizer) NextText() string {
	if t.TokenP >= len(t.Tokens) {
		return EndOfTokens.spelling
	}

	token := t.Tokens[t.TokenP]
	t.TokenP++

	return token.spelling
}

// Peek looks in the token queue relative to the current position
// without advancing the pointer. The offset can be negative to look
// behind the current position, or positive to look ahead. If the
// offset is out of bounds, it returns EndOfTokens.
func (t *Tokenizer) Peek(offset int) Token {
	position := t.TokenP + (offset - 1)
	if position >= len(t.Tokens) || position < 0 {
		return EndOfTokens
	}

	return t.Tokens[position]
}

// PeekText looks in the token queue relative to the current position
// without advancing the pointer, and returns the text spelling of
// the specified token. The offset can be negative to look behind the
// current position, or positive to look ahead. If the offset is out
// of bounds, it returns EndOfTokens spelling, which is an empty string.
func (t *Tokenizer) PeekText(offset int) string {
	pos := t.TokenP + (offset - 1)
	if pos < 0 || pos >= len(t.Tokens) {
		return EndOfTokens.spelling
	}

	return t.Tokens[pos].spelling
}

// AtEnd indicates if the current token position is at the end of the token
// queue. If the current position is at the end of the token queue, it returns true.
func (t *Tokenizer) AtEnd() bool {
	return t.TokenP >= len(t.Tokens)
}

// Advance moves the pointer in the token queue by the given offset. The offset can
// be positive or negative. If the offset is out of bounds, the current position is
// set to either the beginning or the end of the token queue depending on the sign
// of the offset.
func (t *Tokenizer) Advance(p int) {
	t.TokenP = t.TokenP + p
	if t.TokenP < 0 {
		t.TokenP = 0
	} else if t.TokenP > len(t.Tokens) {
		t.TokenP = len(t.Tokens)
	}
}

// IsNext tests to see if the next token is the given token, and if so
// advances and returns true, else does not advance and returns false.
func (t *Tokenizer) IsNext(test Token) bool {
	if t.Peek(1).Is(test) {
		t.Advance(1)

		return true
	}

	return false
}

// EndOfStatement reports whether the tokenizer is at a logical statement
// boundary — either because all tokens have been consumed, or because the
// next token is a semicolon. The Ego lexer inserts synthetic semicolons at
// line breaks (following the same rules as the Go specification), so this
// function correctly handles both explicit ";" and implicit line-ending
// statement terminators.
func (t *Tokenizer) EndOfStatement() bool {
	if t.AtEnd() {
		return true
	}

	token := t.Peek(1)
	if token.Is(SemicolonToken) {
		return true
	}

	return false
}

// AnyNext tests to see if the next token is in the given list
// of tokens, and if so  advances and returns true, else does not
// advance and returns false.
func (t *Tokenizer) AnyNext(test ...Token) bool {
	n := t.Peek(1)
	for _, v := range test {
		if n.Is(v) {
			t.Advance(1)

			return true
		}
	}

	return false
}

// CurrentLine returns the 1-based source line number of the token that will
// be read by the next call to Next() or Peek(1). It returns 0 when the
// tokenizer is positioned before the first token or past the last token,
// which can happen after the token stream has been fully consumed or before
// any tokens have been read. This value is used to attach accurate line
// numbers to compiler error messages.
func (t *Tokenizer) CurrentLine() int {
	if t.TokenP == 0 || t.TokenP >= len(t.Tokens) {
		return 0
	}

	return int(t.Tokens[t.TokenP].line)
}

// CurrentColumn returns the 1-based column position within the source line of
// the token that will be read next. Like CurrentLine, it returns 0 when the
// position is before the first token or past the last token. Together,
// CurrentLine and CurrentColumn let the compiler report the exact location
// of a syntax error in the user's source file.
func (t *Tokenizer) CurrentColumn() int {
	if t.TokenP == 0 || t.TokenP >= len(t.Tokens) {
		return 0
	}

	return int(t.Tokens[t.TokenP].pos)
}
