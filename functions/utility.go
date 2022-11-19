package functions

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Sleep implements util.sleep().
func Sleep(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	duration, err := time.ParseDuration(datatypes.GetString(args[0]))
	if errors.Nil(err) {
		time.Sleep(duration)
	}

	return true, errors.New(err)
}

// ProfileGet implements the profile.get() function.
func ProfileGet(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	key := datatypes.GetString(args[0])

	return settings.Get(key), nil
}

// ProfileSet implements the profile.set() function.
func ProfileSet(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err *errors.EgoError

	if len(args) != 2 {
		return nil, errors.New(errors.ErrArgumentCount).In("Set()")
	}

	key := datatypes.GetString(args[0])
	isEgoSetting := strings.HasPrefix(key, "ego.")

	// Quick check here. The key must already exist if it's one of the
	// "system" settings. That is, you can't create an ego.* setting that
	// doesn't exist yet, for example
	if isEgoSetting {
		if !settings.Exists(key) {
			return nil, errors.New(errors.ErrReservedProfileSetting).In("Set()").Context(key)
		}
	}

	// Additionally, we don't allow anyone to change runtime, compiler, or server settings from Ego code

	mode := "interactive"
	if modeValue, found := symbols.Get("__exec_mode"); found {
		mode = datatypes.GetString(modeValue)
	}

	if mode != "test" &&
		(strings.HasPrefix(key, "ego.runtime") ||
			strings.HasPrefix(key, "ego.server") ||
			strings.HasPrefix(key, "ego.compiler")) {
		return nil, errors.New(errors.ErrReservedProfileSetting).In("Set()").Context(key)
	}

	// If the value is an empty string, delete the key else
	// store the value for the key.
	value := datatypes.GetString(args[1])
	if value == "" {
		err = settings.Delete(key)
	} else {
		settings.Set(key, value)
	}

	// Ego settings can only be updated in the in-memory copy, not in the persisted data.
	if isEgoSetting {
		return err, nil
	}

	// Otherwise, store the value back to the file system.
	return err, settings.Save()
}

// ProfileDelete implements the profile.delete() function. This just calls
// the set operation with an empty value, which results in a delete operatinon.
// The consolidates the persmission checking, etc. in the Set routine only.
func ProfileDelete(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return ProfileSet(symbols, []interface{}{args[0], ""})
}

// ProfileKeys implements the profile.keys() function.
func ProfileKeys(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	keys := settings.Keys()
	result := make([]interface{}, len(keys))

	for i, key := range keys {
		result[i] = key
	}

	return datatypes.NewArrayFromArray(&datatypes.StringType, result), nil
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
		v := datatypes.Coerce(args[0], "")
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
	v := datatypes.GetString(args[0])

	for range v {
		count++
	}

	return count, nil
}

// GetEnv implements the util.getenv() function which reads
// an environment variable from the os.
func GetEnv(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return os.Getenv(datatypes.GetString(args[0])), nil
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
		keys := datatypes.NewArray(&datatypes.StringType, 0)
		keyList := v.Keys()

		for i, v := range keyList {
			_ = keys.Set(i, v)
		}

		_ = keys.Sort()

		return keys, nil

	case *datatypes.EgoStruct:
		return v.FieldNamesArray(), nil

	case datatypes.EgoPackage:
		keys := datatypes.NewArray(&datatypes.StringType, 0)

		for _, k := range v.Keys() {
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

// Exit implements the os.exit() function.
func Exit(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	// If no arguments, just do a simple exit
	if len(args) == 0 {
		os.Exit(0)
	}

	switch v := args[0].(type) {
	case bool, byte, int32, int, int64, float32, float64:
		os.Exit(datatypes.GetInt(args[0]))

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
	kind := &datatypes.InterfaceType

	for i, j := range args {
		if array, ok := j.(*datatypes.EgoArray); ok && i == 0 {
			if !kind.IsInterface() {
				if err := array.Validate(kind); !errors.Nil(err) {
					return nil, err
				}
			}

			result = append(result, array.BaseArray()...)

			if kind.IsInterface() {
				kind = array.ValueType()
			}
		} else if array, ok := j.([]interface{}); ok && i == 0 {
			result = append(result, array...)
		} else {
			if !kind.IsInterface() && !datatypes.TypeOf(j).IsType(kind) {
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
		r = datatypes.NewArray(&datatypes.StringType, 0)
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

func Packages(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	packages := datatypes.NewArray(&datatypes.StringType, 0)

	for k := range s.Symbols {
		if strings.HasPrefix(k, "__") {
			continue
		}

		v, _ := s.Get(k)

		if datatypes.TypeOf(v).Name() == k {
			packages.Append(k)
		}
	}

	if s.Parent != nil {
		px, _ := Packages(s.Parent, args)
		if pa, ok := px.(*datatypes.EgoArray); ok {
			packages.Append(pa)
		}
	}

	_ = packages.Sort()

	return packages, nil
}
