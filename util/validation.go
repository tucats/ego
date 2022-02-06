package util

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// ValidateParameters checks the parameters in a previously-parsed URL against a map
// describing the expected parameters and types. IF there is no error, the function
// returns nil, else an error describing the first parameter found that was invalid.
func ValidateParameters(u *url.URL, validation map[string]string) *errors.EgoError {
	parameters := u.Query()
	for name, values := range parameters {
		if typeString, ok := validation[name]; ok {
			switch strings.ToLower(typeString) {
			case "flag":
				if len(values) != 1 {
					return errors.New(errors.ErrWrongParameterValueCount).Context(name)
				}

				if values[0] != "" {
					return errors.New(errors.ErrWrongParameterValueCount).Context(name)
				}

			case "int":
				if len(values) != 1 {
					return errors.New(errors.ErrWrongParameterValueCount).Context(name)
				}

				if _, ok := strconv.Atoi(datatypes.GetString(values[0])); ok != nil {
					return errors.New(errors.ErrInvalidInteger).Context(name)
				}

			case "bool":
				if len(values) > 1 {
					return errors.New(errors.ErrWrongParameterValueCount).Context(name)
				}

				if len(values) == 1 && datatypes.GetString(values[0]) != "" {
					if !InList(strings.ToLower(values[0]), defs.True, defs.False, "1", "0", "yes", "no") {
						return errors.New(errors.ErrInvalidBooleanValue).Context(name)
					}
				}

			case defs.Any, "string":
				if len(values) != 1 {
					return errors.New(errors.ErrWrongParameterValueCount).Context(name)
				}

			case "list":
				if len(values) == 0 || values[0] == "" {
					return errors.New(errors.ErrWrongParameterValueCount).Context(name)
				}
			}
		} else {
			return errors.New(errors.ErrInvalidKeyword).Context(name)
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
func AcceptedMediaType(r *http.Request, validList []string) *errors.EgoError {
	mediaTypes := r.Header["Accept"]

	for _, mediaType := range mediaTypes {
		// Check for common times that are always accepted.
		if InList(strings.ToLower(mediaType),
			"application/json",
			"application/text",
			"text/plain",
			"text/*",
			"text",
		) {
			continue
		}

		// If not, verify that the media type is in the optional list of additional
		// accepted media types.
		if !InList(mediaType, validList...) {
			return errors.New(errors.ErrInvalidMediaType).Context(mediaType)
		}
	}

	return nil
}
