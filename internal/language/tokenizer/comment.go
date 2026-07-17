package tokenizer

import "strings"

// Comment is a source comment captured during lexing. Comments are collected
// into a side list on the Tokenizer (see Tokenizer.Comments) rather than being
// placed in the main token stream, so that the compiler and every other token
// consumer see exactly the same token sequence as before comment capture was
// added. Tools that need comments — notably the source formatter — read them
// from the side list and correlate them with tokens by source position.
type Comment struct {
	// Text is the full comment text as it appeared in the source, including the
	// leading "//" or the surrounding "/*" and "*/".
	Text string

	// Line and Column are the 1-based source position of the comment's first
	// character, matching the position fields carried by Token.
	Line   int
	Column int

	// Block is true for a "/* ... */" comment and false for a "//" line comment.
	Block bool
}

// captureComment records a comment produced by the lexer. The raw text comes
// straight from the scanner; for line comments it may carry the synthetic line
// ending that splitLines appends to the source (see line.go), which is stripped
// here so the recorded text matches what the developer wrote.
//
// It returns true when the comment absorbed that synthetic ";" — i.e. a "//"
// line comment whose text ended in the appended lineEnding. In that case a
// statement terminator was lost to the comment, and the lexer re-emits a
// semicolon token so statements (and especially directives, which otherwise
// keep consuming tokens) after a trailing-comment line are terminated correctly.
func (t *Tokenizer) captureComment(text string, line, column int) bool {
	block := strings.HasPrefix(text, "/*")
	absorbedSemicolon := false

	if !block {
		// splitLines appends lineEnding ("  ;") to lines that do not end in a
		// continuation rune; for a line that ends in a "//" comment, that suffix
		// lands inside the comment text. Remove it (and any residual trailing
		// whitespace) so the comment reads as written.
		if strings.HasSuffix(text, lineEnding) {
			absorbedSemicolon = true
			text = strings.TrimSuffix(text, lineEnding)
		}

		text = strings.TrimRight(text, " \t")
	}

	t.Comments = append(t.Comments, Comment{
		Text:   text,
		Line:   line,
		Column: column,
		Block:  block,
	})

	return absorbedSemicolon
}
