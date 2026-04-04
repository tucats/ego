package data

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

// Parameter describes one parameter in a function declaration.
type Parameter struct {
	// Name is the variable name used in the function prototype, e.g. "count"
	// in "func Repeat(count int) string".
	Name string

	// Sandboxed is true when the parameter represents a file path that must
	// be validated against the sandbox root before the function may use it.
	// This prevents Ego code from accessing files outside the permitted area.
	Sandboxed bool

	// Type is the expected Ego type for this parameter.
	Type *Type
}

// Range is a pair of integers [min, max].  It is used to express the minimum
// and maximum number of arguments accepted by a function that can be called
// with a variable number of arguments.
type Range [2]int

// Declaration holds all the static metadata that describes a function:
// its name, receiver type (if any), parameter list, return types, and flags.
// Both compiled Ego functions and native Go builtin functions carry a
// Declaration so the runtime can validate calls and format error messages
// consistently.
type Declaration struct {
	// Name is the function identifier, e.g. "Sprintf".
	Name string

	// Type is the receiver type for a method declaration, or nil for a
	// plain function.  For example, the declaration for (*strings.Builder).WriteString
	// would have Type pointing to the Builder type.
	Type *Type

	// Parameters lists the expected arguments in order.
	Parameters []Parameter

	// Returns lists the type of each return value.  A void function has an
	// empty slice.  A function returning (int, error) has a two-element slice.
	Returns []*Type

	// Variadic is true when the last parameter accepts any number of values
	// (...T notation in Go).
	Variadic bool

	// Scope is true when the function needs direct access to the caller's
	// symbol table.  This is rare and reserved for functions like sort.Slice
	// whose comparison closure must see caller-local variables.
	Scope bool

	// ArgCount overrides the parameter count check when its elements are both
	// non-zero.  ArgCount[0] is the minimum number of arguments; ArgCount[1]
	// is the maximum.  When both are zero the count must exactly match
	// len(Parameters).
	ArgCount Range
}

// BuiltinsDictionary is a global map from function name to its Declaration.
// It is populated at startup by calls to RegisterDeclaration from each
// builtin package.  The dictionary is consulted when formatting a builtin
// function value (e.g. for error messages or the reflect package) because
// native Go function values do not carry their own declaration.
var BuiltinsDictionary = map[string]*Declaration{}

// dictionaryMutex serializes concurrent reads and writes to BuiltinsDictionary.
// Multiple goroutines may call RegisterDeclaration (e.g. during parallel
// import) so we need a mutex even though the map is package-level.
var dictionaryMutex sync.Mutex

// RegisterDeclaration stores a Declaration in BuiltinsDictionary.  It is
// called once per builtin function, typically from an init() function in the
// package that provides the function.
func RegisterDeclaration(d *Declaration) {
	if d == nil {
		ui.Log(ui.InternalLogger, "runtime.nil.func", nil)

		return
	}

	dictionaryMutex.Lock()
	defer dictionaryMutex.Unlock()

	BuiltinsDictionary[d.Name] = d
}

// GetBuiltinDeclaration looks up a builtin declaration by name.  It tries an
// exact-case match first, then falls back to a lowercase comparison so that
// callers need not worry about capitalisation.  Returns nil if the name is not
// registered.
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

// String formats a Declaration as a valid Ego function signature, e.g.
// "(b *Builder) WriteString(s string) int".  The three helper functions below
// build the receiver, parameter, and return-type fragments separately and then
// concatenate them.
func (f *Declaration) String() string {
	if f == nil {
		return defs.NilTypeString
	}

	return f.typeAsString() + f.Name + f.parametersAsString() + f.returnsAsString()
}

// returnsAsString formats the return-type list.  A single return type is
// written without parentheses; multiple return types are parenthesized and
// comma-separated, matching Go syntax.
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

// parametersAsString formats the parameter list between parentheses.
// Optional parameters (when ArgCount is set) are enclosed in square brackets.
// The last parameter gets a "..." suffix when Variadic is true.
func (f *Declaration) parametersAsString() string {
	r := strings.Builder{}

	r.WriteRune('(')

	// variable is true when the function accepts an optional parameter range
	// (ArgCount[0] or ArgCount[1] are non-zero).
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

// typeAsString formats the receiver clause "(varName *TypeName) " that
// precedes method names.  The receiver variable name is derived from the first
// (lowercased) letter of the type name, matching Go convention.
func (f *Declaration) typeAsString() string {
	r := strings.Builder{}

	if f.Type != nil {
		ptr := ""
		ft := f.Type

		// If the receiver is a pointer type (*T), note the "*" and unwrap to T.
		if ft.kind == PointerKind {
			ptr = "*"
			ft = ft.valueType
		}

		if ft.name == "" {
			return "(BOGUS RECEIVER TYPE FOR FUNCTION " + f.Name + ") "
		}

		// Derive the idiomatic one-letter receiver variable name from the
		// type name.  For "strings.Builder" the result is "b"; for "Point"
		// it is "p".
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

// ConformingDeclarations returns true if two function declarations are
// structurally identical: same number of parameters, same parameter types
// (in order), and same number and types of return values.
//
// This is used to verify that a concrete type implements an interface —
// every method required by the interface must have a matching (conforming)
// declaration on the type.
func ConformingDeclarations(fd1, fd2 *Declaration) bool {
	// Both declarations must be non-nil; a nil declaration means the
	// function was never properly defined.
	if fd1 == nil || fd2 == nil {
		ui.Log(ui.InternalLogger, "runtime.nil.func.use", nil)

		return false
	}

	if len(fd1.Parameters) != len(fd2.Parameters) {
		return false
	}

	if len(fd1.Returns) != len(fd2.Returns) {
		return false
	}

	// IsType performs Ego's structural type comparison, which handles
	// interface types and type aliases in addition to direct kind equality.
	for index, parm := range fd1.Parameters {
		if !parm.Type.IsType(fd2.Parameters[index].Type) {
			return false
		}
	}

	for index, ret := range fd1.Returns {
		if !ret.IsType(fd2.Returns[index]) {
			return false
		}
	}

	return true
}
