package fmt

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Printf implements fmt.printf() and is a wrapper around the native Go function.
func Printf(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	length := 0

	str, err := Sprintf(s, args)
	if err == nil {
		length, _ = fmt.Printf("%s", data.String(str))
	}

	return length, err
}

// Sprintf implements fmt.sprintf() and is a wrapper around the native Go function.
func Sprintf(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return 0, nil
	}

	fmtString := data.String(args[0])

	if len(args) == 1 {
		return fmtString, nil
	}

	return fmt.Sprintf(fmtString, args[1:]...), nil
}

// Print implements fmt.Print() and is a wrapper around the native Go function.
func Print(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var b strings.Builder

	for i, v := range args {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(FormatAsString(s, v))
	}

	text, e2 := fmt.Printf("%s", b.String())

	if e2 != nil {
		e2 = errors.NewError(e2)
	}

	return text, e2
}

// Println implements fmt.Println() and is a wrapper around the native Go function.
func Println(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var b strings.Builder

	for i, v := range args {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(FormatAsString(s, v))
	}

	text, e2 := fmt.Printf("%s\n", b.String())

	if e2 != nil {
		e2 = errors.NewError(e2)
	}

	return text, e2
}

// FormatAsString will attempt to use the String() function of the
// object type passed in, if it is a typed struct.  Otherwise, it
// just returns the Unquoted format value.
func FormatAsString(s *symbols.SymbolTable, v interface{}) string {
	if m, ok := v.(*data.Struct); ok {
		if f := m.GetType().Function("String"); f != nil {
			if fmt, ok := f.(func(s *symbols.SymbolTable, args []interface{}) (interface{}, error)); ok {
				local := symbols.NewChildSymbolTable("local to format", s)
				local.SetAlways(defs.ThisVariable, v)

				if si, err := fmt(local, []interface{}{}); err == nil {
					if str, ok := si.(string); ok {
						return str
					}
				}
			}
		}
	}

	return data.FormatUnquoted(v)
}
