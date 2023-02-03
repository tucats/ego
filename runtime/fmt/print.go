package fmt

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// printFormat implements fmt.Printf() and is a wrapper around the native Go function.
func printFormat(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	length := 0

	str, err := stringPrintFormat(s, args)
	if err == nil {
		length, _ = fmt.Printf("%s", data.String(str))
	}

	return length, err
}

// stringPrintFormat implements fmt.Sprintf() and is a wrapper around the native Go function.
func stringPrintFormat(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return 0, nil
	}

	fmtString := data.String(args[0])

	if len(args) == 1 {
		return fmtString, nil
	}

	return fmt.Sprintf(fmtString, args[1:]...), nil
}

// printList implements fmt.Println() and is a wrapper around the native Go function.
func printList(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var b strings.Builder

	for i, v := range args {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(formatUsingString(s, v))
	}

	text, e2 := fmt.Printf("%s", b.String())

	if e2 != nil {
		e2 = errors.NewError(e2)
	}

	return text, e2
}

// printLine implements fmt.Println() and is a wrapper around the native Go function.
func printLine(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var b strings.Builder

	for i, v := range args {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(formatUsingString(s, v))
	}

	text, e2 := fmt.Printf("%s\n", b.String())

	if e2 != nil {
		e2 = errors.NewError(e2)
	}

	return text, e2
}

// formatUsingString will attempt to use the String() function of the
// object type passed in, if it is a typed struct.  Otherwise, it
// just returns the Unquoted format value.
func formatUsingString(s *symbols.SymbolTable, v interface{}) string {
	if m, ok := v.(*data.Struct); ok {
		if f := m.Type().Function("String"); f != nil {
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
