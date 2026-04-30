package data

import (
	"fmt"
	"strconv"
)

// Float64 returns the float64 value of the given data,
// or an error if the data cannot be converted to a float64.
func Float64(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	default:
		return strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
	}
}

// Float64orZero returns the float64 value of the given data,
// or 0 if the data cannot be converted to a float64.
func Float64OrZero(value any) float64 {
	f, err := Float64(value)
	if err != nil {
		return 0
	}

	return f
}
