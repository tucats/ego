package tokenizer

import "github.com/tucats/ego/errors"

func (t *Tokenizer) Delete(start, end int) error {
	// IF the delete range is out of range, return an error.
	if start < 0 || start >= len(t.Tokens) || end < start || end > len(t.Tokens) {
		return errors.ErrArrayIndex
	}

	// Build a new token buffer from the old tokens up to the start position,
	// plus the tokens after the end position.
	result := make([]Token, 0, len(t.Tokens)-end+start)

	for _, token := range t.Tokens[:start] {
		result = append(result, token)
	}

	result = append(result, t.Tokens[end:]...)
	t.Tokens = result

	if t.TokenP >= start && t.TokenP <= end {
		t.TokenP = start
	} else if t.TokenP > end {
		t.TokenP -= end - start
	}

	return nil
}
func (t *Tokenizer) Insert(pos int, tokens ...Token) error {
	// IF the insert position is out of range, return an error.
	if pos < 0 || pos >= len(t.Tokens) {
		return errors.ErrArrayIndex
	}

	// IF there are no tokens, nothing to do.
	if len(tokens) == 0 {
		return nil
	}

	// Build a new token buffer from the old tokens up to the insert position,
	// plus the new tokens. Then add the rest of the old tokens after the new ones.
	result := make([]Token, 0, len(t.Tokens)+len(tokens))

	for _, token := range t.Tokens[:pos] {
		result = append(result, token)
	}

	result = append(result, tokens...)
	result = append(result, t.Tokens[pos:]...)

	t.Tokens = result

	return nil
}
