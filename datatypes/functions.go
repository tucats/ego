package datatypes

import (
	"reflect"
	"strings"

	"github.com/tucats/ego/defs"
)

type FunctionParameter struct {
	Name     string
	ParmType Type
}

type FunctionDeclaration struct {
	Name        string
	Parameters  []FunctionParameter
	ReturnTypes []Type
}

// dictionary is a descriptive dictionary that shows the declaration string for
// built-in functions. These are used when you attempt to format a function
// that is a builtin (as opposed to compiled) function.  Note that this data
// MUST be kept in sync with the function definitions in the functions package.
var dictionary = map[string]string{
	"functions.Append": "append( any []interface{}, item... interface{}) []interface{}",
	"functions.Length": "len(any interface{}) int",
	"functions.Min":    "Min(item... interface{}) interface{}",
	"functions.Max":    "Max(item... interface{}) interface{}",
	"functions.Random": "Random(maximumValue int) int",
	"functions.Sqrt":   "Sqrt(value float64) float64",
	"functions.Sum":    "Sum(item... int) int",
}

func GetBuiltinDeclaration(name string) string {
	return dictionary[name]
}

func (f FunctionDeclaration) String() string {
	r := strings.Builder{}
	r.WriteString(f.Name)
	r.WriteRune('(')

	for i, p := range f.Parameters {
		if i > 0 {
			r.WriteString(", ")
		}

		r.WriteString(p.Name)
		r.WriteRune(' ')
		r.WriteString(p.ParmType.String())
	}

	r.WriteRune(')')

	if len(f.ReturnTypes) > 0 {
		r.WriteRune(' ')

		if len(f.ReturnTypes) > 1 {
			r.WriteRune('(')
		}

		for i, p := range f.ReturnTypes {
			if i > 0 {
				r.WriteRune(',')
			}

			r.WriteString(p.String())
		}

		if len(f.ReturnTypes) > 1 {
			r.WriteRune(')')
		}
	}

	return r.String()
}

// GetDeclaration returns the embedded function declaration from a
// bytecode stream, if any. It has to use reflection (ick) to do this
// because my package structure needs work and I haven't found a way to
// do this without creating import cycles.
func GetDeclaration(bc interface{}) *FunctionDeclaration {
	vv := reflect.ValueOf(bc)
	ts := vv.String()

	// If it's a bytecode.Bytecode pointer, use reflection to get the
	// Name field value and use that with the name. A function literal
	// will have no name.
	if vv.Kind() == reflect.Ptr {
		if ts == defs.ByteCodeReflectionTypeString {
			switch v := bc.(type) {
			default:
				e := reflect.ValueOf(v).Elem()
				fd, _ := e.Field(3).Interface().(*FunctionDeclaration)

				return fd
			}
		}
	}

	return nil
}
