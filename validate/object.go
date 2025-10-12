package validate

import "github.com/tucats/ego/errors"

func (o Object) Validate(item any) error {
	var err error

	value, ok := item.(map[string]any)
	if !ok {
		return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidType.Clone().Context(item))
	}

	// Verify that every name present exists, unless this object type allows undefined keys
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

		// See if the key value starts with "ego." or "EGO_" which means it must be an existing key
		// even if foreign keys are allowed. If the key wasn't found, but we allow foreign keys and
		// this key isn't a reserved Ego key, then it's considered valid.
		egoKey := len(key) > 4 && (key[0:4] == "ego." || key[0:4] == "EGO_")
		if !valid && !egoKey && o.AllowForeignKeys {
			valid = true
		}

		if !valid {
			return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidJSONKey.Clone().Context(key))
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
