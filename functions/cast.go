package functions

import (
	"reflect"
	"strings"
	"sync"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// For a new() on an object, we won't recursively copy objects
// nested more deeply than this. Setting this too small will
// prevent complex structures from copying correctly. Too large,
// and memory could be swallowed whole.
const MaxDeepCopyDepth = 100

// Byte implements the byte() function.
func Byte(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if v := util.Coerce(args[0], byte(0)); v != nil {
		return v.(byte), nil
	}

	return nil, errors.New(errors.ErrInvalidType).In("int()").Context(args[0])
}

// Int32 implements the int32() function.
func Int32(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if v := util.Coerce(args[0], int32(32)); v != nil {
		return v.(int32), nil
	}

	return nil, errors.New(errors.ErrInvalidType).In("int()").Context(args[0])
}

// Int implements the int() function.
func Int(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if v := util.Coerce(args[0], 1); v != nil {
		return v.(int), nil
	}

	return nil, errors.New(errors.ErrInvalidType).In("int()").Context(args[0])
}

// Float32 implements the float32() function.
func Float32(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if v := util.Coerce(args[0], float32(1.0)); v != nil {
		return v.(float32), nil
	}

	return nil, errors.New(errors.ErrInvalidType).In("float32()").Context(args[0])
}

// Float64 implements the float64() function.
func Float64(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if v := util.Coerce(args[0], 1.0); v != nil {
		return v.(float64), nil
	}

	return nil, errors.New(errors.ErrInvalidType).In("float64()").Context(args[0])
}

// String implements the string() function.
func String(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
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

	// Is it an integer Ego array?
	if array, ok := args[0].(*datatypes.EgoArray); ok && array.ValueType().IsType(datatypes.IntType) {
		var b strings.Builder

		for i := 0; i < array.Len(); i++ {
			rune, _ := array.Get(i)
			b.WriteRune(int32(util.GetInt(rune)))
		}

		return b.String(), nil
	}

	return util.GetString(args[0]), nil
}

// Bool implements the bool() function.
func Bool(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	v := util.Coerce(args[0], true)
	if v == nil {
		return nil, errors.New(errors.ErrInvalidType).In("bool()").Context(args[0])
	}

	return v.(bool), nil
}

// Normalize coerces a value to match the type of a model value.
func Normalize(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	v1, v2 := util.Normalize(args[0], args[1])

	return MultiValueReturn{Value: []interface{}{v1, v2}}, nil
}

// New implements the new() function. If an integer type number
// or a string type name is given, the "zero value" for that type
// is returned. For an array, struct, or map, a recursive copy is
// done of the members to a new object which is returned.
func New(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
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
			return nil, errors.New(errors.ErrInvalidType).In("new()").Context(typeValue)
		}
	}

	// Is it an actual type?
	if typeValue, ok := args[0].(datatypes.Type); ok {
		return typeValue.InstanceOf(&typeValue), nil
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

		case "float64":
			return float64(0), nil

		default:
			return nil, errors.New(errors.ErrInvalidType).In("new()").Context(typeValue)
		}
	}

	// If it's a WaitGroup, make a new one.
	if _, ok := args[0].(sync.WaitGroup); ok {
		return datatypes.InstanceOfType(datatypes.WaitGroupType), nil
	}

	// If it's a Mutex, make a new one.
	if _, ok := args[0].(sync.Mutex); ok {
		return datatypes.InstanceOfType(datatypes.MutexType), nil
	}

	// If it's a channel, just return the value
	if typeValue, ok := args[0].(*datatypes.Channel); ok {
		return typeValue, nil
	}

	// If it's a native struct, it has it's own deep copy.
	if structValue, ok := args[0].(*datatypes.EgoStruct); ok {
		return datatypes.DeepCopy(structValue), nil
	}

	// @tomcole should we also handle maps and arrays here?

	// Otherwise, make a deep copy of the item.
	r := DeepCopy(args[0], MaxDeepCopyDepth)

	// If there was a user-defined type in the source, make the clone point back to it
	switch v := r.(type) {
	case nil:
		return nil, errors.New(errors.ErrInvalidValue).In("new()").Context(nil)

	case symbols.SymbolTable:
		return nil, errors.New(errors.ErrInvalidValue).In("new()").Context("symbol table")

	case func(*symbols.SymbolTable, []interface{}) (interface{}, error):
		return nil, errors.New(errors.ErrInvalidValue).In("new()").Context("builtin function")

	case int:
	case string:
	case float32:
	case float64:
	case datatypes.EgoPackage:
		// Create the replica count if needed, and update it.
		replica := 0

		if replicaX, ok := datatypes.GetMetadata(v, datatypes.ReplicaMDKey); ok {
			replica = util.GetInt(replicaX) + 1
		}

		datatypes.SetMetadata(v, datatypes.ReplicaMDKey, replica)

		dropList := []string{}

		// Organize the new item by removing things that are handled via the parent.
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
			delete(v, name)
		}

	default:
		return nil, errors.New(errors.ErrInvalidType).In("new()").Context(v)
	}

	return r, nil
}

// DeepCopy makes a deep copy of an Ego data type. It should be called with the
// maximum nesting depth permitted (i.e. array index->array->array...). Because
// it calls itself recursively, this is used to determine when to give up and
// stop traversing nested data. The default is MaxDeepCopyDepth.
func DeepCopy(source interface{}, depth int) interface{} {
	if depth < 0 {
		return nil
	}

	switch v := source.(type) {
	case int:
		return v

	case string:
		return v

	case float32:
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

	case *datatypes.EgoStruct:
		return v.Copy()

	case *datatypes.EgoArray:
		r := datatypes.NewArray(v.ValueType(), v.Len())

		for i := 0; i < v.Len(); i++ {
			vv, _ := v.Get(i)
			vv = DeepCopy(vv, depth-1)
			_ = v.Set(i, vv)
		}

		return r

	case *datatypes.EgoMap:
		r := datatypes.NewMap(v.KeyType(), v.ValueType())

		for _, k := range v.Keys() {
			d, _, _ := v.Get(k)
			_, _ = r.Set(k, DeepCopy(d, depth-1))
		}

		return r

	case datatypes.EgoPackage:
		r := datatypes.EgoPackage{}

		for k, d := range v {
			r[k] = DeepCopy(d, depth-1)
		}

		return r

	default:
		return v
	}
}

// Compiler-generate casting; generally always array types. This is used to
// convert numeric arrays to a different kind of array, to convert a string
// to an array of integer (rune) values, etc.
func InternalCast(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	// Target kind is the last parameter
	kind := datatypes.GetType(args[len(args)-1])
	if !kind.IsArray() {
		return nil, errors.New(errors.ErrInvalidType)
	}

	source := args[0]
	if len(args) > 2 {
		source = datatypes.NewArrayFromArray(datatypes.InterfaceType, args[:len(args)-1])
	}

	switch actual := source.(type) {
	// Conversion of one array type to another
	case *datatypes.EgoArray:
		if kind.IsType(actual.ValueType()) {
			return actual, nil
		}

		elementKind := *kind.BaseType()
		r := datatypes.NewArray(*kind.BaseType(), actual.Len())

		for i := 0; i < actual.Len(); i++ {
			v, _ := actual.Get(i)

			if elementKind.IsType(datatypes.IntType) {
				_ = r.Set(i, util.GetInt(v))
			} else if elementKind.IsType(datatypes.Float64Type) {
				_ = r.Set(i, util.GetFloat64(v))
			} else if elementKind.IsType(datatypes.Float32Type) {
				_ = r.Set(i, util.GetFloat32(v))
			} else if elementKind.IsType(datatypes.StringType) {
				_ = r.Set(i, util.GetString(v))
			} else if elementKind.IsType(datatypes.BoolType) {
				_ = r.Set(i, util.GetBool(v))
			} else {
				return nil, errors.New(errors.ErrInvalidType)
			}
		}

		return r, nil

	case string:
		if !kind.IsType(datatypes.Array(datatypes.IntType)) {
			return nil, errors.New(errors.ErrInvalidType)
		}

		r := datatypes.NewArray(datatypes.IntType, 0)

		for _, rune := range actual {
			r.Append(int(rune))
		}

		return r, nil

	default:
		return nil, errors.New(errors.ErrInvalidType)
	}
}
