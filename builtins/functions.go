// Package builtins contains the builtin functions native to the
// Ego language. These are distinct from functions found in
// packages. Examples are len() and append().

package builtins

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"

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
	// Pkg is the package that contains the function, if it is
	// a builtin package member.
	Pkg string

	// Min is the minimum number of arguments the function can accept.
	Min int

	// Max is the maximum number of arguments the function can accept.
	Max int

	// ErrReturn is true if the function returns a tuple containing the
	// function result and an error return.
	ErrReturn bool

	// FullScope indicates if this function is allowed to access the
	// entire scope tree of the running program.
	FullScope bool

	// F is the address of the function implementation
	F interface{}

	// V is a value constant associated with this name.
	V interface{}

	// D is a function declaration object that details the
	// parameter and return types.
	D *data.Declaration
}

// Any is a constant that defines that a function can have as many arguments
// as desired.
const Any = math.MaxInt32

// FunctionDictionary is the dictionary of functions. As functions are determined
// to allow the return of both a value and an error as multi-part results, add the
// ErrReturn:true flag to each function definition.
var FunctionDictionary = map[string]FunctionDefinition{
	"$new": {Min: 1, Max: 1, F: New},
	"append": {Min: 2, Max: Any, F: Append, D: &data.Declaration{
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
	"close": {Min: 1, Max: 1, F: Close, D: &data.Declaration{
		Name: "close",
		Parameters: []data.Parameter{
			{
				Name: "any",
				Type: data.InterfaceType,
			},
		},
	}},
	"delete": {Min: 1, Max: 2, F: Delete, FullScope: true, D: &data.Declaration{
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
	"index": {Min: 2, Max: 2, F: Index, D: &data.Declaration{
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
	"len": {Min: 1, Max: 1, F: Length, D: &data.Declaration{
		Name: "len",
		Parameters: []data.Parameter{
			{
				Name: "item",
				Type: data.InterfaceType,
			},
		},
		Returns: []*data.Type{data.IntType},
	}},
	"make": {Min: 2, Max: 2, F: Make, D: &data.Declaration{
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
	"sizeof": {Min: 1, Max: 1, F: SizeOf, D: &data.Declaration{
		Name: "sizeof",
		Parameters: []data.Parameter{
			{
				Name: "item",
				Type: data.InterfaceType,
			},
		},
		Returns: []*data.Type{data.IntType},
	}},
	"sync.__empty":   {Min: 0, Max: 0, F: stubFunction}, // Package auto imports, but has no functions
	"sync.WaitGroup": {V: sync.WaitGroup{}},
	"sync.Mutex":     {V: sync.Mutex{}},
}

// AddBuiltins adds or overrides the default function library in the symbol map.
// Function names are distinct in the map because they always have the "()"
// suffix for the key.
func AddBuiltins(symbolTable *symbols.SymbolTable) {
	ui.Log(ui.CompilerLogger, "+++ Adding in builtin functions to symbol table %s", symbolTable.Name)

	functionNames := make([]string, 0)
	for k := range FunctionDictionary {
		functionNames = append(functionNames, k)
	}

	sort.Strings(functionNames)

	for _, n := range functionNames {
		d := FunctionDictionary[n]

		if d.D != nil {
			data.RegisterDeclaration(d.D)
		}

		if dot := strings.Index(n, "."); dot >= 0 {
			d.Pkg = n[:dot]
			n = n[dot+1:]
		}

		if d.Pkg == "" {
			_ = symbolTable.SetWithAttributes(n, d.F, symbols.SymbolAttribute{Readonly: true})
		} else {
			// Does package already exist? If not, make it. The package
			// is just a struct containing where each member is a function
			// definition.
			pkg := data.NewPackage(d.Pkg)

			if p, found := symbolTable.Root().Get(d.Pkg); found {
				if pp, ok := p.(*data.Package); ok {
					pkg = pp
				}
			} else {
				ui.Log(ui.CompilerLogger, "    AddBuiltins creating new package %s", d.Pkg)
			}

			root := symbolTable.Root()
			// Is this a value bound to the package, or a function?
			if d.V != nil {
				pkg.Set(n, d.V)

				_ = root.SetWithAttributes(d.Pkg, pkg, symbols.SymbolAttribute{Readonly: true})

				ui.Log(ui.CompilerLogger, "    adding value %s to %s", n, d.Pkg)
			} else {
				pkg.Set(n, d.F)
				pkg.Set(data.TypeMDKey, data.PackageType(d.Pkg))
				pkg.Set(data.ReadonlyMDKey, true)

				_ = root.SetWithAttributes(d.Pkg, pkg, symbols.SymbolAttribute{Readonly: true})

				ui.Log(ui.CompilerLogger, "    adding builtin %s to %s", n, d.Pkg)
			}
		}
	}
}

// FindFunction returns the function definition associated with the
// provided function pointer, if one is found.
func FindFunction(f func(*symbols.SymbolTable, []interface{}) (interface{}, error)) *FunctionDefinition {
	sf1 := reflect.ValueOf(f)

	for _, d := range FunctionDictionary {
		if d.F != nil { // Only function entry points have an F value
			sf2 := reflect.ValueOf(d.F)
			if sf1.Pointer() == sf2.Pointer() {
				return &d
			}
		}
	}

	return nil
}

// FindName returns the name of a function from the dictionary if one is found.
func FindName(f func(*symbols.SymbolTable, []interface{}) (interface{}, error)) string {
	sf1 := reflect.ValueOf(f)

	for name, d := range FunctionDictionary {
		if d.F != nil {
			sf2 := reflect.ValueOf(d.F)
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
						if fn, ok := fd.Value.(func(*symbols.SymbolTable, []interface{}) (interface{}, error)); ok {
							v, e := fn(s, args)

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

	if len(args) < fdef.Min || len(args) > fdef.Max {
		return nil, errors.ErrPanic.Context(i18n.E("arg.count"))
	}

	fn, ok := fdef.F.(func(*symbols.SymbolTable, []interface{}) (interface{}, error))
	if !ok {
		return nil, errors.ErrPanic.Context(fmt.Errorf(i18n.E("function.pointer",
			map[string]interface{}{"ptr": fdef.F})))
	}

	return fn(s, args)
}

func AddFunction(s *symbols.SymbolTable, fd FunctionDefinition) error {
	// Make sure not a collision
	if _, ok := FunctionDictionary[fd.Name]; ok {
		return errors.ErrFunctionAlreadyExists
	}

	FunctionDictionary[fd.Name] = fd

	// Has the package already been constructed? If so, we need to add this to the package.
	if pkg, ok := s.Get(fd.Pkg); ok {
		if p, ok := pkg.(*data.Package); ok {
			p.Set(fd.Name, fd.F)
		}
	}

	return nil
}

func stubFunction(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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
