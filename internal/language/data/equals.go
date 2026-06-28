package data

import "reflect"

// Equals compares two values presented as interfaces for equality and returns
// true if they are equal.
//
// It first tries reflect.DeepEqual, which handles all standard Go types
// correctly — including nested structs, slices, and maps.  "Deep" equality
// means that two structs with the same field values are equal even if they
// are stored at different memory addresses.
//
// If the first comparison does not match, the function uses a type-switch to
// unwrap both values from their any wrappers and tries again with the
// concrete underlying types.  This handles the common case where two values
// represent the same data but were stored in differently-typed interface
// variables.
func Equals(a any, b any) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}

	// The switch below looks unusual: the switch has no cases other than
	// "default".  Its purpose is purely to perform type assertion — Go's
	// type switch "switch x := y.(type)" binds the concrete type of y to x,
	// which we then pass back to reflect.DeepEqual with both values as their
	// unwrapped concrete types.  This gives DeepEqual one more chance to
	// compare them without any interface boxing in the way.
	switch actualA := a.(type) {
	default:
		switch actualB := b.(type) {
		default:
			return reflect.DeepEqual(actualA, actualB)
		}
	}
}
