package data

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// AddressOf returns a pointer to a copy of the value held inside the given
// any interface.  It is the Ego equivalent of the Go unary & operator.
//
// The function uses a type switch to obtain the concrete value from the
// interface and then takes its address.  This is necessary because you
// cannot write "&v" when v is typed as any — that would give you a
// *any (pointer to the interface box) rather than a pointer to the
// underlying value.  By switching on the concrete type we get a properly
// typed pointer, e.g. *int or *float64.
//
// For Ego's own reference types (Array, Map, Struct, Channel) which are
// already pointers, AddressOf returns a pointer-to-pointer (**Array, etc.)
// so that assignment through the pointer replaces the reference itself
// rather than mutating the object.
//
// The default case handles any type not listed explicitly by falling back to
// a *any, which is safe but loses concrete type information.
func AddressOf(v any) (any, error) {
	if v == nil {
		return nil, errors.ErrNilPointerReference
	}

	switch actual := v.(type) {
	case bool:
		return &actual, nil
	case byte:
		return &actual, nil
	case int8:
		return &actual, nil
	case int16:
		return &actual, nil
	case uint16:
		return &actual, nil
	case uint32:
		return &actual, nil
	case uint:
		return &actual, nil
	case uint64:
		return &actual, nil
	case int32:
		return &actual, nil
	case int:
		return &actual, nil
	case int64:
		return &actual, nil
	case float32:
		return &actual, nil
	case float64:
		return &actual, nil
	case string:
		return &actual, nil
	case Package:
		return &actual, nil
	case *Struct:
		return &actual, nil
	case *Map:
		return &actual, nil
	case *Array:
		return &actual, nil
	case *Channel:
		return &actual, nil
	default:
		// Fall back to *any.  This is safe: the caller can still pass the
		// result to Dereference() to get the original value back.
		return &v, nil
	}
}

// Dereference follows a pointer and returns the value it points to.  It is
// the Ego equivalent of the Go unary * (dereference) operator.
//
// For each scalar pointer type (*int, *float64, etc.) we use the Go "*actual"
// dereference syntax to extract and return the value as a plain (non-pointer)
// concrete type.
//
// For Ego's own reference types the pointer indirection is one level deeper
// (**Map, **Array, **Channel) because AddressOf returns a pointer-to-pointer
// for those types.  Dereferencing **Map gives back the original *Map, which is
// exactly what the bytecode expects.
//
// If v is nil or not a pointer type that this function knows about, an error
// is returned.
func Dereference(v any) (any, error) {
	if v == nil {
		ui.Log(ui.InternalLogger, "runtime.ptr.nil.read", nil)

		return nil, errors.ErrNilPointerReference
	}

	switch actual := v.(type) {
	case *any:
		return *actual, nil
	case *bool:
		return *actual, nil
	case *byte:
		return *actual, nil
	case *int8:
		return *actual, nil
	case *int16:
		return *actual, nil
	case *uint16:
		return *actual, nil
	case *int32:
		return *actual, nil
	case *uint32:
		return *actual, nil
	case *uint:
		return *actual, nil
	case *uint64:
		return *actual, nil
	case *int:
		return *actual, nil
	case *int64:
		return *actual, nil
	case *float32:
		return *actual, nil
	case *float64:
		return *actual, nil
	case *string:
		return *actual, nil
	case *Package:
		return *actual, nil
	// The following three cases handle the double-pointer level created by
	// AddressOf for Ego reference types.
	case **Map:
		return *actual, nil
	case **Array:
		return *actual, nil
	case **Channel:
		return *actual, nil

	default:
		return nil, errors.ErrNotAPointer
	}
}
