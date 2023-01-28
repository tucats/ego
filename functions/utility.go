package functions

import (
	"math"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// ProfileGet implements the profile.get() function.
func ProfileGet(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	key := data.String(args[0])

	return settings.Get(key), nil
}

// ProfileSet implements the profile.set() function.
func ProfileSet(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var err error

	if len(args) != 2 {
		return nil, errors.ErrArgumentCount.In("Set()")
	}

	key := data.String(args[0])
	isEgoSetting := strings.HasPrefix(key, "ego.")

	// Quick check here. The key must already exist if it's one of the
	// "system" settings. That is, you can't create an ego.* setting that
	// doesn't exist yet, for example
	if isEgoSetting {
		if !settings.Exists(key) {
			return nil, errors.ErrReservedProfileSetting.In("Set()").Context(key)
		}
	}

	// Additionally, we don't allow anyone to change runtime, compiler, or server settings from Ego code

	mode := "interactive"
	if modeValue, found := symbols.Get(defs.ModeVariable); found {
		mode = data.String(modeValue)
	}

	if mode != "test" &&
		(strings.HasPrefix(key, "ego.runtime") ||
			strings.HasPrefix(key, "ego.server") ||
			strings.HasPrefix(key, "ego.compiler")) {
		return nil, errors.ErrReservedProfileSetting.In("Set()").Context(key)
	}

	// If the value is an empty string, delete the key else
	// store the value for the key.
	value := data.String(args[1])
	if value == "" {
		err = settings.Delete(key)
	} else {
		settings.Set(key, value)
	}

	// Ego settings can only be updated in the in-memory copy, not in the persisted data.
	if isEgoSetting {
		return err, nil
	}

	// Otherwise, store the value back to the file system.
	return err, settings.Save()
}

// ProfileDelete implements the profile.delete() function. This just calls
// the set operation with an empty value, which results in a delete operatinon.
// The consolidates the persmission checking, etc. in the Set routine only.
func ProfileDelete(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return ProfileSet(symbols, []interface{}{args[0], ""})
}

// ProfileKeys implements the profile.keys() function.
func ProfileKeys(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	keys := settings.Keys()
	result := make([]interface{}, len(keys))

	for i, key := range keys {
		result[i] = key
	}

	return data.NewArrayFromArray(data.StringType, result), nil
}

// Length implements the len() function.
func Length(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return 0, nil
	}

	switch arg := args[0].(type) {
	// For a channel, it's length either zero if it's drained, or bottomless
	case *data.Channel:
		size := int(math.MaxInt32)
		if arg.IsEmpty() {
			size = 0
		}

		return size, nil

	case *data.Array:
		return arg.Len(), nil

	case error:
		return len(arg.Error()), nil

	case *data.Map:
		return len(arg.Keys()), nil

	case *data.Package:
		return nil, errors.ErrInvalidType.Context(data.TypeOf(arg).String())

	case nil:
		return 0, nil

	default:
		v := data.Coerce(args[0], "")
		if v == nil {
			return 0, nil
		}

		return len(v.(string)), nil
	}
}


// Signal creates an error object based on the
// parameters.
func Signal(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	r := errors.ErrUserDefined
	if len(args) > 0 {
		r = r.Context(args[0])
	}

	return r, nil
}

// SizeOf returns the size in bytes of an arbibrary object.
func SizeOf(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	size := data.RealSizeOf(args[0])

	return size, nil
}

// Append implements the builtin append() function, which concatenates all the items
// together as an array. The first argument is flattened into the result, and then each
// additional argument is added to the array as-is.
func Append(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	result := make([]interface{}, 0)
	kind := data.InterfaceType

	for i, j := range args {
		if array, ok := j.(*data.Array); ok && i == 0 {
			if !kind.IsInterface() {
				if err := array.Validate(kind); err != nil {
					return nil, err
				}
			}

			result = append(result, array.BaseArray()...)

			if kind.IsInterface() {
				kind = array.ValueType()
			}
		} else if array, ok := j.([]interface{}); ok && i == 0 {
			result = append(result, array...)
		} else {
			if !kind.IsInterface() && !data.TypeOf(j).IsType(kind) {
				return nil, errors.ErrWrongArrayValueType.In("append()")
			}
			result = append(result, j)
		}
	}

	return data.NewArrayFromArray(kind, result), nil
}

// Delete can be used three ways. To delete a member from a structure, to delete
// an element from an array by index number, or to delete a symbol entirely. The
// first form requires a string name, the second form requires an integer index,
// and the third form does not have a second parameter.
func Delete(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if _, ok := args[0].(string); ok {
		if len(args) != 1 {
			return nil, errors.ErrArgumentCount.In("delete{}")
		}
	} else {
		if len(args) != 2 {
			return nil, errors.ErrArgumentCount.In("delete{}")
		}
	}

	switch v := args[0].(type) {
	case string:
		return nil, s.Delete(v, false)

	case *data.Map:
		_, err := v.Delete(args[1])

		return v, err

	case *data.Array:
		i := data.Int(args[1])
		err := v.Delete(i)

		return v, err

	default:
		return nil, errors.ErrInvalidType.In("delete()")
	}
}

// Make implements the make() function. The first argument must be a model of the
// array type (using the Go native version), and the second argument is the size.
func Make(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	kind := args[0]
	size := data.Int(args[1])

	// if it's an Ego type, get the model for the type.
	if v, ok := kind.(*data.Type); ok {
		kind = data.InstanceOfType(v)
	} else if egoArray, ok := kind.(*data.Array); ok {
		return egoArray.Make(size), nil
	}

	array := make([]interface{}, size)

	if v, ok := kind.([]interface{}); ok {
		if len(v) > 0 {
			kind = v[0]
		}
	}

	// If the model is a type we know about, let's go ahead and populate the array
	// with specific values.
	switch v := kind.(type) {
	case *data.Channel:
		return data.NewChannel(size), nil

	case *data.Array:
		return v.Make(size), nil

	case []int, int:
		for i := range array {
			array[i] = 0
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
	}

	return array, nil
}
