package functions

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Sleep implements util.sleep().
func Sleep(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	duration, err := time.ParseDuration(util.GetString(args[0]))
	if errors.Nil(err) {
		time.Sleep(duration)
	}

	return true, errors.New(err)
}

// ProfileGet implements the profile.get() function.
func ProfileGet(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	key := util.GetString(args[0])

	return persistence.Get(key), nil
}

// ProfileSet implements the profile.set() function.
func ProfileSet(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	key := util.GetString(args[0])

	// Quick check here. The key must already exist if it's one of the
	// "system" settings. That is, you can't create an ego.* setting that
	// doesn't exist yet, for example
	if strings.HasPrefix(key, "ego.") {
		if !persistence.Exists(key) {
			return nil, errors.New(errors.ReservedProfileSetting).In("Set()").Context(key)
		}
	}
	// If the value is an empty string, delete the key else
	// store the value for the key.
	value := util.GetString(args[1])
	if value == "" {
		persistence.Delete(key)
	} else {
		persistence.Set(key, value)
	}

	return nil, persistence.Save()
}

// ProfileDelete implements the profile.delete() function.
func ProfileDelete(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	key := util.GetString(args[0])
	persistence.Delete(key)

	return nil, nil
}

// ProfileKeys implements the profile.keys() function.
func ProfileKeys(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	keys := persistence.Keys()
	result := make([]interface{}, len(keys))

	for i, key := range keys {
		result[i] = key
	}

	return result, nil
}

// Length implements the len() function.
func Length(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if args[0] == nil {
		return 0, nil
	}

	switch arg := args[0].(type) {
	// For a channel, it's length either zero if it's drained, or bottomless
	case *datatypes.Channel:
		size := int(math.MaxInt32)
		if arg.IsEmpty() {
			size = 0
		}

		return size, nil

	case *datatypes.EgoArray:
		return arg.Len(), nil

	case error:
		return len(arg.Error()), nil

	case *datatypes.EgoMap:
		return len(arg.Keys()), nil

	case map[string]interface{}:
		keys := make([]string, 0)

		for k := range arg {
			if !strings.HasPrefix(k, "__") {
				keys = append(keys, k)
			}
		}

		return len(keys), nil

	case []interface{}:
		return len(arg), nil

	case nil:
		return 0, nil

	default:
		v := util.Coerce(args[0], "")
		if v == nil {
			return 0, nil
		}

		return len(v.(string)), nil
	}
}

// StrLen is the strings.Length() function, which counts characters/runes instead of
// bytes like len() does.
func StrLen(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	count := 0
	v := util.GetString(args[0])

	for range v {
		count++
	}

	return count, nil
}

// GetEnv implements the util.getenv() function which reads
// an environment variable from the os.
func GetEnv(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return os.Getenv(util.GetString(args[0])), nil
}

// GetMode implements the util.Mode() function which reports the runtime mode.
func GetMode(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	m, ok := symbols.Get("__exec_mode")
	if !ok {
		m = "run"
	}

	return m, nil
}

// Members gets an array of the names of the fields in a structure.
func Members(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	switch v := args[0].(type) {
	case *datatypes.EgoMap:
		keys := datatypes.NewArray(datatypes.StringType, 0)
		keyList := v.Keys()

		for i, v := range keyList {
			_ = keys.Set(i, v)
		}

		_ = keys.Sort()

		return keys, nil

	case *datatypes.EgoStruct:
		return v.FieldNamesArray(), nil

	case map[string]interface{}:
		keys := datatypes.NewArray(datatypes.StringType, 0)

		for k := range v {
			if !strings.HasPrefix(k, "__") {
				keys.Append(k)
			}
		}

		err := keys.Sort()

		return keys, err

	default:
		return nil, errors.New(errors.InvalidTypeError).In("members()")
	}
}

// SortStrings implements the sort.Strings function.
func SortStrings(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsType(datatypes.StringType) {
			err := array.Sort()

			return array, err
		} else {
			return nil, errors.New(errors.WrongArrayValueType).Context("sort.Strings()")
		}
	} else {
		return nil, errors.New(errors.ArgumentTypeError).Context("sort.Strings()")
	}
}

// SortInts implements the sort.Ints function.
func SortInts(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsType(datatypes.IntType) {
			err := array.Sort()

			return array, err
		} else {
			return nil, errors.New(errors.WrongArrayValueType).Context("sort.Ints()")
		}
	} else {
		return nil, errors.New(errors.ArgumentTypeError).Context("sort.Ints()")
	}
}

// SortFloats implements the sort.Floats function.
func SortFloats(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsType(datatypes.FloatType) {
			err := array.Sort()

			return array, err
		} else {
			return nil, errors.New(errors.WrongArrayValueType).Context("sort.Floats()")
		}
	} else {
		return nil, errors.New(errors.ArgumentTypeError).Context("sort.Floats()")
	}
}

// Sort implements the sort() function.
// TODO remove this (deprecated; replaced by sort package).
func Sort(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	// Make a master array of the values presented
	var array []interface{}

	// Special case. If there is a single argument, and it is already an Ego array,
	// use the native sort function
	if len(args) == 1 {
		if array, ok := args[0].(*datatypes.EgoArray); ok {
			err := array.Sort()

			return array, err
		}
	}

	for _, a := range args {
		switch v := a.(type) {
		case *datatypes.EgoArray:
			array = append(array, v.BaseArray()...)

		case []interface{}:
			array = append(array, v...)

		default:
			array = append(array, v)
		}
	}

	if len(array) == 0 {
		return array, nil
	}

	v1 := array[0]

	switch v1.(type) {
	case int:
		intArray := make([]int, 0)

		for _, i := range array {
			intArray = append(intArray, util.GetInt(i))
		}

		sort.Ints(intArray)

		resultArray := datatypes.NewArray(datatypes.IntType, len(array))

		for n, i := range intArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case float64:
		floatArray := make([]float64, 0)

		for _, i := range array {
			floatArray = append(floatArray, util.GetFloat(i))
		}

		sort.Float64s(floatArray)

		resultArray := datatypes.NewArray(datatypes.FloatType, len(array))

		for n, i := range floatArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case string:
		stringArray := make([]string, 0)

		for _, i := range array {
			stringArray = append(stringArray, util.GetString(i))
		}

		sort.Strings(stringArray)

		resultArray := datatypes.NewArray(datatypes.StringType, len(array))

		for n, i := range stringArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	default:
		return nil, errors.New(errors.InvalidTypeError).In("sort()")
	}
}

// Exit implements the os.exit() function.
func Exit(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	// If no arguments, just do a simple exit
	if len(args) == 0 {
		os.Exit(0)
	}

	switch v := args[0].(type) {
	case int:
		os.Exit(v)

	case string:
		return nil, errors.NewMessage(v)

	default:
		os.Exit(0)
	}

	return nil, nil
}

// FormatSymbols implements the util.symbols() function. We skip over the current
// symbol table, which was created just for this function call and will always be
// empty.
func FormatSymbols(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return s.Parent.Format(false), nil
}

// Type implements the type() function.
func Type(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	switch v := args[0].(type) {
	case *datatypes.EgoMap:
		return v.TypeString(), nil

	case *datatypes.EgoArray:
		return v.TypeString(), nil

	case *datatypes.EgoStruct:
		return v.TypeString(), nil

	case nil:
		return "nil", nil

	case error:
		return "error", nil

	case *datatypes.Channel:
		return "chan", nil

	case datatypes.Type:
		typeName := v.String()

		space := strings.Index(typeName, " ")
		if space > 0 {
			typeName = typeName[space+1:]
		}

		return "type " + typeName, nil

	case []interface{}:
		kind := datatypes.UndefinedType

		for _, n := range v {
			k2 := datatypes.TypeOf(n)
			if !kind.IsType(k2) {
				if kind.IsUndefined() {
					kind = k2
				} else {
					kind = datatypes.UndefinedType
				}
			}
		}

		return kind.String(), nil

	case map[string]interface{}:
		t := datatypes.TypeOf(v)

		if t.IsTypeDefinition() {
			return t.Name(), nil
		}

		return t.String(), nil

	case *interface{}:
		tt := datatypes.TypeOfPointer(v)

		return tt.String(), nil

	default:
		tt := datatypes.TypeOf(v)
		if tt.IsUndefined() {
			vv := reflect.ValueOf(v)
			if vv.Kind() == reflect.Func {
				return "builtin", nil
			}

			if vv.Kind() == reflect.Ptr {
				ts := vv.String()
				if ts == "<*bytecode.ByteCode Value>" {
					return "func", nil
				}

				return fmt.Sprintf("ptr %s", ts), nil
			}

			return "unknown", nil
		}

		return tt.String(), nil
	}
}

// Signal creates an error object based on the
// parameters.
func Signal(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	r := errors.New(errors.UserError)
	if len(args) > 0 {
		r = r.Context(args[0])
	}

	return r, nil
}

// Append implements the builtin append() function, which concatenates all the items
// together as an array. The first argument is flattened into the result, and then each
// additional argument is added to the array as-is.
func Append(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	result := make([]interface{}, 0)
	kind := datatypes.InterfaceType

	for i, j := range args {
		if array, ok := j.(*datatypes.EgoArray); ok && i == 0 {
			if !kind.IsType(datatypes.InterfaceType) {
				if err := array.Validate(kind); !errors.Nil(err) {
					return nil, err
				}
			}

			result = append(result, array.BaseArray()...)

			if kind.IsType(datatypes.InterfaceType) {
				kind = array.ValueType()
			}
		} else if array, ok := j.([]interface{}); ok && i == 0 {
			result = append(result, array...)
		} else {
			if !kind.IsType(datatypes.InterfaceType) && !datatypes.TypeOf(j).IsType(kind) {
				return nil, errors.New(errors.WrongArrayValueType).In("append()")
			}
			result = append(result, j)
		}
	}

	return datatypes.NewFromArray(kind, result), nil
}

// Delete can be used three ways. To delete a member from a structure, to delete
// an element from an array by index number, or to delete a symbol entirely. The
// first form requires a string name, the second form requires an integer index,
// and the third form does not have a second parameter.
func Delete(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if _, ok := args[0].(string); ok {
		if len(args) != 1 {
			return nil, errors.New(errors.ArgumentCountError).In("delete{}")
		}
	} else {
		if len(args) != 2 {
			return nil, errors.New(errors.ArgumentCountError).In("delete{}")
		}
	}

	switch v := args[0].(type) {
	case string:
		return nil, s.Delete(v, false)

	case *datatypes.EgoMap:
		_, err := v.Delete(args[1])

		return v, err

	case map[string]interface{}:
		key := util.GetString(args[1])
		delete(v, key)

		return v, nil

	case *datatypes.EgoArray:
		i := util.GetInt(args[1])
		err := v.Delete(i)

		return v, err

	case []interface{}:
		i := util.GetInt(args[1])
		if i < 0 || i >= len(v) {
			return nil, errors.New(errors.InvalidArrayIndexError).In("delete()")
		}

		r := append(v[:i], v[i+1:]...)

		return r, nil

	default:
		return nil, errors.New(errors.InvalidTypeError).In("delete()")
	}
}

// GetArgs implements util.Args() which fetches command-line arguments from
// the Ego command invocation, if any.
func GetArgs(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	r, found := s.Get("__cli_args")
	if !found {
		r = datatypes.NewArray(datatypes.StringType, 0)
	}

	return r, nil
}

// Make implements the make() function. The first argument must be a model of the
// array type (using the Go native version), and the second argument is the size.
func Make(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	kind := args[0]
	size := util.GetInt(args[1])

	if egoArray, ok := kind.(*datatypes.EgoArray); ok {
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
	switch kind.(type) {
	case *datatypes.Channel:
		return datatypes.NewChannel(size), nil

	case *datatypes.EgoArray:

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

	case map[string]interface{}:
		for i := range array {
			array[i] = map[string]interface{}{}
		}

	default:
		fmt.Printf("DEBUG: v = %#v\n", kind)
	}

	return array, nil
}

func Reflect(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	vv := reflect.ValueOf(args[0])
	ts := vv.String()

	// If it's a builtin function, it's description will match the signature. If it's a
	// match, find out it's name and return it as a builtin.
	if ts == "<func(*symbols.SymbolTable, []interface {}) (interface {}, error) Value>" {
		name := runtime.FuncForPC(reflect.ValueOf(args[0]).Pointer()).Name()
		name = strings.Replace(name, "github.com/tucats/ego/", "", 1)
		name = strings.Replace(name, "github.com/tucats/ego/runtime.", "", 1)

		return datatypes.NewStructFromMap(map[string]interface{}{
			datatypes.TypeMDKey:     "builtin",
			datatypes.BasetypeMDKey: "builtin " + name,
		}), nil
	}

	// If it's a bytecode.Bytecode pointer, use reflection to get the
	// Name field value and use that with the name. A function literal
	// will have no name.
	if vv.Kind() == reflect.Ptr {
		if ts == "<*bytecode.ByteCode Value>" {
			switch v := args[0].(type) {
			default:
				e := reflect.ValueOf(v).Elem()

				name, ok := e.Field(0).Interface().(string)
				if !ok || len(name) == 0 {
					name = "<anonymous>"
				}

				return datatypes.NewStructFromMap(map[string]interface{}{
					datatypes.TypeMDKey:     "func",
					datatypes.BasetypeMDKey: "func " + name,
				}), nil
			}
		}
	}

	if m, ok := args[0].(*datatypes.EgoStruct); ok {
		return m.Reflect(), nil
	}

	// Is it an Ego package?
	if m, ok := args[0].(map[string]interface{}); ok {
		// Make a list of the visible member names
		memberList := []string{}

		for k := range m {
			if !strings.HasPrefix(k, "__") {
				memberList = append(memberList, k)
			}
		}

		// Sort the member list and forge it into an Ego array
		members := datatypes.NewFromArray(datatypes.StringType, util.MakeSortedArray(memberList))

		result := map[string]interface{}{}

		metadata := m[datatypes.MetadataKey]
		if metadata == nil {
			result = map[string]interface{}{
				datatypes.MembersMDKey:  members,
				datatypes.TypeMDKey:     "struct",
				"native":                false,
				datatypes.BasetypeMDKey: "map",
			}
		} else {
			if mm, ok := metadata.(map[string]interface{}); ok {
				result[datatypes.MembersMDKey] = members
				result[datatypes.BasetypeMDKey] = "map"
				result[datatypes.TypeMDKey] = "struct"
				result["native"] = false
				result["basetype"] = "package"

				// If there's a Type designation, convert it to string format.
				if t, ok := mm[datatypes.TypeMDKey].(datatypes.Type); ok {
					result["type"] = t.TypeString()
				}
			}
		}

		return datatypes.NewStructFromMap(result), nil
	}

	// Is it an Ego map datatype?
	if m, ok := args[0].(*datatypes.EgoMap); ok {
		// Make a list of the visible member names
		result := map[string]interface{}{
			datatypes.SizeMDKey:     len(m.Keys()),
			datatypes.TypeMDKey:     m.TypeString(),
			datatypes.BasetypeMDKey: "map[interface{}]interface{}",
		}

		return result, nil
	}

	// Is it an Ego array datatype?
	if m, ok := args[0].(*datatypes.EgoArray); ok {
		// Make a list of the visible member names
		result := map[string]interface{}{
			datatypes.SizeMDKey:     m.Len(),
			datatypes.TypeMDKey:     m.TypeString(),
			datatypes.BasetypeMDKey: "[]interface{}",
		}

		return datatypes.NewStructFromMap(result), nil
	}

	if e, ok := args[0].(*errors.EgoError); ok {
		wrappedError := e.Unwrap()

		if e.Is(errors.UserError) {
			text := datatypes.GetString(e.GetContext())

			return datatypes.NewStructFromMap(map[string]interface{}{
				datatypes.TypeMDKey:     "error",
				datatypes.BasetypeMDKey: "error",
				"error":                 wrappedError.Error(),
				"text":                  text,
			}), nil
		}

		return datatypes.NewStructFromMap(map[string]interface{}{
			datatypes.TypeMDKey:     "error",
			datatypes.BasetypeMDKey: "error",
			"error":                 wrappedError.Error(),
			"text":                  e.Error(),
		}), nil
	}

	typeString, err := Type(s, args)
	if errors.Nil(err) {
		result := map[string]interface{}{
			datatypes.TypeMDKey:     typeString,
			datatypes.BasetypeMDKey: typeString,
		}
		if array, ok := args[0].([]interface{}); ok {
			result[datatypes.SizeMDKey] = len(array)
			types := "nil"

			for _, a := range array {
				ts, _ := Type(s, []interface{}{a})
				tsx := util.GetString(ts)

				if types == "nil" {
					types = tsx
				} else if types != tsx {
					types = "mixed"

					break
				}
			}

			result[datatypes.BasetypeMDKey] = "array"
			result[datatypes.ElementTypesMDKey] = types
		}

		return datatypes.NewStructFromMap(result), nil
	}

	return nil, err
}

func MemStats(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var m runtime.MemStats

	result := map[string]interface{}{}

	runtime.ReadMemStats(&m)

	// For info on each, see: https://golang.org/pkg/runtime/#MemStats

	result["time"] = time.Now().Format("Mon Jan 2 2006 15:04:05 MST")
	result["current"] = bToMb(m.Alloc)
	result["total"] = bToMb(m.TotalAlloc)
	result["system"] = bToMb(m.Sys)
	result["gc"] = int(m.NumGC)

	datatypes.SetMetadata(result, datatypes.TypeMDKey, datatypes.StructType)

	return result, nil
}

func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}

func CurrentSymbolTable(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var result strings.Builder

	depth := 0
	p := s

	for p != nil {
		depth++
		result.WriteString(fmt.Sprintf("%2d:  %s\n", depth, p.Name))
		p = p.Parent
	}

	return result.String(), nil
}
