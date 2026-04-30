package data

import "github.com/tucats/apitest/errors"

// Int returns the integer value of the given data,
// or an error if the data is not an integer.
func Int(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	default:
		return 0, errors.ErrInvalidInteger.Context(value)
	}
}

// IntOrZero returns the integer value of the given data,
// or 0 if the data cannot be converted to an integer.
func IntOrZero(value any) int {
	i, err := Int(value)
	if err != nil {
		return 0
	}

	return i
}
