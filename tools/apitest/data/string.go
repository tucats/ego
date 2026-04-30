package data

import (
	"fmt"
)

// String returns the string value of the given data.
func String(value any) string {
	return fmt.Sprintf("%v", value)
}
