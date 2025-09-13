package validate

import "github.com/tucats/ego/errors"

func (o Object) Validate(item any) error {
	var err error

	value, ok := item.(map[string]any)
	if !ok {
		return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidType.Clone().Context(item))
	}

	// Verify that every name present exists
	for key, data := range value {
		valid := false

		for _, field := range o.Fields {
			if key == field.Name {
				if err := field.Validate(data); err != nil {
					return err
				}

				valid = true

				break
			}
		}

		if !valid {
			return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidField.Clone().Context(key))
		}
	}

	// Verify that the required field names do exist
	for _, field := range o.Fields {
		if _, found := value[field.Name]; !found && field.Required {
			return errors.ErrValidationError.Clone().Chain(errors.ErrMissingField.Clone().Context(field.Name))
		}
	}

	return err
}
