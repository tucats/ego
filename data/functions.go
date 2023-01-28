package data

import (
	"fmt"
	"strings"
)

type Parameter struct {
	Name string
	Type *Type
}

type Range [2]int

type Declaration struct {
	Name       string
	Type       *Type
	Parameters []Parameter
	Returns    []*Type
	Variadic   bool
	Scope      bool
	ArgCount   Range
}

// dictionary is a descriptive dictionary that shows the declaration string for
// built-in functions. These are used when you attempt to format a function
// that is a builtin (as opposed to compiled) function.  Note that this data
// MUST be kept in sync with the function definitions in the functions package.
var dictionary = map[string]string{
	"functions.Append":   "append( any []interface{}, item... interface{}) []interface{}",
	"functions.CloseAny": "close(any interface{})",
	"functions.Delete":   "delete(map interface{}, key string)",
	"functions.Length":   "len(any interface{}) int",
	"functions.Make":     "make(t type, count int) interface{}",
	"functions.New":      "InstanceOf(any interface{}) interface{}",
	"functions.Sizeof":   "sizeof(any interface{}) int",
	"functions.Signal":   "error(msg string) error",
}

func GetBuiltinDeclaration(name string) string {
	return dictionary[name]
}

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
