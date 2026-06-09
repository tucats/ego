package builtins

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Make implements the make() function. The first argument must be a model of the
// array type (using the Go native version), and the second argument is the size.
func Make(s *symbols.SymbolTable, args data.List) (any, error) {
	kind := args.Get(0)

	size, err := data.Int(args.Get(1))
	if err != nil {
		return nil, errors.New(err).In("make")
	}

	// BUILTIN-MAKE-1 fix: a negative size is passed directly to Go's make(),
	// which panics with "makeslice: len out of range" — an unrecoverable error
	// that bypasses Ego's try/catch mechanism.  Reject it explicitly here.
	if size < 0 {
		return nil, errors.ErrInvalidValue.In("make").Context(size)
	}

	// If it's an Ego type descriptor, resolve it to a concrete model value so
	// the type-switch below can dispatch on the underlying Go type.
	if v, ok := kind.(*data.Type); ok {
		kind = data.InstanceOfType(v)
	} else if egoArray, ok := kind.(*data.Array); ok {
		// A *data.Array model delegates directly to the array's own Make method,
		// which creates a correctly-typed Ego array rather than a raw []any slice.
		return egoArray.Make(size), nil
	}

	array := make([]any, size)

	// If kind is already a []any (e.g. from InstanceOfType resolving to a slice),
	// unwrap it to get the element model.
	if v, ok := kind.([]any); ok {
		if len(v) > 0 {
			kind = v[0]
		}
	}

	// Populate the array with the correct zero-value for the element type.
	// BUILTIN-MAKE-2 fix: the original code only handled int, bool, string, and
	// float64.  Additional numeric widths (int64, int32, float32) now have their
	// own cases so make() does not silently return a nil-filled []any for those
	// types.  The default case now returns an explicit error for any type that
	// is not a recognised element kind.
	switch v := kind.(type) {
	case *data.Channel:
		// For a channel model, create a fresh channel of the requested size.
		return data.NewChannel(size), nil

	case *data.Array:
		// For a typed array model, delegate to the array's Make method.
		return v.Make(size), nil

	case []int, int:
		for i := range array {
			array[i] = 0
		}

	case int64:
		for i := range array {
			array[i] = int64(0)
		}

	case int32:
		for i := range array {
			array[i] = int32(0)
		}

	case int16:
		for i := range array {
			array[i] = int16(0)
		}

	case int8:
		for i := range array {
			array[i] = int8(0)
		}

	case byte: // byte is an alias for uint8
		for i := range array {
			array[i] = byte(0)
		}

	case []bool, bool:
		for i := range array {
			array[i] = false
		}

	case []string, string:
		for i := range array {
			array[i] = ""
		}

	case []float64, float64:
		for i := range array {
			array[i] = 0.0
		}

	case float32:
		for i := range array {
			array[i] = float32(0)
		}

	default:
		// An unrecognised element type.  Returning a nil-filled slice silently
		// (the old behaviour) makes bugs very hard to diagnose — callers receive
		// valid-looking data that just happens to contain only nil elements.
		// Return an explicit error instead so the caller knows the type is not
		// supported by make().
		return nil, errors.ErrInvalidType.In("make").Context(data.TypeOf(v).String())
	}

	return array, nil
}

func New(s *symbols.SymbolTable, args data.List) (any, error) {
	if t, ok := args.Get(0).(*data.Type); ok {
		vx := data.InstanceOfType(t)

		tmp := data.GenerateName()
		s.SetAlways(tmp, vx)

		addr, ok := s.GetAddress(tmp)
		if !ok {
			return nil, errors.ErrPanic.Context("failed to create new &" + t.String() + " instance")
		}

		return addr, nil
	}

	return nil, errors.ErrNotAType.Context(data.String(args.Get(0)))
}
