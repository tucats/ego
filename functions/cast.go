package functions

import (
	"math"
	"reflect"
	"strings"
	"sync"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// For a new() on an object, we won't recursively copy objects
// nested more deeply than this. Setting this too small will
// prevent complex structures from copying correctly. Too large,
// and memory could be swallowed whole.
const MaxDeepCopyDepth = 100

// Normalize coerces a value to match the type of a model value.
func Normalize(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	v1, v2 := data.Normalize(args[0], args[1])

	return MultiValueReturn{Value: []interface{}{v1, v2}}, nil
}

// New implements the new() function. If an integer type number
// or a string type name is given, the "zero value" for that type
// is returned. For an array, struct, or map, a recursive copy is
// done of the members to a new object which is returned.
func New(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Is the type an integer? If so it's a type
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
			return nil, errors.EgoError(errors.ErrInvalidType).In("new()").Context(typeValue)
		}
	}

	// Is it an actual type?
	if typeValue, ok := args[0].(*data.Type); ok {
		return typeValue.InstanceOf(typeValue), nil
	}

	if typeValue, ok := args[0].(*data.Type); ok {
		return typeValue.InstanceOf(typeValue), nil
	}

	// Is the type an string? If so it's a type name
	if typeValue, ok := args[0].(string); ok {
		switch strings.ToLower(typeValue) {
		case data.ByteType.Name():
			return byte(0), nil

		case data.Int32TypeName:
			return int32(0), nil

		case data.Int64TypeName:
			return int64(0), nil

		case data.IntTypeName:
			return 0, nil

		case data.StringTypeName:
			return "", nil

		case data.BoolType.Name():
			return false, nil

		case data.Float32TypeName:
			return float32(0), nil

		case data.Float64TypeName:
			return float64(0), nil

		default:
			return nil, errors.EgoError(errors.ErrInvalidType).In("new()").Context(typeValue)
		}
	}

	// If it's a WaitGroup, make a new one. Note, have to use the switch statement
	// form here to prevent Go from complaining that the interface{} is being copied.
	// In reality, we don't care as we don't actually make a copy anyway but instead
	// make a new waitgroup object.
	switch args[0].(type) {
	case sync.WaitGroup:
		return data.InstanceOfType(&data.WaitGroupType), nil
	}

	// If it's a Mutex, make a new one. We hae to do this as a swtich on the type, since a
	// cast attempt will yield a warning on invalid mutex copy operation.
	switch args[0].(type) {
	case sync.Mutex:
		return data.InstanceOfType(&data.MutexType), nil
	}

	// If it's a channel, just return the value
	if typeValue, ok := args[0].(*data.Channel); ok {
		return typeValue, nil
	}

	// If it's a native struct, it has it's own deep copy.
	if structValue, ok := args[0].(*data.EgoStruct); ok {
		return data.DeepCopy(structValue), nil
	}

	// Otherwise, make a deep copy of the item.
	r := DeepCopy(args[0], MaxDeepCopyDepth)

	// If there was a user-defined type in the source, make the clone point back to it
	switch v := r.(type) {
	case nil:
		return nil, errors.EgoError(errors.ErrInvalidValue).In("new()").Context(nil)

	case symbols.SymbolTable:
		return nil, errors.EgoError(errors.ErrInvalidValue).In("new()").Context("symbol table")

	case func(*symbols.SymbolTable, []interface{}) (interface{}, error):
		return nil, errors.EgoError(errors.ErrInvalidValue).In("new()").Context("builtin function")

	// No action for this group
	case byte, int32, int, int64, string, float32, float64:

	case *data.EgoPackage:
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
		return nil, errors.EgoError(errors.ErrInvalidType).In("new()").Context(v)
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
	case bool:
		return v

	case byte:
		return v

	case int32:
		return v

	case int:
		return v

	case int64:
		return v

	case string:
		return v

	case float32:
		return v

	case float64:
		return v

	case []interface{}:
		r := make([]interface{}, 0)

		for _, d := range v {
			r = append(r, DeepCopy(d, depth-1))
		}

		return r

	case *data.EgoStruct:
		return v.Copy()

	case *data.EgoArray:
		r := data.NewArray(v.ValueType(), v.Len())

		for i := 0; i < v.Len(); i++ {
			vv, _ := v.Get(i)
			vv = DeepCopy(vv, depth-1)
			_ = v.Set(i, vv)
		}

		return r

	case *data.EgoMap:
		r := data.NewMap(v.KeyType(), v.ValueType())

		for _, k := range v.Keys() {
			d, _, _ := v.Get(k)
			_, _ = r.Set(k, DeepCopy(d, depth-1))
		}

		return r

	case *data.EgoPackage:
		r := data.EgoPackage{}
		keys := v.Keys()

		for _, k := range keys {
			d, _ := v.Get(k)
			r.Set(k, DeepCopy(d, depth-1))
		}

		return &r

	default:
		return v
	}
}

// Compiler-generate casting; generally always array types. This is used to
// convert numeric arrays to a different kind of array, to convert a string
// to an array of integer (rune) values, etc.
func InternalCast(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Target kind is the last parameter
	kind := data.TypeOf(args[len(args)-1])

	source := args[0]
	if len(args) > 2 {
		source = data.NewArrayFromArray(&data.InterfaceType, args[:len(args)-1])
	}

	if kind.IsKind(data.StringKind) {
		r := strings.Builder{}

		// If the source is an array of integers, treat them as runes to re-assemble.
		if actual, ok := source.(*data.EgoArray); ok && actual != nil && actual.ValueType().IsIntegerType() {
			for i := 0; i < actual.Len(); i++ {
				ch, _ := actual.Get(i)
				r.WriteRune(rune(data.Int(ch) & math.MaxInt32))
			}
		} else {
			str := data.FormatUnquoted(source)
			r.WriteString(str)
		}

		return r.String(), nil
	}

	switch actual := source.(type) {
	// Conversion of one array type to another
	case *data.EgoArray:
		if kind.IsType(actual.ValueType()) {
			return actual, nil
		}

		if kind.IsKind(data.StringKind) &&
			(actual.ValueType().IsIntegerType() || actual.ValueType().IsInterface()) {
			r := strings.Builder{}

			for i := 0; i < actual.Len(); i++ {
				ch, _ := actual.Get(i)
				r.WriteRune(data.Int32(ch) & math.MaxInt32)
			}

			return r.String(), nil
		}

		elementKind := *kind.BaseType()
		r := data.NewArray(kind.BaseType(), actual.Len())

		for i := 0; i < actual.Len(); i++ {
			v, _ := actual.Get(i)

			switch elementKind.Kind() {
			case data.BoolKind:
				_ = r.Set(i, data.Bool(v))

			case data.ByteKind:
				_ = r.Set(i, data.Byte(v))

			case data.Int32Kind:
				_ = r.Set(i, data.Int32(v))

			case data.IntKind:
				_ = r.Set(i, data.Int(v))

			case data.Int64Kind:
				_ = r.Set(i, data.Int64(v))

			case data.Float32Kind:
				_ = r.Set(i, data.Float32(v))

			case data.Float64Kind:
				_ = r.Set(i, data.Float64(v))

			case data.StringKind:
				_ = r.Set(i, data.String(v))

			default:
				return nil, errors.EgoError(errors.ErrInvalidType).Context(data.TypeOf(v).String())
			}
		}

		return r, nil

	case string:
		if kind.IsType(data.Array(&data.IntType)) {
			r := data.NewArray(&data.IntType, 0)

			for _, rune := range actual {
				r.Append(int(rune))
			}

			return r, nil
		}

		return data.Coerce(source, data.InstanceOfType(kind)), nil

	default:
		if kind.IsArray() {
			r := data.NewArray(kind.BaseType(), 1)
			value := data.Coerce(source, data.InstanceOfType(kind.BaseType()))
			_ = r.Set(0, value)

			return r, nil
		}

		v := data.Coerce(source, data.InstanceOfType(kind))
		if v != nil {
			return data.Coerce(source, data.InstanceOfType(kind)), nil
		}

		return nil, errors.EgoError(errors.ErrInvalidType).Context(data.TypeOf(source).String())
	}
}
