package data

import (
	"fmt"
	"strconv"
)

// Bool returns the boolean value of the given data, or
// an error if the data cannot be converted to a boolean.
func Bool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	default:
		return strconv.ParseBool(fmt.Sprintf("%v", v))
	}
}
