package validate

import (
	"encoding/json"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

func Load(name string, data []byte) error {
	var (
		err    error
		result Object
	)

	err = json.Unmarshal(data, &result)
	if err != nil {
		return errors.New(err)
	}

	Define(name, result)

	return nil
}

func Validate(data []byte, kind string) error {
	spec := Lookup(kind)
	if spec == nil {
		return errors.ErrValidationError.Clone().Context(kind)
	}

	ui.Log(ui.ValidationsLogger, "validation.evaluate", ui.A{
		"name": kind})

	err := validateWithSpec(data, spec)
	if err != nil {
		ui.Log(ui.ValidationsLogger, "validation.failed", ui.A{
			"name":  kind,
			"error": err.Error()})
	}

	return err
}

func validateWithSpec(data []byte, spec any) error {
	var (
		err   error
		value any
	)

	// Convert the JSON data to an any object
	err = json.Unmarshal(data, &value)
	if err != nil {
		return errors.ErrValidationError.Clone().Chain(errors.New(err))
	}

	return validate(value, spec)
}

func validate(value, abstract any) error {
	switch item := value.(type) {
	case map[string]any:
		if spec, ok := abstract.(Object); ok {
			return spec.Validate(item)
		}

		return errors.ErrValidationError.Clone().Context(abstract)

	case []any:
		if spec, ok := abstract.(Array); ok {
			return spec.Validate(item)
		}

		return errors.ErrValidationError.Clone().Context(abstract)

	default:
		if spec, ok := abstract.(Item); ok {
			return spec.Validate(item)
		}

		return errors.ErrValidationError.Clone().Context(abstract)
	}
}
