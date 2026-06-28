package data

// Interface is an Ego-level wrapper for a value that was declared with the
// interface{} (or "any") type.  In Ego, unlike plain Go, even an untyped
// interface value carries its concrete type alongside the value so that the
// Ego runtime can always answer "what type is this?" without resorting to
// Go reflection.
//
// BaseType records the Ego type of Value.  If the Interface was constructed
// around a nil value, BaseType is left nil to signal "uninitialized".
type Interface struct {
	Value    any   // the wrapped value
	BaseType *Type // the Ego type of Value; nil means the interface holds nothing
}

// Wrap creates a new Interface object around the given value, automatically
// recording its Ego type in BaseType.
//
// If value is nil, BaseType is left nil rather than calling TypeOf(nil),
// because a nil interface has no meaningful type.
func Wrap(value any) any {
	result := Interface{
		Value: value,
	}

	// TypeOf inspects the concrete Go type of value and returns the
	// corresponding *Type from the data package's type system.
	if value != nil {
		result.BaseType = TypeOf(value)
	}

	return result
}

// UnWrap unwraps an Interface object, returning the underlying value and its
// associated type.  If value is not an Interface (for example, it is a plain
// int or string), the function returns the value unchanged and a nil type to
// signal that no Interface wrapper was present.
func UnWrap(value any) (any, *Type) {
	// The "comma-ok" type assertion "v, ok := value.(Interface)" checks
	// whether value holds an Interface without panicking if it does not.
	if v, ok := value.(Interface); ok {
		if v.BaseType == nil {
			// Interface was constructed around nil — signal "empty interface".
			return nil, nil
		}

		return v.Value, v.BaseType
	}

	// Not an Interface wrapper at all — return as-is with no type info.
	return value, nil
}
