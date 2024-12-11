package data

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

// Parameter is used to describe the parameters of a declaration.
type Parameter struct {
	// Name is the name of the parameter (the variable name used
	// in the prototype declaration).
	Name string

	// If this is a string parameter that is a file name, then this
	// indiates that the parameter must have filename sandboxing applied
	// if sandboxing is active.
	Sandboxed bool

	// Type is the type of the parameter.
	Type *Type
}

// Range is a type used to hold a pair of integers. This is most
// commonly used to express minimum and maximum numbers of parameters
// accepted by a variadic function.
type Range [2]int

// Declaration describes a function declaration. This is used for
// compiled (bytecode) functions, as well as defined at runtime for
// the builtin functions and elements of runtime packages.
type Declaration struct {
	// Name is the name of the function.
	Name string

	// Type is the receiver type for a function. This is nil if the
	// declared function is not a receiver function.
	Type *Type

	// Parameters is an array of items describing the parameters to
	// a function in a declaration.
	Parameters []Parameter

	// Returns is an array of types describing the types of each of
	// the return values of a function declaration (if the non-void f
	// function does not return a tuple then this array will be of 1).
	Returns []*Type

	// Variadic is true if the declaration is for a variadic function.
	// The last parameter can be repeated in the function call, and is
	// represented with "..." notation in the parameter list of the
	// declaration.
	Variadic bool

	// Scope is true if the declared function needs access to the symbol
	// table scope of the caller. For example, sort.Slice functions need
	// to be able to see the symbol table of the calling function to get the
	// array name. Most functions do not need this access.
	Scope bool

	// ArgCount describes the minimum and maximum number of arguments. If
	// both items are zero, the argument count must match the size of the
	// Parameters array.
	ArgCount Range
}

// BuiltinsDictionary is a descriptive dictionary that holds the declaration for
// built-in functions. These are used when you attempt to format a builtin
// function (as opposed to compiled or runtime function).
var BuiltinsDictionary = map[string]*Declaration{}

var dictionaryMutex sync.Mutex

// RegisterDeclaration stores a declaration object in the internal declaration
// dictionary. This dictionary is only used for native builtins (such as append
// or make) that do not have a formal declaration already defined.
func RegisterDeclaration(d *Declaration) {
	if d == nil {
		ui.Log(ui.InternalLogger, "Attempt to register nil function declaration")

		return
	}

	dictionaryMutex.Lock()
	defer dictionaryMutex.Unlock()

	BuiltinsDictionary[d.Name] = d
}

// GetBuiltinDeclaration retrieves a builtin delaration by name. This is used
// when formatting the function for output, or validating parameters.
func GetBuiltinDeclaration(name string) *Declaration {
	dictionaryMutex.Lock()
	defer dictionaryMutex.Unlock()

	if d, found := BuiltinsDictionary[name]; found {
		return d
	}

	if d, found := BuiltinsDictionary[strings.ToLower(name)]; found {
		return d
	}

	return nil
}

// Format a declaration object as an Ego-language compliant human-readable
// string value.
func (f *Declaration) String() string {
	if f == nil {
		return defs.NilTypeString
	}

	return f.typeAsString() + f.Name + f.parametersAsString() + f.returnsAsString()
}

func (f *Declaration) returnsAsString() string {
	r := strings.Builder{}

	if len(f.Returns) > 0 {
		r.WriteRune(' ')

		if len(f.Returns) > 1 {
			r.WriteRune('(')
		}

		for i, p := range f.Returns {
			if p == nil {
				panic("Nil return type in function declaration for " + f.Name)
			}

			if i > 0 {
				r.WriteString(", ")
			}

			r.WriteString(p.ShortTypeString())
		}

		if len(f.Returns) > 1 {
			r.WriteRune(')')
		}
	}

	return r.String()
}

func (f *Declaration) parametersAsString() string {
	r := strings.Builder{}

	r.WriteRune('(')

	variable := (f.ArgCount[0] != 0 || f.ArgCount[1] != 0)

	for i, p := range f.Parameters {
		if variable && i == f.ArgCount[1]-1 {
			if i == 0 {
				r.WriteString("[")
			} else {
				r.WriteString(" [")
			}
		}

		if i > 0 {
			r.WriteString(", ")
		}

		r.WriteString(p.Name)

		if f.Variadic && i == len(f.Parameters)-1 {
			r.WriteString("...")
		}

		if p.Type != nil {
			r.WriteRune(' ')
			r.WriteString(p.Type.ShortString())
		}
	}

	if variable {
		r.WriteString("]")
	}

	r.WriteRune(')')

	return r.String()
}

func (f *Declaration) typeAsString() string {
	r := strings.Builder{}

	if f.Type != nil {
		ptr := ""
		ft := f.Type

		if ft.kind == PointerKind {
			ptr = "*"
			ft = ft.valueType
		}

		varName := ft.name[:1]

		if strings.Contains(ft.name, ".") {
			names := strings.Split(ft.name, ".")
			varName = strings.ToLower(names[1][:1])
		} else {
			varName = strings.ToLower(varName)
		}

		typeName := ft.name
		r.WriteString(fmt.Sprintf("(%s %s%s) ", varName, ptr, typeName))
	}

	return r.String()
}

func ConformingDeclarations(fd1, fd2 *Declaration) bool {
	// Both declarations must exist
	if fd1 == nil || fd2 == nil {
		ui.Log(ui.InternalLogger, "Attempt to compare nil function declarations for conformance")

		return false
	}

	// Number of parameters must match.
	if len(fd1.Parameters) != len(fd2.Parameters) {
		return false
	}

	// Number of return values must match.
	if len(fd1.Returns) != len(fd2.Returns) {
		return false
	}

	// Parameter types must match
	for index, parm := range fd1.Parameters {
		if !parm.Type.IsType(fd2.Parameters[index].Type) {
			return false
		}
	}

	// Return types must match
	for index, ret := range fd1.Returns {
		if !ret.IsType(fd2.Returns[index]) {
			return false
		}
	}

	// Everything lines up.
	return true
}
