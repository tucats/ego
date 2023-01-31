package data

import "fmt"

type Immutable struct {
	Value interface{}
}

func Constant(v interface{}) Immutable {
	if i, ok := v.(Immutable); ok {
		v = i.Value
	}

	return Immutable{Value: v}
}

func UnwrapConstant(i interface{}) interface{} {
	if v, ok := i.(Immutable); ok {
		return v.Value
	}

	return i
}

// String generates a human-readable string describing the value
// in the immutable wrapper.
func (w Immutable) String() string {
	return fmt.Sprintf("%s <read only>", Format(w.Value))
}
