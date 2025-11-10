package validate

import (
	"encoding/json"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/validator"
)

func Load(name string, data []byte) error {
	var (
		err    error
		result validator.Item
	)

	err = json.Unmarshal(data, &result)
	if err != nil {
		return errors.New(err)
	}

	Define(name, result)

	return nil
}

func LoadForeign(name string, data []byte) error {
	var (
		err    error
		result validator.Item
	)

	err = json.Unmarshal(data, &result)
	if err != nil {
		return errors.New(err)
	}

	DefineForeign(name, &result)

	return nil
}

func Validate(data []byte, kind string) error {
	spec := Lookup(kind)
	if spec == nil {
		return errors.ErrValidationError.Clone().Context(kind)
	}

	ui.Log(ui.ValidationsLogger, "validation.evaluate", ui.A{
		"name": kind})

	err := spec.Validate(string(data))
	if err != nil {
		ui.Log(ui.ValidationsLogger, "validation.failed", ui.A{
			"name":  kind,
			"error": err.Error()})
	}

	return err
}
