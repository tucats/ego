package tokenizer

import (
	"strings"
)

// Reset line numbers. This is done after a prolog injeccted by the command processor,
// so errors reported during compilation or runtime reflect line numbers based on the
// @line specification rather than the actual literal line number of the source code.
func (t *Tokenizer) SetLineNumber(line int) error {
	if t.TokenP >= len(t.Line) {
		return nil
	}

	currentLine := t.Line[t.TokenP]

	offset := line - currentLine - 1
	if offset > len(t.Line) {
		return nil // nothing to do.
	}

	for i, n := range t.Line {
		newLine := n + offset
		if newLine < 0 {
			newLine = 0
		}

		t.Line[i] = newLine
	}

	t.Source = t.Source[currentLine:]

	return nil
}

// GetLine returns a given line of text from the token stream. This refers to the
// original line splits done when the  source was first received.
func (t *Tokenizer) GetLine(line int) string {
	if line < 1 || line > len(t.Source) {
		return ""
	}

	return t.Source[line-1]
}

// splitLines splits a string by line endings, and returns the source as an array of
// strings. If the isCode flag is set, the source lines have ";" added according to
// Go tokenization rules to add extra tokens to make command breaks clear. If the flag
// is false, no modifiecation to the code other than line splitting is done.
func splitLines(src string, isCode bool) []string {
	var result []string

	// Are we seeing Windows-style line endings? If so, use that as
	// the split boundary.
	if strings.Index(src, "\r\n") > 0 {
		result = strings.Split(src, "\r\n")
	} else {
		// Otherwise, simple split by new-line works fine.
		result = strings.Split(src, "\n")
	}

	// Look to see if we should add in semicolons in the Go style. We
	// do not add them if in the middle of a multi-line backtick-quoted
	// constant, or if the last rune is a "continuation" rune like a comma.
	if isCode {
		backTick := false

		for n, line := range result {
			text := strings.TrimSpace(line)
			lastChar := rune(0)

			for _, ch := range text {
				if ch == '`' {
					backTick = !backTick
				}

				lastChar = ch
			}

			if backTick {
				continue
			}

			found := false
			continuationRunes := []rune{
				rune(0),
				';',
				':',
				',',
				'.',
				'{',
				'`',
			}

			for _, t := range continuationRunes {
				if lastChar == t {
					found = true

					break
				}
			}

			if !found {
				result[n] = text + " ;"
			}
		}
	}

	return result
}

// GetSource returns the entire string of the tokenizer.
func (t *Tokenizer) GetSource() string {
	result := strings.Builder{}

	for _, line := range t.Source {
		result.WriteString(line)
		result.WriteRune('\n')
	}

	return result.String()
}

// Remainder returns the rest of the source from the current token position.
// This allows the caller to get "the rest" of a command line or other element
// as needed. If the token position is invalid (i.e. past end-of-tokens, for
// example) then an empty string is returned. This is typically used when the
// command line is processed using the tokenizer.
func (t *Tokenizer) Remainder() string {
	if t.TokenP < 0 || t.TokenP >= len(t.Pos) {
		return ""
	}

	p := t.Pos[t.TokenP] - 1
	s := t.GetSource()

	if p < 0 || p >= len(s) {
		return ""
	}

	return strings.TrimSuffix(s[p:], "\n")
}
