package functions

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Int implements the int() function
func Int(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	v := util.Coerce(args[0], 1)
	if v == nil {
		return nil, NewError("int", InvalidTypeError)
	}

	return v.(int), nil
}

// Float implements the float() function
func Float(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	v := util.Coerce(args[0], 1.0)
	if v == nil {
		return nil, NewError("float", InvalidValueError, args[0])
	}

	return v.(float64), nil
}

// String implements the string() function
func String(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Special case. Is the argument an array of strings? If so, restructure as a single
	// string with line breaks.
	if array, ok := args[0].([]interface{}); ok {
		isString := true

		for _, v := range array {
			if _, ok := v.(string); !ok {
				isString = false

				break
			}
		}

		if isString {
			var b strings.Builder

			for i, v := range array {
				if i > 0 {
					b.WriteString("\n")
				}
				b.WriteString(v.(string))
			}

			return b.String(), nil
		}
	}

	return util.GetString(args[0]), nil
}

// Bool implements the bool() function
func Bool(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	v := util.Coerce(args[0], true)
	if v == nil {
		return nil, NewError("bool", InvalidValueError, args[0])
	}

	return v.(bool), nil
}

// Coerce coerces a value to match the type of a model value
func Coerce(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return util.Coerce(args[0], args[1]), nil
}

// Normalize coerces a value to match the type of a model value
func Normalize(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	v1, v2 := util.Normalize(args[0], args[1])

	return []interface{}{v1, v2}, nil
}

// New implements the new() function
func New(syms *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Is the type an integer? If so it's a type
	if typeValue, ok := args[0].(int); ok {
		switch reflect.Kind(typeValue) {
		case reflect.Int:
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
			return nil, fmt.Errorf("unsupported new() type %d", typeValue)
		}
	}

	// Is the type an string? If so it's a type name
	if typeValue, ok := args[0].(string); ok {
		switch strings.ToLower(typeValue) {
		case "int":
			return 0, nil

		case "string":
			return "", nil

		case "bool":
			return false, nil

		case "float32":
			return float32(0), nil

		case "float", "float64":
			return float64(0), nil

		default:
			return nil, fmt.Errorf("unsupported new() type %s", typeValue)
		}
	}

	// Otherwise, make a deep copy of the item.
	r := DeepCopy(args[0], 10)

	// IF there was a type in the source, make the clone point back to it
	switch v := r.(type) {
	case nil:
		return nil, NewError("new", InvalidNewValueError)

	case symbols.SymbolTable:
		return nil, NewError("new", InvalidNewValueError)

	case func(*symbols.SymbolTable, []interface{}) (interface{}, error):
		return nil, NewError("new", InvalidNewValueError)

	case int:
	case string:
	case float64:
	case []interface{}:
	case map[string]interface{}:
		// Create the replica count if needed, and update it.
		replica := 0
		if replicaX, ok := datatypes.GetMetadata(v, datatypes.ReplicaMDKey); ok {
			replica = util.GetInt(replicaX) + 1
		}

		datatypes.SetMetadata(v, datatypes.ReplicaMDKey, replica)

		// Organize the new item by removing things that are handled via the parent.
		dropList := []string{}

		for k, vv := range v {
			// IF it's an internal function, we don't want to copy it; it can be found via the
			// __parent link to the type
			vx := reflect.ValueOf(vv)

			if vx.Kind() == reflect.Ptr {
				ts := vx.String()
				if ts == "<*bytecode.ByteCode Value>" {
					dropList = append(dropList, k)
				}
			} else {
				if vx.Kind() == reflect.Func {
					dropList = append(dropList, k)
				}
			}
		}

		for _, name := range dropList {
			delete(r.(map[string]interface{}), name)
		}
		// If there is a parent key, override it with this item.
		if _, ok := datatypes.GetMetadata(r, datatypes.ParentMDKey); ok {
			datatypes.SetMetadata(r, datatypes.ParentMDKey, args[0])
		}

	default:
		return nil, NewError("new", InvalidTypeError)
	}

	return r, nil
}

// DeepCopy makes a deep copy of an Ego data type
func DeepCopy(source interface{}, depth int) interface{} {
	if depth < 0 {
		return nil
	}

	switch v := source.(type) {
	case int:
		return v

	case string:
		return v

	case float64:
		return v

	case bool:
		return v

	case []interface{}:
		r := make([]interface{}, 0)

		for _, d := range v {
			r = append(r, DeepCopy(d, depth-1))
		}

		return r

	case *datatypes.EgoMap:
		r := datatypes.NewMap(v.KeyType(), v.ValueType())

		for _, k := range v.Keys() {
			d, _, _ := v.Get(k)
			_, _ = r.Set(k, DeepCopy(d, depth-1))
		}

		return r

	case map[string]interface{}:
		r := map[string]interface{}{}

		for k, d := range v {
			r[k] = DeepCopy(d, depth-1)
		}

		return r

	default:
		return v
	}
}
