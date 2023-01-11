package util

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
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
	parameters := u.Query()
	for name, values := range parameters {
		if typeString, ok := validation[name]; ok {
			switch strings.ToLower(typeString) {
			case FlagParameterType:
				if len(values) != 1 {
					return errors.ErrWrongParameterValueCount.Context(name)
				}

				if values[0] != "" {
					return errors.ErrWrongParameterValueCount.Context(name)
				}

			case IntParameterType:
				if len(values) != 1 {
					return errors.ErrWrongParameterValueCount.Context(name)
				}

				if _, ok := strconv.Atoi(data.String(values[0])); ok != nil {
					return errors.ErrInvalidInteger.Context(name)
				}

			case BoolParameterType:
				if len(values) > 1 {
					return errors.ErrWrongParameterValueCount.Context(name)
				}

				if len(values) == 1 && data.String(values[0]) != "" {
					if !InList(strings.ToLower(values[0]), defs.True, defs.False, "1", "0", "yes", "no") {
						return errors.ErrInvalidBooleanValue.Context(name)
					}
				}

			case defs.Any, StringParameterType:
				if len(values) != 1 {
					return errors.ErrWrongParameterValueCount.Context(name)
				}

			case ListParameterType:
				if len(values) == 0 || values[0] == "" {
					return errors.ErrWrongParameterValueCount.Context(name)
				}
			}
		} else {
			return errors.ErrInvalidKeyword.Context(name)
		}
	}

	return nil
}

// InList is a support function that checks to see if a string matches
// any of a list of other strings.
func InList(s string, test ...string) bool {
	for _, t := range test {
		if s == t {
			return true
		}
	}

	return false
}

// AcceptedMediaType validates the media type in the "Accept" header for this
// request against a list of valid media types. This includes common types that
// are always accepted, as well as additional types provided as paraameters to
// this function call.  The result is a nil error value if the media type is
// valid, else an error indicating that there was an invalid media type found.
func AcceptedMediaType(r *http.Request, validList []string) error {
	mediaTypes := r.Header["Accept"]

	for _, mediaType := range mediaTypes {
		// Check for common times that are always accepted.
		if InList(strings.ToLower(mediaType),
			"application/json",
			"application/text",
			"text/plain",
			"text/*",
			"text",
			"*/*",
		) {
			continue
		}

		// If not, verify that the media type is in the optional list of additional
		// accepted media types.
		if !InList(mediaType, validList...) {
			return errors.ErrInvalidMediaType.Context(mediaType)
		}
	}

	return nil
}
