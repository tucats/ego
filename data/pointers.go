package data

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// AddressOf will generate an interface{} object that
// is a pointer to a given value. It unwraps the interface
// type provided, and returns the address of that unwrapped
// object. This is used to get the Go-native address of an
// object to read or write it.
func AddressOf(v interface{}) (interface{}, error) {
	if v == nil {
		return nil, errors.ErrNilPointerReference
	}

	switch actual := v.(type) {
	case bool:
		return &actual, nil
	case byte:
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
		return &v, nil
	}
}

// Dereference returns the actual value that is pointed to by
// the passed-in value. This can be a pointer to any object,
// and it returns the actual object itself. This can be used
// to get the value of an item generated by AddressOf(), and
// supports bytecode instructions that manipulate pointer
// values.
func Dereference(v interface{}) (interface{}, error) {
	if v == nil {
		ui.Log(ui.InternalLogger, "runtime.ptr.nil.read")

		return nil, errors.ErrNilPointerReference
	}

	switch actual := v.(type) {
	case *interface{}:
		return *actual, nil
	case *bool:
		return *actual, nil
	case *byte:
		return *actual, nil
	case *int32:
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
