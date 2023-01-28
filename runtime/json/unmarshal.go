package json

import (
	"encoding/json"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Unmarshal reads a string as JSON data.
func Unmarshal(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var v interface{}

	var err error

	// Simplest case, []byte input. Otherwise, treat the argument
	// as a string.
	if a, ok := args[0].(*data.Array); ok && a.ValueType().Kind() == data.ByteKind {
		err = json.Unmarshal(a.GetBytes(), &v)
	} else {
		jsonBuffer := data.String(args[0])
		err = json.Unmarshal([]byte(jsonBuffer), &v)
	}

	// If there is no model, assume a generic return value is okay
	if len(args) < 2 {
		// Hang on, if the result is a map, then Ego won't be able to use it,
		// so convert that to an EgoMap. Same for an array.
		if m, ok := v.(map[string]interface{}); ok {
			v = data.NewMapFromMap(m)
		} else if a, ok := v.([]interface{}); ok {
			v = data.NewArrayFromArray(data.InterfaceType, a)
		}

		if err != nil {
			err = errors.NewError(err)
		}

		return data.List(v, err), err
	}

	// There's a model, so the return value should be an error code. IF we already
	// have had an error on the Unmarshal, we report it now.
	if err != nil {
		return data.List(errors.NewError(err)), nil
	}

	// There is a model, so do some mapping if possible.
	pointer, ok := args[1].(*interface{})
	if !ok {
		return data.List(errors.ErrInvalidPointerType), nil
	}

	value := *pointer

	// Structure
	if target, ok := value.(*data.Struct); ok {
		if m, ok := v.(map[string]interface{}); ok {
			for k, v := range m {
				err = target.Set(k, v)
				if err != nil {
					return data.List(errors.NewError(err)), nil
				}
			}
		} else {
			return data.List(errors.ErrInvalidType), nil
		}

		*pointer = target

		return data.List(nil), nil
	}

	// Map
	if target, ok := value.(*data.Map); ok {
		if m, ok := v.(map[string]interface{}); ok {
			for k, v := range m {
				k2 := data.Coerce(k, data.InstanceOfType(target.KeyType()))
				v2 := v

				if !target.ValueType().IsInterface() {
					v2 = data.Coerce(v, data.InstanceOfType(target.ValueType()))
				}

				_, err = target.Set(k2, v2)
				if err != nil {
					return data.List(errors.NewError(err)), nil
				}
			}
		} else {
			return data.List(errors.ErrInvalidType), nil
		}

		*pointer = target

		return data.List(nil), nil
	}

	// Array
	if target, ok := value.(*data.Array); ok {
		if m, ok := v.([]interface{}); ok {
			// The target data size may be wrong, fix it
			target.SetSize(len(m))

			for k, v := range m {
				if target.ValueType().Kind() == data.StructKind {
					if mm, ok := v.(map[string]interface{}); ok {
						v = data.NewStructOfTypeFromMap(target.ValueType(), mm)
					}
				}

				err = target.Set(k, v)
				if err != nil {
					return data.List(errors.NewError(err)), nil
				}
			}
		} else {
			return data.List(errors.ErrInvalidType), nil
		}

		*pointer = target

		return data.List(nil), nil
	}

	if !data.TypeOf(v).IsType(data.TypeOf(value)) {
		err = errors.ErrInvalidType
		v = nil
	}

	*pointer = v

	if err != nil {
		err = errors.NewError(err)
	}

	return data.List(err), err
}
