package data

import "fmt"

// Immutable is the type that describes an immutable (i.e. readonly)
// value. The value itself can be anything. When loading this value
// from storage for use on the stack in a computation, it is unwrapped
// and the contained value used instead. Attempts to store data to an
// object of type Immutable will result in a "read-only value" error
// being generated in the Ego code.
type Immutable struct {
	Value any
}

// Wrap the value in an Immutable object, unless it is already an
// immutable object. This is used to convert a value before storage.
func Constant(v any) Immutable {
	if i, ok := v.(Immutable); ok {
		v = i.Value
	}

	return Immutable{Value: v}
}

// Unwrap an Immutable object and return the value it contains.
// If the object is not an Immutable object, no action is done and
// it just returns the interface value.
func UnwrapConstant(i any) any {
	if v, ok := i.(Immutable); ok {
		return v.Value
	}

	return i
}

// String generates a human-readable string describing the value
// in the immutable wrapper.
func (w Immutable) String() string {
	return fmt.Sprintf("^%s", Format(w.Value))
}
