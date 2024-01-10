package data

import "reflect"

// Equals compares two values presented as interfaces for equality.
func Equals(a interface{}, b interface{}) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}

	// If they weren't an obvious match, unwrap them based on type and
	// try again to be sure.
	switch actualA := a.(type) {
	default:
		switch actualB := b.(type) {
		default:
			return reflect.DeepEqual(actualA, actualB)
		}
	}
}
