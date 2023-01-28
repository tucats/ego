package reflect

import (
	"reflect"
	"strings"
	"sync"

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
//
// @tomcole This is the same as the "$new" internal function. Look for ways to
// consolidate these in the future.
func instanceOf(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Is the type an integer? If so it's a type kind from the native
	// reflection package.
	if typeValue, ok := args[0].(int); ok {
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
			return nil, errors.ErrInvalidType.In("new()").Context(typeValue)
		}
	}

	// Is it an actual type?
	if typeValue, ok := args[0].(*data.Type); ok {
		return typeValue.InstanceOf(typeValue), nil
	}

	// Is the type a string? If so it's a bult-in scalar type name
	if typeValue, ok := args[0].(string); ok {
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
			return nil, errors.ErrInvalidType.In("new()").Context(typeValue)
		}
	}

	// If it's a WaitGroup, make a new one. Note, have to use the switch statement
	// form here to prevent Go from complaining that the interface{} is being copied.
	// In reality, we don't care as we don't actually make a copy anyway but instead
	// make a new waitgroup object.
	switch args[0].(type) {
	case sync.WaitGroup:
		return data.InstanceOfType(data.WaitGroupType), nil
	}

	// If it's a Mutex, make a new one. We hae to do this as a swtich on the type, since a
	// cast attempt will yield a warning on invalid mutex copy operation.
	switch args[0].(type) {
	case *sync.Mutex:
		return data.InstanceOfType(data.MutexType), nil
	}

	// If it's a channel, just return the value
	if typeValue, ok := args[0].(*data.Channel); ok {
		return typeValue, nil
	}

	// Some native complex types work using the data package deep
	// copy operation on that type.

	switch actual := args[0].(type) {
	case *data.Struct:
		return data.DeepCopy(actual), nil

	case *data.Array:
		return data.DeepCopy(actual), nil

	case *data.Map:
		return data.DeepCopy(actual), nil
	}

	// Otherwise, make a deep copy of the item ourselves.
	r := recursiveCopy(args[0], MaxDeepCopyDepth)

	// If there was a user-defined type in the source, make the clone point back to it
	switch v := r.(type) {
	case nil:
		return nil, errors.ErrInvalidValue.In("new()").Context(nil)

	case symbols.SymbolTable:
		return nil, errors.ErrInvalidValue.In("new()").Context("symbol table")

	case func(*symbols.SymbolTable, []interface{}) (interface{}, error):
		return v, nil

	// No action for this group
	case byte, int32, int, int64, string, float32, float64:

	case *data.Package:
		// Create the replica count if needed, and update it.
		replica := 0

		if replicaX, ok := data.GetMetadata(v, data.ReplicaMDKey); ok {
			replica = data.Int(replicaX) + 1
		}

		data.SetMetadata(v, data.ReplicaMDKey, replica)

		dropList := []string{}

		// Organize the new item by removing things that are handled via the parent.
		keys := v.Keys()
		for _, k := range keys {
			vv, _ := v.Get(k)
			// IF it's an internal function, we don't want to copy it; it can be found via the
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
		return nil, errors.ErrInvalidType.In("new()").Context(v)
	}

	return r, nil
}
