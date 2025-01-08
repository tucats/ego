package util

import (
	"net/url"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
)

const (
	FlagParameterType   = "flag"
	BoolParameterType   = "bool"
	IntParameterType    = "int"
	StringParameterType = "string"
	ListParameterType   = "list"
)

// ValidateParameters checks the parameters in a previously-parsed URL against a map
// describing the expected parameters and types. IF there is no error, the function
// returns nil, else an error describing the first parameter found that was invalid.
func ValidateParameters(u *url.URL, validation map[string]string) error {
	var err error

	parameters := u.Query()
	for name, values := range parameters {
		if typeString, ok := validation[name]; ok {
			switch strings.ToLower(typeString) {
			case FlagParameterType:
				if err = validateFlagParameter(values, name); err != nil {
					return err
				}

			case IntParameterType:
				if err = validateIntParameter(values, name); err != nil {
					return err
				}

			case BoolParameterType:
				if err = validateBoolParameter(values, name); err != nil {
					return err
				}

			case defs.Any, StringParameterType:
				if err = validateStringParameter(values, name); err != nil {
					return err
				}

			case ListParameterType:
				if err = validateListParameter(values, name); err != nil {
					return err
				}
			}
		} else {
			return errors.ErrInvalidKeyword.Context(name)
		}
	}

	return err
}

func validateListParameter(values []string, name string) error {
	if len(values) == 0 || values[0] == "" {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	return nil
}

func validateStringParameter(values []string, name string) error {
	if len(values) != 1 {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	return nil
}

func validateBoolParameter(values []string, name string) error {
	if len(values) > 1 {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	if len(values) == 1 && data.String(values[0]) != "" {
		if !InList(strings.ToLower(values[0]), defs.True, defs.False, "1", "0", "yes", "no") {
			return errors.ErrInvalidBooleanValue.Context(name)
		}
	}

	return nil
}

func validateIntParameter(values []string, name string) error {
	if len(values) != 1 {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	if _, ok := egostrings.Atoi(data.String(values[0])); ok != nil {
		return errors.ErrInvalidInteger.Context(name)
	}

	return nil
}

func validateFlagParameter(values []string, name string) error {
	if len(values) != 1 {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	if values[0] != "" {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	return nil
}
