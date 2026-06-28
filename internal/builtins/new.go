package builtins

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// NewInstanceOf implements the $new() function. This function creates a new
// "zero value" of any given type or object. If an integer type
// number or a string type name is given, the "zero value" for
// that type is returned. For an array, struct, or map, a recursive
// copy is done of the members to a new object which is returned.
func NewInstanceOf(s *symbols.SymbolTable, args data.List) (any, error) {
	// Is the type an integer? If so it's a type kind from the native
	// reflection package.
	if typeValue, ok := args.Get(0).(int); ok {
		return newReflectKind(reflect.Kind(typeValue))
	}

	// Is it a pointer to an actual type?
	if typeValue, ok := args.Get(0).(*data.Type); ok {
		return typeValue.InstanceOf(typeValue), nil
	}

	// Is the type a string? If so it's a built-in scalar type name
	if typeValue, ok := args.Get(0).(string); ok {
		return newTypeName(typeValue)
	}

	// If it's a channel, create a NEW independent channel with the same capacity.
	// BUILTIN-NEW-2 fix: the original code returned the existing channel unchanged,
	// meaning $new(ch) and ch aliased the same underlying channel.  We now use
	// Cap() to read the buffer size and construct a fresh channel.
	if typeValue, ok := args.Get(0).(*data.Channel); ok {
		return data.NewChannel(typeValue.Cap()), nil
	}

	// Some native complex types work using the data package deep
	// copy operation on that type.

	switch actual := args.Get(0).(type) {
	case *data.Struct:
		return data.DeepCopy(actual), nil

	case *data.Array:
		return data.DeepCopy(actual), nil

	case *data.Map:
		return data.DeepCopy(actual), nil
	}

	// Otherwise, make a deep copy of the item ourselves.
	r := DeepCopy(args.Get(0), MaxDeepCopyDepth)

	// If there was a user-defined type in the source, make the clone point back to it
	switch v := r.(type) {
	case nil:
		return nil, errors.ErrInvalidValue.In("new").Context(nil)

	case symbols.SymbolTable:
		return nil, errors.ErrInvalidValue.In("new").Context("symbol table")

	case func(*symbols.SymbolTable, []any) (any, error):
		return v, nil

	// No action for this group
	case byte, int32, int8, int16, uint16, int, int64, string, float32, float64:

	case *data.Package:
		dropList := []string{}

		// Organize the new item by removing things that are handled via the parent.
		keys := v.Keys()
		for _, k := range keys {
			vv, _ := v.Get(k)
			// If it's an internal function, we don't want to copy it; it can be found via the
			// __parent link to the type
			vx := reflect.ValueOf(vv)

			if vx.Kind() == reflect.Ptr {
				ts := vx.String()
				if ts == defs.ByteCodeReflectionTypeString {
					dropList = append(dropList, k)
				}
			} else {
				if vx.Kind() == reflect.Func {
					dropList = append(dropList, k)
				}
			}
		}

		for _, name := range dropList {
			v.Delete(name)
		}

	default:
		return nil, errors.ErrInvalidType.In("new").Context(v)
	}

	return r, nil
}

// Helper function to generate a "zero value" based on the native
// reflection type values. If the kind is unsupported (not one of
// the base types Ego uses) then an error is returned.
func newReflectKind(kind reflect.Kind) (any, error) {
	switch kind {
	case reflect.Uint8:
		return byte(0), nil

	case reflect.Int8:
		return int8(0), nil

	case reflect.Int16:
		return int16(0), nil

	case reflect.Uint16:
		return uint16(0), nil

	case reflect.Int32:
		return int32(0), nil

	case reflect.Int:
		return 0, nil

	// BUILTIN-NEW-1 fix: reflect.Int64 previously shared a case with reflect.Int,
	// causing an untyped integer literal 0 to be returned — Go infers that as int,
	// not int64.  Callers expecting int64 received the wrong concrete type.
	// The two cases are now separate so each returns the correct zero value.
	case reflect.Int64:
		return int64(0), nil

	case reflect.String:
		return "", nil

	case reflect.Bool:
		return false, nil

	case reflect.Float32:
		return float32(0), nil

	case reflect.Float64:
		return float64(0), nil

	default:
		return nil, errors.ErrInvalidType.In("new").Context(kind)
	}
}

// Helper function to generate a "zero value" based on a string
// containing a valid base type name. If the type is unsupported
// (not one of the base types Ego uses) then an error is returned.
func newTypeName(kind string) (any, error) {
	switch strings.ToLower(kind) {
	case data.BoolType.Name():
		return false, nil

	case data.ByteType.Name():
		return byte(0), nil

	case data.Int32TypeName:
		return int32(0), nil

	case data.IntTypeName:
		return 0, nil

	case data.Int64TypeName:
		return int64(0), nil

	case data.StringTypeName:
		return "", nil

	case data.Float32TypeName:
		return float32(0), nil

	case data.Float64TypeName:
		return float64(0), nil

	default:
		return nil, errors.ErrInvalidType.In("new").Context(kind)
	}
}
