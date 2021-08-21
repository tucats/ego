package functions

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
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
			return nil, errors.New(errors.ErrReservedProfileSetting).In("Set()").Context(key)
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

	case datatypes.EgoPackage:
		return nil, errors.New(errors.ErrInvalidType)

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

	case datatypes.EgoPackage:
		keys := datatypes.NewArray(datatypes.StringType, 0)

		for k := range v {
			if !strings.HasPrefix(k, datatypes.MetadataPrefix) {
				keys.Append(k)
			}
		}

		err := keys.Sort()

		return keys, err

	default:
		return nil, errors.New(errors.ErrInvalidType).In("members()")
	}
}

// SortStrings implements the sort.Strings function.
func SortStrings(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsType(datatypes.StringType) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.New(errors.ErrWrongArrayValueType).Context("sort.Strings()")
	}

	return nil, errors.New(errors.ErrArgumentType).Context("sort.Strings()")
}

// SortInts implements the sort.Ints function.
func SortInts(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsType(datatypes.IntType) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.New(errors.ErrWrongArrayValueType).Context("sort.Ints()")
	}

	return nil, errors.New(errors.ErrArgumentType).Context("sort.Ints()")
}

// SortFloats implements the sort.Floats function.
func SortFloats(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsType(datatypes.Float64Type) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.New(errors.ErrWrongArrayValueType).Context("sort.Floats()")
	}

	return nil, errors.New(errors.ErrArgumentType).Context("sort.Floats()")
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
			intArray = append(intArray, datatypes.GetInt(i))
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
			floatArray = append(floatArray, util.GetFloat64(i))
		}

		sort.Float64s(floatArray)

		resultArray := datatypes.NewArray(datatypes.Float64Type, len(array))

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
		return nil, errors.New(errors.ErrInvalidType).In("sort()")
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

// Signal creates an error object based on the
// parameters.
func Signal(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	r := errors.New(errors.ErrUserDefined)
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
				return nil, errors.New(errors.ErrWrongArrayValueType).In("append()")
			}
			result = append(result, j)
		}
	}

	return datatypes.NewArrayFromArray(kind, result), nil
}

// Delete can be used three ways. To delete a member from a structure, to delete
// an element from an array by index number, or to delete a symbol entirely. The
// first form requires a string name, the second form requires an integer index,
// and the third form does not have a second parameter.
func Delete(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if _, ok := args[0].(string); ok {
		if len(args) != 1 {
			return nil, errors.New(errors.ErrArgumentCount).In("delete{}")
		}
	} else {
		if len(args) != 2 {
			return nil, errors.New(errors.ErrArgumentCount).In("delete{}")
		}
	}

	switch v := args[0].(type) {
	case string:
		return nil, s.Delete(v, false)

	case *datatypes.EgoMap:
		_, err := v.Delete(args[1])

		return v, err

	case *datatypes.EgoArray:
		i := datatypes.GetInt(args[1])
		err := v.Delete(i)

		return v, err

	default:
		return nil, errors.New(errors.ErrInvalidType).In("delete()")
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
	size := datatypes.GetInt(args[1])

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
	}

	return array, nil
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

	return datatypes.NewStructFromMap(result), nil
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

func LogTail(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	count := datatypes.GetInt(args[0])
	lines := ui.Tail(count)

	if lines == nil {
		return []interface{}{}, nil
	}

	xLines := make([]interface{}, len(lines))
	for i, j := range lines {
		xLines[i] = j
	}

	return datatypes.NewArrayFromArray(datatypes.StringType, xLines), nil
}
