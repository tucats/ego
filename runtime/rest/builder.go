package rest

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/ego/data"
)

// The object that knows how to semantically add values to a URL to
// forma a valid string. Note that this does not have to be a valid
// complete URL, but may consist of just the path and parameters.
type urlString struct {
	buffer         strings.Builder
	parameterCount int
}

// URLBuilder creates a new instance of a URLBuilder, and optionally
// populates it with the arguments provided. The parts are treated as
// a path name and optional path arguments. If no initial parts are
// provided in the call, then the URLString starts as an empty string.
//
// You can pass an initial part with format operators in it, or it can
// contain {{name}}-style replacements, which become "%v" operators in
// the foramt string. This allows the same string expression used to
// define an endpoint route to be used to form the endpoint expression.
func URLBuilder(initialParts ...interface{}) *urlString {
	url := &urlString{}

	if len(initialParts) > 0 {
		format := data.String(initialParts[0])

		for strings.Contains(format, "{{") {
			start := strings.Index(format, "{{")
			end := strings.Index(format, "}}")
			format = format[:start] + "%v" + format[end+2:]
		}

		if len(initialParts) == 1 {
			url.buffer.WriteString(format)
		} else {
			url.Path(format, initialParts[1:]...)
		}
	}

	return url
}

// Path adds path eleemnts to the URL being constructed. The format string
// contains the literal text, and can contain standard Go format operators like
// %s or %d. The array of parts items is read to fill in the format operators in
// the format string. Any remaining items in the parts array are treated as
// URL parameter values to add to the URL.
func (u *urlString) Path(format string, parts ...interface{}) *urlString {
	substitutions := strings.Count(format, "%")

	subs := make([]interface{}, substitutions)
	copy(subs, parts[:substitutions])

	u.buffer.WriteString(fmt.Sprintf(format, subs...))

	if len(parts) > substitutions {
		for _, part := range parts[substitutions:] {
			u.Parameter(fmt.Sprintf("%v", part))
		}
	}

	return u
}

// Parameter adds a parameter to the URL being constructed. The name string
// contains the parameter name. This is added to the URL being built. The arguments
// are optional additional arguments which follow the parameter value if specified.
func (u *urlString) Parameter(name string, arguments ...interface{}) *urlString {
	if u.parameterCount == 0 {
		u.buffer.WriteRune('?')
	} else {
		u.buffer.WriteRune('&')
	}

	u.buffer.WriteString(url.QueryEscape(name))
	
	u.parameterCount++

	if len(arguments) > 0 {
		for n, argument := range arguments {
			if n == 0 {
				u.buffer.WriteRune('=')
			} else {
				u.buffer.WriteRune(',')
			}

			u.buffer.WriteString(url.QueryEscape(fmt.Sprintf("%v", argument)))
		}
	}

	return u
}

// String converts the URL that was constructed to a string value.
func (u *urlString) String() string {
	return u.buffer.String()
}
