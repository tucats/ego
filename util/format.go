package util

import (
	"github.com/tucats/ego/datatypes"
)

// LineColumnFormat describes the format string for the portion
// of formatted messages that include a line and column designation.
const LineColumnFormat = "at %d:%d"

// LineFormat describes the format string for a message that contains
// just a line number.
const LineFormat = "at %d"

// FormatUnquoted formats a value but does not
// put quotes on strings.
func FormatUnquoted(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v

	default:
		return Format(v)
	}
}

// Format converts the given object into a string representation.
// In particular, this varies from a simple "%v" format in Go because
// it puts commas in the array list output to match the syntax of an
// array constant and puts quotes around string values.
func Format(arg interface{}) string {
	return datatypes.Format(arg)
}
