package tokenizer

import "github.com/tucats/ego/errors"

// Reset line numbers. This is done after a prolog that the user
// might not be aware of is injected, so errors reported during
// compilation or runtime reflect line numbers based on the
// @line specification rather than the actual literal line number.
func (t *Tokenizer) SetLineNumber(line int) *errors.EgoError {
	currentLine := t.Line[t.TokenP]
	offset := line - currentLine - 1

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
