package egostrings

import (
	"fmt"
	"strings"
)

// Join functions just like strings.Joint but accepts an slice of any type, and formats
// the value as a string value before joining them together.
func Join(separator string, values ...any) string {
	var result []string

	// Iterate over the arguments and format them as strings. If they are arrays,
	// join each member of the array as string values. If they are not arrays, add
	// them directly as string values to the result array.
	for _, value := range values {
		switch actual := value.(type) {
		case []string:
			for _, s := range actual {
				result = append(result, s)
			}

		case []any:
			for _, v := range actual {
				result = append(result, fmt.Sprintf("%v", v))
			}

		case []int:
			for _, i := range actual {
				result = append(result, fmt.Sprintf("%d", i))
			}

		case []float64:
			for _, f := range actual {
				result = append(result, fmt.Sprintf("%f", f))
			}

		case []rune:
			for _, r := range actual {
				result = append(result, string(r))
			}

		case []bool:
			for _, b := range actual {
				result = append(result, fmt.Sprintf("%t", b))
			}

		default:
			result = append(result, fmt.Sprintf("%v", actual))
		}
	}

	// Return the array of formatted string values joined together with the specified separator.
	return strings.Join(result, separator)
}
