package functions

import (
	"encoding/json"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// JSONUnmarshal reads a string as JSON data.
func JSONUnmarshal(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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

		return v, err
	}

	// There's a model, so the return value should be an error code. IF we already
	// have had an error on the Unmarshal, we report it now.
	if err != nil {
		return errors.NewError(err), nil
	}

	// There is a model, so do some mapping if possible.
	pointer, ok := args[1].(*interface{})
	if !ok {
		return errors.ErrInvalidPointerType, nil
	}

	value := *pointer

	// Structure
	if target, ok := value.(*data.Struct); ok {
		if m, ok := v.(map[string]interface{}); ok {
			for k, v := range m {
				err = target.Set(k, v)
				if err != nil {
					return errors.NewError(err), nil
				}
			}
		} else {
			return errors.ErrInvalidType, nil
		}

		*pointer = target

		return nil, nil
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
					return errors.NewError(err), nil
				}
			}
		} else {
			return errors.ErrInvalidType, nil
		}

		*pointer = target

		return nil, nil
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
					return errors.NewError(err), nil
				}
			}
		} else {
			return errors.ErrInvalidType, nil
		}

		*pointer = target

		return nil, nil
	}

	if !data.TypeOf(v).IsType(data.TypeOf(value)) {
		err = errors.ErrInvalidType
		v = nil
	}

	*pointer = v

	if err != nil {
		err = errors.NewError(err)
	}

	return err, nil
}

func Seal(i interface{}) interface{} {
	switch actualValue := i.(type) {
	case *data.Struct:
		actualValue.SetStatic(true)

		return actualValue

	case *data.Array:
		for i := 0; i <= actualValue.Len(); i++ {
			element, _ := actualValue.Get(i)
			actualValue.SetAlways(i, Seal(element))
		}

		return actualValue

	default:
		return actualValue
	}
}

// JSONMarshal writes a JSON string from arbitrary data.
func JSONMarshal(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 1 {
		jsonBuffer, err := json.Marshal(data.Sanitize(args[0]))
		if err != nil {
			err = errors.NewError(err)
		}

		return data.NewArray(data.ByteType, 0).Append(jsonBuffer), err
	}

	var b strings.Builder

	b.WriteString("[")

	for n, v := range args {
		if n > 0 {
			b.WriteString(", ")
		}

		jsonBuffer, err := json.Marshal(data.Sanitize(v))
		if err != nil {
			return nil, errors.NewError(err)
		}

		b.WriteString(string(jsonBuffer))
	}

	b.WriteString("]")
	jsonBuffer := []byte(b.String())

	return data.NewArray(data.ByteType, 0).Append(jsonBuffer), nil
}

// JSONMarshalIndent writes a  JSON string from arbitrary data.
func JSONMarshalIndent(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	prefix := data.String(args[1])
	indent := data.String(args[2])

	jsonBuffer, err := json.MarshalIndent(data.Sanitize(args[0]), prefix, indent)
	if err != nil {
		err = errors.NewError(err)
	}

	return data.NewArray(data.ByteType, 0).Append(jsonBuffer), err
}
