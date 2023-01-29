package data

import (
	"fmt"
	"strings"
	"sync"
)

// Parameter is used to describe the parameters of a declaration.
type Parameter struct {
	// Name is the name of the parameter (the variable name used
	// in the prototype declaration).
	Name string

	// Type is the type of the parameter.
	Type *Type
}

// Range is a type used to hold a pair of integers.
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
	dictionaryMutex.Lock()
	defer dictionaryMutex.Unlock()

	BuiltinsDictionary[d.Name] = d
}

// GetBuiltinDeclaration retrieves a builtin delaration by name. This is used
// when formatting the function for output, or validating parameters.
func GetBuiltinDeclaration(name string) string {
	dictionaryMutex.Lock()
	defer dictionaryMutex.Unlock()

	if d, found := BuiltinsDictionary[name]; found {
		return d.String()
	}

	return name + "()"
}

// Format a declaration object as an Ego-language compliant human-readable
// string value.
func (f Declaration) String() string {
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

	r.WriteString(f.Name)
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

		r.WriteRune(' ')
		r.WriteString(p.Type.ShortString())
	}

	if variable {
		r.WriteString("]")
	}

	r.WriteRune(')')

	if len(f.Returns) > 0 {
		r.WriteRune(' ')

		if len(f.Returns) > 1 {
			r.WriteRune('(')
		}

		for i, p := range f.Returns {
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
