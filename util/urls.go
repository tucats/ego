package util

import (
	"net/url"
	"strings"
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
)

// Parameter type strings used as values in the validation map passed to
// ValidateParameters. Each constant names the expected type of a URL query
// parameter and controls which validation logic is applied to it.
const (
	// FlagParameterType expects the parameter to appear with no value (e.g. ?verbose).
	FlagParameterType = "flag"

	// BoolParameterType expects a single value that represents a boolean:
	// true/false, 1/0, or yes/no (case-insensitive).
	BoolParameterType = "bool"

	// IntParameterType expects a single value that is a valid integer string.
	IntParameterType = "int"

	// StringParameterType expects exactly one (possibly empty) string value.
	StringParameterType = "string"

	// StringOrFlagParameterType accepts either a string value or a bare flag
	// (no value). When no value is present an empty string is assumed.
	StringOrFlagParameterType = "string|flag"

	// ListParameterType expects one or more comma-separated values; the slice
	// must be non-empty and the first element must not be the empty string.
	ListParameterType = "list"

	// DurationParameterType expects a single value parseable by time.ParseDuration
	// (e.g. "5m", "1h30m"). An absent or empty value is also accepted.
	DurationParameterType = "duration"
)

// ValidateParameters checks the parameters in a previously-parsed URL against a map
// describing the expected parameters and types. If there is no error, the function
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

			case StringOrFlagParameterType:
				// No validation; if there isn't a value we assume an empty
				// string.

			case ListParameterType:
				if err = validateListParameter(values, name); err != nil {
					return err
				}

			case DurationParameterType:
				if err = validateDurationParameter(values, name); err != nil {
					return err
				}
			}
		} else {
			return errors.ErrInvalidKeyword.Context(name)
		}
	}

	return err
}

// validateListParameter checks that a "list" parameter has at least one
// non-empty value. Lists are typically comma-separated strings passed as
// a single query parameter value; the caller is responsible for splitting
// the value — this function only confirms that something was supplied.
func validateListParameter(values []string, name string) error {
	if len(values) == 0 || values[0] == "" {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	return nil
}

// validateStringParameter checks that a "string" parameter appears exactly
// once. Duplicate query parameters with the same name (e.g. ?x=a&x=b) are
// rejected because the caller expects a single unambiguous value.
func validateStringParameter(values []string, name string) error {
	if len(values) != 1 {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	return nil
}

// validateDurationParameter checks that a "duration" parameter, when present,
// contains a value parseable by time.ParseDuration (e.g. "5m", "1h30m").
// The parameter may appear at most once; an absent or empty value is valid
// and means "use the default duration".
func validateDurationParameter(values []string, name string) error {
	if len(values) > 1 {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	if len(values) == 1 && data.String(values[0]) != "" {
		if _, err := time.ParseDuration(data.String(values[0])); err != nil {
			return errors.ErrInvalidDuration.Context(name)
		}
	}

	return nil
}

// validateBoolParameter checks that a "bool" parameter, when present, contains
// one of the recognized boolean strings: true/false, 1/0, or yes/no
// (case-insensitive). The parameter may appear at most once; an absent or
// empty value is valid and means the parameter was omitted by the caller.
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

// validateIntParameter checks that an "int" parameter appears exactly once and
// that its value can be parsed as an integer by egostrings.Atoi.
func validateIntParameter(values []string, name string) error {
	if len(values) != 1 {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	if _, ok := egostrings.Atoi(data.String(values[0])); ok != nil {
		return errors.ErrInvalidInteger.Context(name)
	}

	return nil
}

// validateFlagParameter checks that a "flag" parameter appears with no value
// (i.e. the URL contains ?name= or just ?name with an empty string). Flags
// are boolean presence indicators — a non-empty value is unexpected and is
// treated as an error.
func validateFlagParameter(values []string, name string) error {
	if len(values) != 1 {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	if values[0] != "" {
		return errors.ErrWrongParameterValueCount.Context(name)
	}

	return nil
}
