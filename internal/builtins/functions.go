package builtins

import (
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// FunctionDefinition is an element in the function dictionary. This
// defines each function that is implemented as native Go code (a
// "builtin" function).
type FunctionDefinition struct {
	// Name is the name of the function.
	Name string
	// Package is the package that contains the function, if it is
	// a builtin package member.
	Package string

	// MinArgCount is the minimum number of arguments the function can accept.
	MinArgCount int

	// MaxArgCount is the maximum number of arguments the function can accept.
	MaxArgCount int

	// HasErrReturn is true if the function returns a tuple containing the
	// function result and an error return.
	HasErrReturn bool

	// Is this function entry only allowed when language extensions are
	// enabled?
	Extension bool

	// FullScope indicates if this function is allowed to access the
	// entire scope tree of the running program.
	FullScope bool

	// FunctionAddress is the address of the function implementation
	FunctionAddress any

	// Value is a value constant associated with this name.
	Value any

	// Declaration is a function declaration object that details the
	// parameter and return types.
	Declaration *data.Declaration
}

// Any is a constant that defines that a function can have as many arguments
// as desired.
const Any = math.MaxInt32

// functionDictionaryMu guards all reads and writes to FunctionDictionary.
// BUILTIN-FUNCTIONS-1 fix: FunctionDictionary is a package-level map shared
// across goroutines.  Without a mutex, concurrent calls to AddFunction (or
// AddBuiltins from different goroutines) produce a data race detected by
// Go's race detector.  All exported helpers that iterate or write the map
// must hold the appropriate lock (RLock for reads, Lock for writes).
//
// Note: FunctionDictionary is still an exported variable, so callers that
// access it directly (without using AddFunction / FindFunction / CallBuiltin)
// bypass this protection.  Prefer the accessor functions whenever possible.
var functionDictionaryMu sync.RWMutex

// FunctionDictionary is the dictionary of functions. Each entry in the dictionary
// indicates the min and max argument counts, the native function address, and
// the declaration metadata for the builtin function.
var FunctionDictionary = map[string]FunctionDefinition{
	"$new": {
		MinArgCount:     1,
		MaxArgCount:     1,
		FunctionAddress: NewInstanceOf,
	},
	"new": {
		MinArgCount:     1,
		MaxArgCount:     1,
		FunctionAddress: New,
		Declaration: &data.Declaration{
			Name: "new",
			Parameters: []data.Parameter{
				{
					Name: "type",
					Type: data.PointerType(data.TypeType),
				},
			},
			Returns: []*data.Type{data.PointerType(data.InterfaceType)},
		},
	},
	"append": {
		MinArgCount:     2,
		MaxArgCount:     Any,
		FunctionAddress: Append,
		Declaration: &data.Declaration{
			Name: "append",
			Parameters: []data.Parameter{
				{
					Name: "array",
					Type: data.ArrayType(data.InterfaceType),
				},
				{
					Name: "item",
					Type: data.InterfaceType,
				},
			},
			Variadic: true,
			Returns:  []*data.Type{data.ArrayType(data.InterfaceType)},
		}},
	"close": {
		MinArgCount:     1,
		MaxArgCount:     1,
		FunctionAddress: Close,
		Declaration: &data.Declaration{
			Name: "close",
			Parameters: []data.Parameter{
				{
					Name: "any",
					Type: data.InterfaceType,
				},
			},
			// close() on a channel returns (wasOpen bool, err error) — see
			// the Close() doc comment in internal/builtins/close.go for why
			// this two-value, catchable-error return exists (BUG-29).
			Returns: []*data.Type{data.BoolType, data.ErrorType},
		}},
	"delete": {
		MinArgCount:     1,
		MaxArgCount:     2,
		FunctionAddress: Delete,
		FullScope:       true,
		Declaration: &data.Declaration{
			Name: "delete",
			Parameters: []data.Parameter{
				{
					Name: "item",
					Type: data.InterfaceType,
				},
				{
					Name: "index",
					Type: data.InterfaceType,
				},
			},
			ArgCount: data.Range{1, 2},
		}},
	"index": {
		MinArgCount:     2,
		MaxArgCount:     2,
		FunctionAddress: Index,
		Declaration: &data.Declaration{
			Name: "index",
			Parameters: []data.Parameter{
				{
					Name: "item",
					Type: data.InterfaceType,
				},
				{
					Name: "index",
					Type: data.InterfaceType,
				},
			},
			Returns: []*data.Type{data.IntType},
		}},
	"len": {
		MinArgCount:     1,
		MaxArgCount:     1,
		FunctionAddress: Length,
		Declaration: &data.Declaration{
			Name: "len",
			Parameters: []data.Parameter{
				{
					Name: "item",
					Type: data.InterfaceType,
				},
			},
			Returns: []*data.Type{data.IntType},
		}},
	"make": {
		MinArgCount:     1,
		MaxArgCount:     3,
		FunctionAddress: Make,
		Declaration: &data.Declaration{
			Name:     "make",
			Variadic: true,
			ArgCount: data.Range{1, 3},
			Parameters: []data.Parameter{
				{
					Name: "t",
					Type: data.TypeType,
				},
				{
					Name: "size",
					Type: data.IntType,
				},
				{
					Name: "capacity",
					Type: data.IntType,
				},
			},
			// make() returns a map, array or channel. InterfaceType is used
			// here because the actual return type depends on the first argument
			// at runtime.
			Returns: []*data.Type{data.InterfaceType},
		}},
	"sizeof": {
		Extension:       true,
		MinArgCount:     1,
		MaxArgCount:     1,
		FunctionAddress: SizeOf,
		Declaration: &data.Declaration{
			Name: "sizeof",
			Parameters: []data.Parameter{
				{
					Name: "item",
					Type: data.InterfaceType,
				},
			},
			Returns: []*data.Type{data.IntType},
		},
	},
	"typeof": {
		Extension:       true,
		MinArgCount:     1,
		MaxArgCount:     1,
		FunctionAddress: typeOf,
		Declaration: &data.Declaration{
			Name: "typeof",
			Parameters: []data.Parameter{
				{
					Name: "item",
					Type: data.InterfaceType,
				},
			},
			Returns: []*data.Type{data.TypeType},
		},
	},
}

// AddBuiltins adds or overrides the default function library in the symbol map.
// Function names are distinct in the map because they always have the "()"
// suffix for the key.
func AddBuiltins(symbolTable *symbols.SymbolTable) {
	ui.Log(ui.PackageLogger, "pkg.builtins.table", ui.A{
		"name": symbolTable.Name})

	extensions := settings.GetBool(defs.ExtensionsEnabledSetting)

	// Hold a read lock while we snapshot the keys so that concurrent calls
	// to AddFunction do not race with this iteration (BUILTIN-FUNCTIONS-1).
	functionDictionaryMu.RLock()
	functionNames := make([]string, 0, len(FunctionDictionary))

	for k := range FunctionDictionary {
		functionNames = append(functionNames, k)
	}

	functionDictionaryMu.RUnlock()

	sort.Strings(functionNames)

	for _, functionName := range functionNames {
		// Re-acquire the read lock for each lookup so that individual entries
		// can be added concurrently without holding the lock across the entire loop.
		functionDictionaryMu.RLock()
		functionDefinition := FunctionDictionary[functionName]
		functionDictionaryMu.RUnlock()

		if functionDefinition.Extension && !extensions {
			continue
		}

		if functionDefinition.Declaration != nil {
			data.RegisterDeclaration(functionDefinition.Declaration)
		}

		_ = symbolTable.SetWithAttributes(functionName, functionDefinition.FunctionAddress, symbols.SymbolAttribute{Readonly: true})
	}
}

// FindFunction returns the function definition associated with the
// provided function pointer, if one is found.
func FindFunction(f func(*symbols.SymbolTable, data.List) (any, error)) *FunctionDefinition {
	sf1 := reflect.ValueOf(f)

	// Hold a read lock for the duration of the linear search so that
	// concurrent AddFunction calls do not race with the iteration.
	functionDictionaryMu.RLock()
	defer functionDictionaryMu.RUnlock()

	for _, d := range FunctionDictionary {
		if d.FunctionAddress != nil { // Only function entry points have an F value
			sf2 := reflect.ValueOf(d.FunctionAddress)
			if sf1.Pointer() == sf2.Pointer() {
				return &d
			}
		}
	}

	return nil
}

// FindName returns the name of a function from the dictionary if one is found.
func FindName(f func(*symbols.SymbolTable, data.List) (any, error)) string {
	sf1 := reflect.ValueOf(f)

	// Hold a read lock for the duration of the linear search (BUILTIN-FUNCTIONS-1).
	functionDictionaryMu.RLock()
	defer functionDictionaryMu.RUnlock()

	for name, d := range FunctionDictionary {
		if d.FunctionAddress != nil {
			sf2 := reflect.ValueOf(d.FunctionAddress)
			if sf1.Pointer() == sf2.Pointer() {
				return name
			}
		}
	}

	return ""
}

// CallBuiltin calls a native Ego function by name, with supplied arguments. A native
// function is one written in Go and contained within the Ego image (as opposed to an
// Ego function written in Ego and compiled to bytecode). The argument count is validated
// but the argument types are assumed to be correct for the function.
//
// CallBuiltin returns the result of the function call, or an error if the call fails.
//
// Parameters:
//
//	s			The current runtime symbol table.
//	name		The name of the function to call.
//	args		The arguments to pass to the function.
//
// Returns:
//
//	result		The result of the function call, or nil if the call fails.
//	error:		An error if the call fails, or nil if the function succeeds.
func CallBuiltin(s *symbols.SymbolTable, name string, args ...any) (any, error) {
	// See if it's a runtime package item (as opposed to a builtin). If so, extract the function
	// value and call the function.
	if dot := strings.Index(name, "."); dot > 0 {
		packageName := name[:dot]
		functionName := name[dot+1:]

		if v, ok := s.Get(packageName); ok {
			if pkg, ok := v.(*data.Package); ok {
				if v, ok := pkg.Get(functionName); ok {
					if fd, ok := v.(data.Function); ok {
						if fn, ok := fd.Value.(func(*symbols.SymbolTable, data.List) (any, error)); ok {
							v, e := fn(s, data.NewList(args...))

							return v, e
						}
					}
				}
			}
		}
	}

	// Nope, see if it's a builtin or local function.
	// Hold a read lock while looking up the entry (BUILTIN-FUNCTIONS-1).
	functionDictionaryMu.RLock()
	functionDefinition, found := FunctionDictionary[name]
	functionDictionaryMu.RUnlock()

	if !found {
		return nil, errors.ErrInvalidFunctionName.Context(name)
	}

	// Validate the argument count.
	if len(args) < functionDefinition.MinArgCount || len(args) > functionDefinition.MaxArgCount {
		return nil, errors.ErrPanic.Context(i18n.E("arg.count"))
	}

	// Verify it's a built-in function pointer type. If not, this was a bogus call.
	fn, ok := functionDefinition.FunctionAddress.(func(*symbols.SymbolTable, data.List) (any, error))
	if !ok {
		err := errors.Message(i18n.E("function.ptr",
			map[string]any{"ptr": functionDefinition.FunctionAddress}))

		return nil, errors.ErrPanic.Context(err)
	}

	// Use the function pointer to call the function.
	return fn(s, data.NewList(args...))
}

// AddFunction adds a function definition to the dictionary of known built-in functions.
// This dictionary is used to resolve Ego function calls by name, and to access the function
// definition information. This is called once for each function added to the dictionary.
func AddFunction(s *symbols.SymbolTable, fd FunctionDefinition) error {
	// Acquire the write lock for the collision check AND the subsequent insert so
	// that neither operation races with a concurrent read (BUILTIN-FUNCTIONS-1).
	// The check-then-set pair must be atomic to prevent two goroutines from both
	// passing the collision check and then writing the same key.
	functionDictionaryMu.Lock()

	_, alreadyExists := FunctionDictionary[fd.Name]
	if !alreadyExists {
		FunctionDictionary[fd.Name] = fd
	}

	functionDictionaryMu.Unlock()

	if alreadyExists {
		return errors.ErrFunctionAlreadyExists
	}

	// Has the package already been constructed? If so, we need to add this to the package.
	if pkg, ok := s.Get(fd.Package); ok {
		if p, ok := pkg.(*data.Package); ok {
			p.Set(fd.Name, fd.FunctionAddress)
		}
	}

	return nil
}

// extensions retrieves the boolean indicating if extensions are supported. This can
// be used to do runtime checks for extended features of builtins.
func extensions() bool {
	var (
		err error
		f   bool
	)

	if v, ok := symbols.RootSymbolTable.Get(defs.ExtensionsVariable); ok {
		f, err = data.Bool(v)
		if err != nil {
			ui.Log(ui.InternalLogger, "runtime.extensions.error", ui.A{
				"name":  defs.ExtensionsVariable,
				"error": err})
		}
	}

	return f
}
