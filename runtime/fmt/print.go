// Package fmt implements the Ego standard library "fmt" package, providing
// formatted I/O functions similar to Go's fmt package. Because Ego uses its
// own type system (data.Array, data.Struct, data.Map, etc.) instead of Go's
// reflection-based types, some format verbs behave differently:
//
//   - %T prints the Ego type name (e.g. "int", "[]string") instead of the
//     Go runtime type name.
//   - %V prints the value wrapped with its Ego type (e.g. "int(42)").
//   - %#v is silently replaced with %v because Ego cannot produce Go syntax
//     representations of its composite types.
//
// The print.go file contains the Print/Printf/Println/Sprintf implementations.
// The scan.go file contains the Scan/Sscanf implementations and the core
// scanner helper.
// The types.go file registers all functions in the FmtPackage variable, which
// is loaded when an Ego program imports "fmt".
package fmt

import (
	"fmt"
	"io"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// printFormat implements fmt.Printf() and is a wrapper around the native Go function.
func printFormat(s *symbols.SymbolTable, args data.List) (any, error) {
	length := 0

	str, err := stringPrintFormat(s, args)
	if err == nil {
		if writer, found := s.Get(defs.StdoutWriterSymbol); found {
			if writer, ok := writer.(io.Writer); ok {
				return writer.Write([]byte(data.String(str)))
			}
		}

		length, _ = fmt.Printf("%s", data.String(str))
	}

	return length, err
}

// stringPrintFormat implements fmt.Sprintf() and is a wrapper around the native Go function.
//
// It preprocesses the format string before passing it to Go's fmt.Sprintf to
// handle Ego-specific verbs:
//
//   - %#v is replaced with %v (Go's "Go-syntax" representation is unsupported).
//   - %T is replaced with %s and the corresponding argument is replaced with
//     the Ego type name of that argument (e.g. "int", "string", "[]float64").
//   - %V is replaced with %s and the argument is replaced with a type-decorated
//     string (e.g. int(42)).
func stringPrintFormat(s *symbols.SymbolTable, args data.List) (any, error) {
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
	//
	// count tracks how many consumed format verbs have been seen so far,
	// which tells us which argument in args corresponds to the current verb.
	// args[0] is the format string itself, so the first verb maps to args[1].
	//
	// skipNext is set when the first % of a %% pair is seen; the second %
	// must not be counted as a verb (it produces a literal percent output).
	count := 0
	skipNext := false

	for pos, ch := range fmtString {
		if skipNext {
			skipNext = false

			continue
		}

		if ch == '%' {
			next := ""
			if pos+1 < len(fmtString) {
				next = fmtString[pos+1 : pos+2]
			}

			if next == "%" {
				// %% is a literal percent — skip both characters, count nothing.
				skipNext = true

				continue
			}

			count = count + 1

			switch next {
			case "T":
				// Replace the %T with %s
				fmtString = fmtString[:pos] + "%s" + fmtString[pos+2:]

				// Replace the value with string representation of the type
				item := args.Get(count)
				itemType := data.TypeOf(item)
				typeName := itemType.TypeString()
				args.Set(count, typeName)
			case "V":
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

// printList implements fmt.Print() and is a wrapper around the native Go function.
// This prints the arguments to the output but with no trailing newline.
func printList(s *symbols.SymbolTable, args data.List) (any, error) {
	var b strings.Builder

	for i, v := range args.Elements() {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(formatUsingString(s, v))
	}

	str := b.String()

	if writer, found := s.Get(defs.StdoutWriterSymbol); found {
		if writer, ok := writer.(io.Writer); ok {
			return writer.Write([]byte(str))
		}
	}

	text, e2 := fmt.Printf("%s", str)

	if e2 != nil {
		e2 = errors.New(e2)
	}

	return text, e2
}

// printLine implements fmt.Println() and is a wrapper around the native Go function.
// This prints the arguments to the output with a trailing newline.
func printLine(s *symbols.SymbolTable, args data.List) (any, error) {
	var b strings.Builder

	for i, v := range args.Elements() {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(formatUsingString(s, v))
	}

	str := b.String()

	if writer, found := s.Get(defs.StdoutWriterSymbol); found {
		if writer, ok := writer.(io.Writer); ok {
			return writer.Write([]byte(str + "\n"))
		}
	}

	text, e2 := fmt.Printf("%s\n", str)

	if e2 != nil {
		e2 = errors.New(e2)
	}

	return text, e2
}

// formatUsingString will attempt to use the String() function of the
// object type passed in, if it is a typed struct. Otherwise, it
// just returns the Unquoted format value.
func formatUsingString(s *symbols.SymbolTable, v any) string {
	if m, ok := v.(*data.Struct); ok {
		if f := m.Type().Function("String"); f != nil {
			if fmt, ok := f.(func(s *symbols.SymbolTable, args data.List) (any, error)); ok {
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
