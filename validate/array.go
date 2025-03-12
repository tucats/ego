package validate

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

func (a Array) Validate(item interface{}) error {
	var err error

	array, ok := item.([]interface{})
	if !ok {
		return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidType.Clone().Context(item))
	}

	minSize, _ := data.Int(a.Min)
	if minSize > 0 && len(array) < minSize {
		return errors.ErrValidationError.Clone().Chain(errors.ErrArraySize.Clone().Context(len(array)))
	}

	maxSize, _ := data.Int(a.Max)
	if maxSize > 0 && len(array) > maxSize {
		return errors.ErrValidationError.Clone().Chain(errors.ErrArraySize.Clone().Context(len(array)))
	}

	for _, data := range array {
		if err := a.Type.Validate(data); err != nil {
			return err
		}
	}

	return err
}
