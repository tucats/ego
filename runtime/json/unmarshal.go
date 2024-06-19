package json

import (
	"encoding/json"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// unmarshal reads a byte array or string as JSON data.
func unmarshal(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		v   interface{}
		err error
	)

	// Simplest case, []byte input. Otherwise, treat the argument
	// as a string.
	if a, ok := args.Get(0).(*data.Array); ok && a.Type().Kind() == data.ByteKind {
		err = json.Unmarshal(a.GetBytes(), &v)
	} else {
		jsonBuffer := data.String(args.Get(0))
		err = json.Unmarshal([]byte(jsonBuffer), &v)
	}

	if err != nil {
		err = errors.New(err).In("Unmarshal")

		return data.NewList(nil, err), err
	}

	// If there is no model, assume a generic return value is okay
	if args.Len() < 2 {
		// Hang on, if the result is a map, then Ego won't be able to use it,
		// so convert that to an EgoMap. Same for an array.
		if m, ok := v.(map[string]interface{}); ok {
			v = data.NewMapFromMap(m)
		} else if a, ok := v.([]interface{}); ok {
			v = data.NewArrayFromInterfaces(data.InterfaceType, a...)
		}

		return data.NewList(v, err), err
	}

	// There is a model, so do some mapping if possible.
	pointer, ok := args.Get(1).(*interface{})
	if !ok {
		return data.NewList(errors.ErrInvalidPointerType), nil
	}

	value := *pointer

	switch target := value.(type) {
	case *data.Struct:
		if m, ok := v.(map[string]interface{}); ok {
			for k, v := range m {
				if err = target.Set(k, v); err != nil {
					err = errors.New(err).In("Unmarshal")

					return data.NewList(err), nil
				}
			}
		} else {
			return data.NewList(errors.ErrInvalidType), nil
		}

		*pointer = target

		return data.NewList(nil), nil

	case *data.Map:
		if m, ok := v.(map[string]interface{}); ok {
			for k, v := range m {
				k2 := data.Coerce(k, data.InstanceOfType(target.KeyType()))
				v2 := v

				if !target.ElementType().IsInterface() {
					v2 = data.Coerce(v, data.InstanceOfType(target.ElementType()))
				}

				if _, err = target.Set(k2, v2); err != nil {
					return data.NewList(errors.New(err).In("Unmarshal")), nil
				}
			}
		} else {
			return data.NewList(errors.ErrInvalidType), nil
		}

		*pointer = target

		return data.NewList(nil), nil

	case *data.Array:
		if m, ok := v.([]interface{}); ok {
			// The target data size may be wrong, fix it
			target.SetSize(len(m))

			for k, v := range m {
				if target.Type().Kind() == data.StructKind {
					if mm, ok := v.(map[string]interface{}); ok {
						v = data.NewStructOfTypeFromMap(target.Type(), mm)
					}
				}

				if err = target.Set(k, v); err != nil {
					return data.NewList(errors.New(err)), nil
				}
			}
		} else {
			return data.NewList(errors.ErrInvalidType), nil
		}

		*pointer = target

		return data.NewList(nil), nil

	default:
		v := data.Coerce(v, target)
		if !data.TypeOf(v).IsType(data.TypeOf(value)) {
			err = errors.ErrInvalidType
			v = nil
		}

		*pointer = v

		if err != nil {
			err = errors.New(err)
		}

		return data.NewList(err), err
	}
}
