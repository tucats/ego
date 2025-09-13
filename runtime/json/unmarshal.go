package json

import (
	"encoding/json"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// unmarshal reads a byte array or string as JSON data.
func unmarshal(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		decodedValue any
		err          error
	)

	// Simplest case, []byte input. Otherwise, treat the argument
	// as a string.
	if a, ok := args.Get(0).(*data.Array); ok && a.Type().Kind() == data.ByteKind {
		err = json.Unmarshal(a.GetBytes(), &decodedValue)
	} else {
		jsonBuffer := data.String(args.Get(0))
		err = json.Unmarshal([]byte(jsonBuffer), &decodedValue)
	}

	if err != nil {
		err = errors.New(err).In("Unmarshal")

		return data.NewList(nil, err), err
	}

	// If there is no model, assume a generic return value is okay
	if args.Len() < 2 {
		// Hang on, if the result is a map, then Ego won't be able to use it,
		// so convert that to an EgoMap. Same for an array.
		if m, ok := decodedValue.(map[string]any); ok {
			decodedValue = data.NewMapFromMap(m)
		} else if a, ok := decodedValue.([]any); ok {
			decodedValue = data.NewArrayFromInterfaces(data.InterfaceType, a...)
		}

		return data.NewList(decodedValue, err), err
	}

	// There is a model, so do some mapping if possible.
	pointer, ok := args.Get(1).(*any)
	if !ok {
		return data.NewList(errors.ErrInvalidPointerType), nil
	}

	return remapDecodedValue(decodedValue, pointer)
}

// When decoding a JSON value into an Ego object by pointer, we must remap the actual data item to a suitable
// Ego object and write it via the interface pointer provided by the caller. Basically, this handles the fact
// that the data value and the pointer are both abstract interface types, but we have to do the conversions
// using the underlying real value.
func remapDecodedValue(decodedValue any, destinationPointer *any) (any, error) {
	var err error

	destination := *destinationPointer

	// Depending on the actual type of the destination value, convert the Native decoded data to the Ego type.
	switch target := destination.(type) {
	case *data.Struct:
		// If we are writing to a struct, the JSON data has to be a map. Use the map keys as struct field
		// names and attempt to write the values to the structure.
		if m, ok := decodedValue.(map[string]any); ok {
			for k, v := range m {
				if err = target.Set(k, v); err != nil {
					err = errors.New(err).In("Unmarshal")

					return data.NewList(err), nil
				}
			}
		} else {
			return data.NewList(errors.ErrInvalidType), nil
		}

		*destinationPointer = target

		return data.NewList(nil), nil

	case *data.Map:
		// If the target is a map, convert the abstract map data to the correct type based on the declaration of
		// the Ego map type.
		if m, ok := decodedValue.(map[string]any); ok {
			for k, v := range m {
				k2, err := data.Coerce(k, data.InstanceOfType(target.KeyType()))
				if err != nil {
					return nil, err
				}

				v2 := v

				if !target.ElementType().IsInterface() {
					v2, err = data.Coerce(v, data.InstanceOfType(target.ElementType()))
					if err != nil {
						return nil, err
					}
				}

				if _, err = target.Set(k2, v2); err != nil {
					return data.NewList(errors.New(err).In("Unmarshal")), nil
				}
			}
		} else {
			return data.NewList(errors.ErrInvalidType), nil
		}

		*destinationPointer = target

		return data.NewList(nil), nil

	case *data.Array:
		// If the target is an array, convert the abstract array data to the correct type based on the declaration of
		// the Ego array type.
		if m, ok := decodedValue.([]any); ok {
			target.SetSize(len(m))

			for k, v := range m {
				if target.Type().Kind() == data.StructKind {
					if mm, ok := v.(map[string]any); ok {
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

		*destinationPointer = target

		return data.NewList(nil), nil

	default:
		// Not a complex type, so convert the abstract value to a suitable Ego type.
		v, err := data.Coerce(decodedValue, target)
		if err != nil {
			return nil, err
		}

		if !data.TypeOf(v).IsType(data.TypeOf(destination)) {
			err = errors.ErrInvalidType
			v = nil
		}

		*destinationPointer = v

		if err != nil {
			err = errors.New(err)
		}

		return data.NewList(err), err
	}
}
