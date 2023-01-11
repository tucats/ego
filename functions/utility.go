package functions

import (
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Sleep implements util.sleep().
func Sleep(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	duration, err := time.ParseDuration(data.String(args[0]))
	if err == nil {
		time.Sleep(duration)
	} else {
		err = errors.NewError(err)
	}

	return true, err
}

// ProfileGet implements the profile.get() function.
func ProfileGet(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	key := data.String(args[0])

	return settings.Get(key), nil
}

// ProfileSet implements the profile.set() function.
func ProfileSet(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var err error

	if len(args) != 2 {
		return nil, errors.ErrArgumentCount.In("Set()")
	}

	key := data.String(args[0])
	isEgoSetting := strings.HasPrefix(key, "ego.")

	// Quick check here. The key must already exist if it's one of the
	// "system" settings. That is, you can't create an ego.* setting that
	// doesn't exist yet, for example
	if isEgoSetting {
		if !settings.Exists(key) {
			return nil, errors.ErrReservedProfileSetting.In("Set()").Context(key)
		}
	}

	// Additionally, we don't allow anyone to change runtime, compiler, or server settings from Ego code

	mode := "interactive"
	if modeValue, found := symbols.Get("__exec_mode"); found {
		mode = data.String(modeValue)
	}

	if mode != "test" &&
		(strings.HasPrefix(key, "ego.runtime") ||
			strings.HasPrefix(key, "ego.server") ||
			strings.HasPrefix(key, "ego.compiler")) {
		return nil, errors.ErrReservedProfileSetting.In("Set()").Context(key)
	}

	// If the value is an empty string, delete the key else
	// store the value for the key.
	value := data.String(args[1])
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
func ProfileDelete(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return ProfileSet(symbols, []interface{}{args[0], ""})
}

// ProfileKeys implements the profile.keys() function.
func ProfileKeys(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	keys := settings.Keys()
	result := make([]interface{}, len(keys))

	for i, key := range keys {
		result[i] = key
	}

	return data.NewArrayFromArray(&data.StringType, result), nil
}

// Length implements the len() function.
func Length(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if args[0] == nil {
		return 0, nil
	}

	switch arg := args[0].(type) {
	// For a channel, it's length either zero if it's drained, or bottomless
	case *data.Channel:
		size := int(math.MaxInt32)
		if arg.IsEmpty() {
			size = 0
		}

		return size, nil

	case *data.Array:
		return arg.Len(), nil

	case error:
		return len(arg.Error()), nil

	case *data.Map:
		return len(arg.Keys()), nil

	case *data.Package:
		return nil, errors.ErrInvalidType.Context(data.TypeOf(arg).String())

	case nil:
		return 0, nil

	default:
		v := data.Coerce(args[0], "")
		if v == nil {
			return 0, nil
		}

		return len(v.(string)), nil
	}
}

// StrLen is the strings.Length() function, which counts characters/runes instead of
// bytes like len() does.
func StrLen(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	count := 0
	v := data.String(args[0])

	for range v {
		count++
	}

	return count, nil
}

// GetEnv implements the util.getenv() function which reads
// an environment variable from the os.
func GetEnv(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return os.Getenv(data.String(args[0])), nil
}

// GetMode implements the util.Mode() function which reports the runtime mode.
func GetMode(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	m, ok := symbols.Get("__exec_mode")
	if !ok {
		m = "run"
	}

	return m, nil
}

// Members gets an array of the names of the fields in a structure.
func Members(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	switch v := args[0].(type) {
	case *data.Map:
		keys := data.NewArray(&data.StringType, 0)
		keyList := v.Keys()

		for i, v := range keyList {
			_ = keys.Set(i, v)
		}

		_ = keys.Sort()

		return keys, nil

	case *data.Struct:
		return v.FieldNamesArray(), nil

	case *data.Package:
		keys := data.NewArray(&data.StringType, 0)

		for _, k := range v.Keys() {
			if !strings.HasPrefix(k, data.MetadataPrefix) {
				keys.Append(k)
			}
		}

		err := keys.Sort()

		return keys, err

	default:
		return nil, errors.ErrInvalidType.In("members()")
	}
}

// Exit implements the os.exit() function.
func Exit(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// If no arguments, just do a simple exit
	if len(args) == 0 {
		os.Exit(0)
	}

	switch v := args[0].(type) {
	case bool, byte, int32, int, int64, float32, float64:
		os.Exit(data.Int(args[0]))

	case string:
		return nil, errors.NewMessage(v)

	default:
		os.Exit(0)
	}

	return nil, nil
}

// Signal creates an error object based on the
// parameters.
func Signal(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	r := errors.ErrUserDefined
	if len(args) > 0 {
		r = r.Context(args[0])
	}

	return r, nil
}

// Append implements the builtin append() function, which concatenates all the items
// together as an array. The first argument is flattened into the result, and then each
// additional argument is added to the array as-is.
func Append(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	result := make([]interface{}, 0)
	kind := &data.InterfaceType

	for i, j := range args {
		if array, ok := j.(*data.Array); ok && i == 0 {
			if !kind.IsInterface() {
				if err := array.Validate(kind); err != nil {
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
			if !kind.IsInterface() && !data.TypeOf(j).IsType(kind) {
				return nil, errors.ErrWrongArrayValueType.In("append()")
			}
			result = append(result, j)
		}
	}

	return data.NewArrayFromArray(kind, result), nil
}

// Delete can be used three ways. To delete a member from a structure, to delete
// an element from an array by index number, or to delete a symbol entirely. The
// first form requires a string name, the second form requires an integer index,
// and the third form does not have a second parameter.
func Delete(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if _, ok := args[0].(string); ok {
		if len(args) != 1 {
			return nil, errors.ErrArgumentCount.In("delete{}")
		}
	} else {
		if len(args) != 2 {
			return nil, errors.ErrArgumentCount.In("delete{}")
		}
	}

	switch v := args[0].(type) {
	case string:
		return nil, s.Delete(v, false)

	case *data.Map:
		_, err := v.Delete(args[1])

		return v, err

	case *data.Array:
		i := data.Int(args[1])
		err := v.Delete(i)

		return v, err

	default:
		return nil, errors.ErrInvalidType.In("delete()")
	}
}

// GetArgs implements util.Args() which fetches command-line arguments from
// the Ego command invocation, if any.
func GetArgs(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	r, found := s.Get("__cli_args")
	if !found {
		r = data.NewArray(&data.StringType, 0)
	}

	return r, nil
}

// Make implements the make() function. The first argument must be a model of the
// array type (using the Go native version), and the second argument is the size.
func Make(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	kind := args[0]
	size := data.Int(args[1])

	// if it's an Ego type, get the model for the type.
	if v, ok := kind.(*data.Type); ok {
		kind = data.InstanceOfType(v)
	} else if egoArray, ok := kind.(*data.Array); ok {
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
	switch v := kind.(type) {
	case *data.Channel:
		return data.NewChannel(size), nil

	case *data.Array:
		return v.Make(size), nil

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

func MemStats(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var m runtime.MemStats

	result := map[string]interface{}{}

	runtime.ReadMemStats(&m)

	// For info on each, see: https://golang.org/pkg/runtime/#MemStats

	result["time"] = time.Now().Format("Mon Jan 2 2006 15:04:05 MST")
	result["current"] = bToMb(m.Alloc)
	result["total"] = bToMb(m.TotalAlloc)
	result["system"] = bToMb(m.Sys)
	result["gc"] = int(m.NumGC)

	return data.NewStructFromMap(result), nil
}

func bToMb(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}

func Packages(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Make the unordered list of all package names defined in all
	// scopes from here. This may include duplicates.
	allNames := makePackageList(s)

	// Scan the list and set values in the map accordingly. This will
	// effectively remove the duplicates.
	var uniqueNames = map[string]bool{}

	for _, name := range allNames {
		uniqueNames[name] = true
	}

	// Now scan over the list of now-unique names and make an Ego array
	// out of the values.
	packages := data.NewArray(&data.StringType, 0)
	for name := range uniqueNames {
		packages.Append(name)
	}

	// Ask the array to sort itself, and return the array as the
	// function value.
	_ = packages.Sort()

	return packages, nil
}

// makePackageList is a helper function that recursively
// scans the symbol table scope tree from the current
// location, and makes a list of all the package names
// defined within the current scope. The result is an
// array of strings, which may contain duplicates as the
// same package may be defined at multiple scope levels.
func makePackageList(s *symbols.SymbolTable) []string {
	var result []string

	// Scan over the symbol table. Skip hidden symbols.
	for _, k := range s.Names() {
		if strings.HasPrefix(k, "__") {
			continue
		}

		// Get the symbol. IF it is a package, add it's name
		// to our list.
		v, _ := s.Get(k)
		if p, ok := v.(*data.Package); ok {
			result = append(result, p.Name())
		}
	}

	// If there is a parent table, repeat the operation
	// with the parent table, appending those results to
	// our own.
	if s.Parent() != nil {
		px := makePackageList(s.Parent())
		if len(px) > 0 {
			result = append(result, px...)
		}
	}

	return result
}
