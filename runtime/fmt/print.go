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
func printFormat(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	length := 0

	str, err := stringPrintFormat(s, args)
	if err == nil {
		length, _ = fmt.Printf("%s", data.String(str))
	}

	return length, err
}

// stringPrintFormat implements fmt.Sprintf() and is a wrapper around the native Go function.
func stringPrintFormat(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if args.Len() == 0 {
		return 0, nil
	}

	fmtString := data.String(args.Get(0))

	if args.Len() == 1 {
		return fmtString, nil
	}

	// We can't support extended %v formats
	fmtString = strings.ReplaceAll(fmtString, "%#v", "%v")

	// Preprocess the format string to find any instances of %T, which
	// we will preprocess to the type name of the object, and replace
	// in the parameter list with the string value.
	count := 0

	for pos, ch := range fmtString {
		if ch == '%' {
			count = count + 1

			if pos < len(fmtString) && fmtString[pos+1:pos+2] == "T" {
				// Replace the %T with %s
				fmtString = fmtString[:pos] + "%s" + fmtString[pos+2:]

				// Replace the value with string representation of the type
				item := args.Get(count)
				itemType := data.TypeOf(item)
				typeName := itemType.TypeString()
				args.Set(count, typeName)
			} else if pos < len(fmtString) && fmtString[pos+1:pos+2] == "V" {
				// Replace the %V with %s
				fmtString = fmtString[:pos] + "%s" + fmtString[pos+2:]

				// Replace the value with string representation of the type
				text := data.FormatWithType(args.Get(count))
				args.Set(count, text)
			}
		}
	}

	// Now, format the remainder of the string using native conversions.
	return fmt.Sprintf(fmtString, args.Elements()[1:]...), nil
}

// printList implements fmt.Println() and is a wrapper around the native Go function.
func printList(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var b strings.Builder

	for i, v := range args.Elements() {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(formatUsingString(s, v))
	}

	text, e2 := fmt.Printf("%s", b.String())

	if e2 != nil {
		e2 = errors.New(e2)
	}

	return text, e2
}

// printLine implements fmt.Println() and is a wrapper around the native Go function.
func printLine(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var b strings.Builder

	for i, v := range args.Elements() {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(formatUsingString(s, v))
	}

	text, e2 := fmt.Printf("%s\n", b.String())

	if e2 != nil {
		e2 = errors.New(e2)
	}

	return text, e2
}

// formatUsingString will attempt to use the String() function of the
// object type passed in, if it is a typed struct.  Otherwise, it
// just returns the Unquoted format value.
func formatUsingString(s *symbols.SymbolTable, v interface{}) string {
	if m, ok := v.(*data.Struct); ok {
		if f := m.Type().Function("String"); f != nil {
			if fmt, ok := f.(func(s *symbols.SymbolTable, args data.List) (interface{}, error)); ok {
				local := symbols.NewChildSymbolTable("local to format", s)
				local.SetAlways(defs.ThisVariable, v)

				if si, err := fmt(local, data.NewList()); err == nil {
					if str, ok := si.(string); ok {
						return str
					}
				}
			}
		}
	}

	return data.FormatUnquoted(v)
}
