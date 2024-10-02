package builtins

import (
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/symbols"
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
	FunctionAddress interface{}

	// Value is a value constant associated with this name.
	Value interface{}

	// Declaration is a function declaration object that details the
	// parameter and return types.
	Declaration *data.Declaration
}

// Any is a constant that defines that a function can have as many arguments
// as desired.
const Any = math.MaxInt32

// FunctionDictionary is the dictionary of functions. Each entry in the dictionary
// indicates the min and max argument counts, the native function address, and
// the declaration metadata for the builtin function.
var FunctionDictionary = map[string]FunctionDefinition{
	"$new": {MinArgCount: 1, MaxArgCount: 1, FunctionAddress: New},
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
		MinArgCount:     2,
		MaxArgCount:     2,
		FunctionAddress: Make,
		Declaration: &data.Declaration{
			Name: "make",
			Parameters: []data.Parameter{
				{
					Name: "t",
					Type: data.TypeType,
				},
				{
					Name: "count",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.IntType},
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
	"sync.__empty": {
		MinArgCount:     0,
		MaxArgCount:     0,
		FunctionAddress: stubFunction,
	}, // Package auto imports, but has no functions
	"sync.WaitGroup": {Value: sync.WaitGroup{}},
	"sync.Mutex":     {Value: sync.Mutex{}},
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
	ui.Log(ui.PackageLogger, "+++ Adding in builtin functions to symbol table %s", symbolTable.Name)

	extensions := settings.GetBool(defs.ExtensionsEnabledSetting)

	functionNames := make([]string, 0)
	for k := range FunctionDictionary {
		functionNames = append(functionNames, k)
	}

	sort.Strings(functionNames)

	for _, n := range functionNames {
		d := FunctionDictionary[n]
		if d.Extension && !extensions {
			continue
		}

		if d.Declaration != nil {
			data.RegisterDeclaration(d.Declaration)
		}

		if dot := strings.Index(n, "."); dot >= 0 {
			d.Package = n[:dot]
			n = n[dot+1:]
		}

		if d.Package == "" {
			_ = symbolTable.SetWithAttributes(n, d.FunctionAddress, symbols.SymbolAttribute{Readonly: true})
		} else {
			// Does package already exist? If not, make it. The package
			// is just a struct containing where each member is a function
			// definition.
			pkg := data.NewPackage(d.Package)

			if p, found := symbolTable.Root().Get(d.Package); found {
				if pp, ok := p.(*data.Package); ok {
					pkg = pp
				}
			} else {
				ui.Log(ui.PackageLogger, "    AddBuiltins creating new package %s", d.Package)
			}

			root := symbolTable.Root()
			// Is this a value bound to the package, or a function?
			if d.Value != nil {
				pkg.Set(n, d.Value)

				_ = root.SetWithAttributes(d.Package, pkg, symbols.SymbolAttribute{Readonly: true})

				ui.Log(ui.PackageLogger, "    adding value %s to %s", n, d.Package)
			} else {
				pkg.Set(n, d.FunctionAddress)
				pkg.Set(data.TypeMDKey, data.PackageType(d.Package))
				pkg.Set(data.ReadonlyMDKey, true)

				_ = root.SetWithAttributes(d.Package, pkg, symbols.SymbolAttribute{Readonly: true})

				ui.Log(ui.PackageLogger, "    adding builtin %s to %s", n, d.Package)
			}
		}
	}
}

// FindFunction returns the function definition associated with the
// provided function pointer, if one is found.
func FindFunction(f func(*symbols.SymbolTable, data.List) (interface{}, error)) *FunctionDefinition {
	sf1 := reflect.ValueOf(f)

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
func FindName(f func(*symbols.SymbolTable, data.List) (interface{}, error)) string {
	sf1 := reflect.ValueOf(f)

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

func CallBuiltin(s *symbols.SymbolTable, name string, args ...interface{}) (interface{}, error) {
	// See if it's a runtime package item (as opposed to a builtin)
	if dot := strings.Index(name, "."); dot > 0 {
		packageName := name[:dot]
		functionName := name[dot+1:]

		if v, ok := s.Get(packageName); ok {
			if pkg, ok := v.(*data.Package); ok {
				if v, ok := pkg.Get(functionName); ok {
					if fd, ok := v.(data.Function); ok {
						if fn, ok := fd.Value.(func(*symbols.SymbolTable, data.List) (interface{}, error)); ok {
							v, e := fn(s, data.NewList(args...))

							return v, e
						}
					}
				}
			}
		}
	}

	// Nope, see if it's a builtin

	var fdef = FunctionDefinition{}

	found := false

	for fn, d := range FunctionDictionary {
		if fn == name {
			fdef = d
			found = true
		}
	}

	if !found {
		return nil, errors.ErrInvalidFunctionName.Context(name)
	}

	if len(args) < fdef.MinArgCount || len(args) > fdef.MaxArgCount {
		return nil, errors.ErrPanic.Context(i18n.E("arg.count"))
	}

	fn, ok := fdef.FunctionAddress.(func(*symbols.SymbolTable, data.List) (interface{}, error))
	if !ok {
		err := errors.Message(i18n.E("function.pointer",
			map[string]interface{}{"ptr": fdef.FunctionAddress}))

		return nil, errors.ErrPanic.Context(err)
	}

	return fn(s, data.NewList(args...))
}

func AddFunction(s *symbols.SymbolTable, fd FunctionDefinition) error {
	// Make sure not a collision
	if _, ok := FunctionDictionary[fd.Name]; ok {
		return errors.ErrFunctionAlreadyExists
	}

	FunctionDictionary[fd.Name] = fd

	// Has the package already been constructed? If so, we need to add this to the package.
	if pkg, ok := s.Get(fd.Package); ok {
		if p, ok := pkg.(*data.Package); ok {
			p.Set(fd.Name, fd.FunctionAddress)
		}
	}

	return nil
}

func stubFunction(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	return nil, errors.ErrInvalidFunctionName
}

// extensions retrieves the boolean indicating if extensions are supported. This can
// be used to do runtime checks for etended featues of builtins.
func extensions() bool {
	f := false
	if v, ok := symbols.RootSymbolTable.Get(defs.ExtensionsVariable); ok {
		f = data.Bool(v)
	}

	return f
}
