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

		return data.NewList(err), err
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

// remapDecodedValue writes the raw Go value produced by encoding/json into the
// Ego object referenced by destinationPointer. The destination object acts as
// a type model: its concrete Go type determines how the decoded value is
// converted. Struct, array, and map destinations are updated in-place so that
// fields/elements absent from the JSON retain their existing values.
func remapDecodedValue(decodedValue any, destinationPointer *any) (any, error) {
	var err error

	destination := *destinationPointer

	switch target := destination.(type) {
	case *data.Struct:
		// Write JSON object fields into the existing struct, converting each
		// field value to the type declared for that field.
		if m, ok := decodedValue.(map[string]any); ok {
			for k, v := range m {
				existing, _ := target.Get(k)
				converted, convErr := reconstructValue(v, existing)
				if convErr != nil {
					return data.NewList(errors.New(convErr).In("Unmarshal")), nil
				}

				if err = target.Set(k, converted); err != nil {
					return data.NewList(errors.New(err).In("Unmarshal")), nil
				}
			}
		} else {
			return data.NewList(errors.ErrInvalidType), nil
		}

		*destinationPointer = target

		return data.NewList(nil), nil

	case *data.Map:
		// Write JSON object entries into the existing map, converting each
		// key and value to the declared types.
		if m, ok := decodedValue.(map[string]any); ok {
			// Build a model value for map entries so reconstructValue can
			// apply type-aware conversion. Use nil when the value type is
			// interface{} so reconstructValue falls through to best-effort.
			var elemModel any
			if !target.ElementType().IsInterface() {
				elemModel = data.InstanceOfType(target.ElementType())
			}

			for k, v := range m {
				k2, err := data.Coerce(k, data.InstanceOfType(target.KeyType()))
				if err != nil {
					return data.NewList(errors.New(err).In("Unmarshal")), nil
				}

				v2, err := reconstructValue(v, elemModel)
				if err != nil {
					return data.NewList(errors.New(err).In("Unmarshal")), nil
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
		// Write JSON array elements into the existing array, converting each
		// element to the declared element type. The element type hint is read
		// from the existing array contents before SetSize may clear them.
		if m, ok := decodedValue.([]any); ok {
			origLen := target.Len()

			// Save the first element as a type hint for newly-appended slots.
			var elemModel any
			if origLen > 0 {
				elemModel, _ = target.Get(0)
			}

			target.SetSize(len(m))

			for i, v := range m {
				// For indices within the original array, use the existing element
				// as the model (it carries the correct Ego type). For new indices
				// beyond the original length, fall back to the first-element hint.
				var thisModel any
				if i < origLen {
					thisModel, _ = target.Get(i)
				} else {
					thisModel = elemModel
				}

				converted, convErr := reconstructValue(v, thisModel)
				if convErr != nil {
					return data.NewList(errors.New(convErr).In("Unmarshal")), nil
				}

				if err = target.Set(i, converted); err != nil {
					return data.NewList(errors.New(err).In("Unmarshal")), nil
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

// reconstructValue converts a raw Go value (decoded by encoding/json) into
// the Ego type indicated by model. model is an existing Ego value whose
// concrete type drives the conversion — it is inspected for its type but
// never modified. If model is nil or carries no useful type information,
// best-effort conversion is applied: JSON objects become Ego maps, JSON
// arrays become Ego []any arrays, and scalars are returned as-is.
//
// reconstructValue is called recursively so that arbitrarily deep nesting
// (arrays of structs, structs containing maps of structs, etc.) is handled
// uniformly at every level.
func reconstructValue(decoded any, model any) (any, error) {
	if decoded == nil {
		return nil, nil
	}

	switch m := model.(type) {
	case *data.Struct:
		// JSON object → Ego struct. Create a fresh struct of the same type
		// and populate it field by field, recursing into each value.
		mm, ok := decoded.(map[string]any)
		if !ok {
			return decoded, nil
		}

		result := data.NewStruct(m.Type())

		for k, v := range mm {
			existing, _ := m.Get(k)
			converted, err := reconstructValue(v, existing)
			if err != nil {
				return nil, err
			}

			// Silently skip fields not declared in this struct type.
			_ = result.Set(k, converted)
		}

		return result, nil

	case *data.Array:
		// JSON array → Ego array with the same element type. Use the
		// existing element at each position as the per-element model;
		// fall back to the first element for out-of-bounds positions.
		arr, ok := decoded.([]any)
		if !ok {
			return decoded, nil
		}

		origLen := m.Len()

		var elemModel any
		if origLen > 0 {
			elemModel, _ = m.Get(0)
		}

		result := data.NewArray(m.Type(), len(arr))

		for i, v := range arr {
			var thisModel any
			if i < origLen {
				thisModel, _ = m.Get(i)
			} else {
				thisModel = elemModel
			}

			converted, err := reconstructValue(v, thisModel)
			if err != nil {
				converted = v // fall back to raw value for this element
			}

			if setErr := result.Set(i, converted); setErr != nil {
				return nil, errors.New(setErr)
			}
		}

		return result, nil

	case *data.Map:
		// JSON object → Ego map with the same key and value types.
		mm, ok := decoded.(map[string]any)
		if !ok {
			return decoded, nil
		}

		var elemModel any
		if !m.ElementType().IsInterface() {
			elemModel = data.InstanceOfType(m.ElementType())
		}

		result := data.NewMap(m.KeyType(), m.ElementType())

		for k, v := range mm {
			k2, err := data.Coerce(k, data.InstanceOfType(m.KeyType()))
			if err != nil {
				return nil, err
			}

			v2, err := reconstructValue(v, elemModel)
			if err != nil {
				v2 = v // fall back to raw value
			}

			if _, setErr := result.Set(k2, v2); setErr != nil {
				return nil, errors.New(setErr)
			}
		}

		return result, nil

	default:
		// Scalar model (int, string, bool, float64, etc.) or nil.
		if model != nil {
			if converted, err := data.Coerce(decoded, model); err == nil {
				return converted, nil
			}
		}

		// No usable type hint: best-effort conversion so callers always
		// get a usable Ego value rather than a raw Go type.
		switch v := decoded.(type) {
		case map[string]any:
			return data.NewMapFromMap(v), nil
		case []any:
			return data.NewArrayFromInterfaces(data.InterfaceType, v...), nil
		default:
			return decoded, nil
		}
	}
}
