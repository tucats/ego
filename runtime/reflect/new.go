package reflect

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// instanceOf implements the reflect.instanceOf() function. This
// function creates a new "zero value" of any given type or object.
// If an integer type number or a string type name is given, the
// "zero value" for that type is returned. For an array, struct,
// or map, a recursive copy is done of the members to a new object
// which is returned.
func instanceOf(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	// Is the type an integer? If so it's a type kind from the native
	// reflection package.
	if typeValue, ok := args.Get(0).(int); ok {
		switch reflect.Kind(typeValue) {
		case reflect.Uint8, reflect.Int8:
			return byte(0), nil

		case reflect.Int32:
			return int32(0), nil

		case reflect.Int, reflect.Int64:
			return 0, nil

		case reflect.String:
			return "", nil

		case reflect.Bool:
			return false, nil

		case reflect.Float32:
			return float32(0), nil

		case reflect.Float64:
			return float64(0), nil

		default:
			return nil, errors.ErrInvalidType.In("New").Context(typeValue)
		}
	}

	// Is it an actual type?
	if typeValue, ok := args.Get(0).(*data.Type); ok {
		return typeValue.InstanceOf(typeValue), nil
	}

	// Is the type a string? If so it's a built-in scalar type name
	if typeValue, ok := args.Get(0).(string); ok {
		switch strings.ToLower(typeValue) {
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
			return nil, errors.ErrInvalidType.In("New").Context(typeValue)
		}
	}

	// If it's a channel, just return the value
	if typeValue, ok := args.Get(0).(*data.Channel); ok {
		return typeValue, nil
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
	r := recursiveCopy(args.Get(0), MaxDeepCopyDepth)

	// If there was a user-defined type in the source, make the clone point back to it
	switch v := r.(type) {
	case nil:
		return nil, errors.ErrInvalidValue.In("New").Context(nil)

	case symbols.SymbolTable:
		return nil, errors.ErrInvalidValue.In("New").Context("symbol table")

	case func(*symbols.SymbolTable, []interface{}) (interface{}, error):
		return v, nil

	// No action for this group
	case byte, int32, int, int64, string, float32, float64:

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
		return nil, errors.ErrInvalidType.In("New").Context(v)
	}

	return r, nil
}
