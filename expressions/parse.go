package expressions

import (
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// Parse parses a text expression.
func (e *Expression) Parse(s string) *errors.EgoError {
	e.t = tokenizer.New(s)

	return nil
}
