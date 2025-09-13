package data

type Interface struct {
	Value    any
	BaseType *Type
}

// Wrap creates a new Interface object around the given value. If the
// value passed in is a nil, then the interface is left incomplete.
func Wrap(value any) any {
	result := Interface{
		Value: value,
	}

	if value != nil {
		result.BaseType = TypeOf(value)
	}

	return result
}

// Unwrap unwraps an Interface object, and returns the underlying value
// and the associated type. If the interface was never initialized or is
// not actually an interface, the result is a nil type.
func UnWrap(value any) (any, *Type) {
	if v, ok := value.(Interface); ok {
		if v.BaseType == nil {
			return nil, nil
		}

		return v.Value, v.BaseType
	}

	return value, nil
}
