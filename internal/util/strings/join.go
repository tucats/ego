package egostrings

import (
	"fmt"
	"strings"
)

// Join concatenates any number of values into a single string, with each
// value separated by separator.  It is similar to strings.Join but accepts
// values of any type rather than requiring a []string.
//
// If a value is itself a slice ([]string, []any, []int, []float64, []rune,
// or []bool), each element of the slice is appended to the result
// individually, as if the caller had passed them as separate arguments.
// This makes it easy to flatten a mixed list of scalars and slices without
// having to pre-process the arguments.
//
// All non-string values are converted to their default string representation
// using fmt.Sprintf with the appropriate verb:
//
//	[]any      → "%v"  (Go's generic "value" format)
//	[]int      → "%d"  (decimal integer)
//	[]float64  → "%f"  (decimal floating-point)
//	[]bool     → "%t"  (true / false)
//	[]rune     → the single Unicode character the rune represents
//	scalar     → "%v"  (default format for the concrete type)
func Join(separator string, values ...any) string {
	// result accumulates the individual string fragments before the final join.
	var result []string

	for _, value := range values {
		// A type switch inspects the runtime type of value and executes the
		// matching case.  "actual" is automatically typed to the matched
		// concrete type inside each case block.
		switch actual := value.(type) {
		case []string:
			// Already strings — append each one directly without formatting.
			for _, s := range actual {
				result = append(result, s)
			}

		case []any:
			// A slice of mixed types — use %v to get a reasonable string for
			// whatever concrete type each element happens to be.
			for _, v := range actual {
				result = append(result, fmt.Sprintf("%v", v))
			}

		case []int:
			// Format each integer as a decimal number ("%d").
			for _, i := range actual {
				result = append(result, fmt.Sprintf("%d", i))
			}

		case []float64:
			// Format each float as a decimal floating-point number ("%f").
			for _, f := range actual {
				result = append(result, fmt.Sprintf("%f", f))
			}

		case []rune:
			// A rune is a Unicode code point (an alias for int32).
			// string(r) converts a single rune to the UTF-8 string it represents.
			for _, r := range actual {
				result = append(result, string(r))
			}

		case []bool:
			// Format each bool as "true" or "false" using the %t verb.
			for _, b := range actual {
				result = append(result, fmt.Sprintf("%t", b))
			}

		default:
			// Scalar value (int, float64, bool, string, etc.) or any other type
			// — use %v for the default string representation.
			result = append(result, fmt.Sprintf("%v", actual))
		}
	}

	// strings.Join inserts separator between adjacent elements and returns the
	// concatenated string.  If result is empty, strings.Join returns "".
	return strings.Join(result, separator)
}
