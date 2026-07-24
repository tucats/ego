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

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// printFormat implements fmt.Printf() and is a wrapper around the native Go function.
// It supports writing to the stdout writer if active, else uses the local formatter.
func printFormat(s *symbols.SymbolTable, args data.List) (any, error) {
	length := 0

	str, err := stringPrintFormat(s, args)
	if err == nil {
		if writer, found := s.Get(defs.StdoutWriterSymbol); found {
			if writer, ok := writer.(io.Writer); ok {
				length, err = writer.Write([]byte(data.String(str)))
			}
		} else {
			length, err = fmt.Printf("%s", data.String(str))
		}
	}

	return data.NewList(length, err), err
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

// formatPrintArgs builds the string representation of args using the same
// spacing rule Go's real fmt.Print/fmt.Sprint document: a space is inserted
// between two consecutive operands only when NEITHER of them is a string
// (so "a", "b" formats as "ab", but 1, 2 formats as "1 2", and "a", 1
// formats as "a1"). This is deliberately NOT the same rule fmt.Println uses
// (see printLine below), which always inserts a space between every pair of
// operands regardless of type.
//
// Shared by printList (fmt.Print, which writes the result to output) and
// sprintList (fmt.Sprint, which returns it directly) so the two can never
// drift out of sync with each other -- they differ only in what they do
// with the finished string.
func formatPrintArgs(s *symbols.SymbolTable, args data.List) (string, error) {
	var b strings.Builder

	elements := args.Elements()

	for i, v := range elements {
		if i > 0 {
			prevIsString := data.TypeOf(elements[i-1]).IsString()
			curIsString := data.TypeOf(v).IsString()

			if !prevIsString && !curIsString {
				b.WriteString(" ")
			}
		}

		text, err := formatUsingString(s, v)
		if err != nil {
			return "", err
		}

		b.WriteString(text)
	}

	return b.String(), nil
}

// printList implements fmt.Print() and is a wrapper around the native Go function.
// This prints the arguments to the output but with no trailing newline.
func printList(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		length int
		e2     error
	)

	str, err := formatPrintArgs(s, args)
	if err != nil {
		return nil, err
	}

	if writer, found := s.Get(defs.StdoutWriterSymbol); found {
		if writer, ok := writer.(io.Writer); ok {
			length, e2 = writer.Write([]byte(str))
		}
	} else {
		length, e2 = fmt.Printf("%s", str)
	}

	if e2 != nil {
		e2 = errors.New(e2)
	}

	return data.NewList(length, e2), e2
}

// sprintList implements fmt.Sprint() and returns the arguments formatted
// using their default formats, exactly as fmt.Print does, but as a string
// instead of writing to output.
//
// This cannot be a native pass-through to Go's real fmt.Sprint: the native
// argument marshaller only knows how to convert a fixed set of scalar Ego
// types to their Go equivalents, and would fail (or silently mishandle) an
// Ego struct, array, map, or typed-nil value passed through the variadic
// ...any parameter. Instead, per-argument formatting is emulated using the
// same formatUsingString helper Print/Println/Sprintf already share, which
// understands Ego's own runtime value shapes (including calling a value's
// String() method, if it has one) directly.
func sprintList(s *symbols.SymbolTable, args data.List) (any, error) {
	return formatPrintArgs(s, args)
}

// printLine implements fmt.Println() and is a wrapper around the native Go function.
// This prints the arguments to the output with a trailing newline. Unlike
// printList/sprintList (fmt.Print/fmt.Sprint, see formatPrintArgs above),
// Println always inserts a space between every pair of operands regardless
// of type -- that is Go's own documented rule for Println specifically, not
// a simplification, so this loop intentionally does not use formatPrintArgs.
func printLine(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		length int
		e2     error
		b      strings.Builder
	)

	for i, v := range args.Elements() {
		if i > 0 {
			b.WriteString(" ")
		}

		text, err := formatUsingString(s, v)
		if err != nil {
			return "", err
		}

		b.WriteString(text)
	}

	str := b.String()

	if writer, found := s.Get(defs.StdoutWriterSymbol); found {
		if writer, ok := writer.(io.Writer); ok {
			length, e2 = writer.Write([]byte(str + "\n"))
		}
	} else {
		length, e2 = fmt.Printf("%s\n", str)
	}

	if e2 != nil {
		e2 = errors.New(e2)
	}

	return data.NewList(length, e2), e2
}

// formatUsingString will attempt to use the String() function of the
// object type passed in, if it is a typed struct. Otherwise, it
// just returns the Unquoted format value.
//
// The error comes if an error occurs locating and executing a String()
// function for this object, including bytecode runtime errors if the
// String() function is bytecode.
func formatUsingString(s *symbols.SymbolTable, v any) (string, error) {
	var typeDef *data.Type

	switch m := v.(type) {
	case *data.Struct:
		typeDef = m.Type()

	case *data.Scalar:
		typeDef = m.Type()
	}

	if typeDef != nil {
		if f := typeDef.Function("String"); f != nil {
			if fmt, ok := f.(func(s *symbols.SymbolTable, args data.List) (any, error)); ok {
				local := symbols.NewChildSymbolTable("local to format", s)
				local.SetAlways(defs.ThisVariable, v)

				if si, err := fmt(local, data.NewList()); err == nil {
					if str, ok := si.(string); ok {
						return str, nil
					} else {
						return "", errors.ErrInvalidReturnValue.Context(typeDef.Name() + ".String()")
					}
				}
			}

			// Might also be bytecode String function.
			if fmt, ok := f.(data.Function); ok { // Create a symbol table to use for the slice comparator callback function.
				if fn, ok := fmt.Value.(*bytecode.ByteCode); ok {
					stringSymbols := symbols.NewChildSymbolTable(fmt.Declaration.Name, s)
					ctx := bytecode.NewContext(stringSymbols, fn)

					// Set up a call to the String function with our data item
					// There are no arguments to a String function.
					stringSymbols.SetAlways(defs.ArgumentListVariable,
						data.NewArrayFromInterfaces(data.InterfaceType))

					// If there is a __tracing flag in our stack, use it to
					// set the trace mode in this new context.
					if v, found := s.Get(defs.TraceSymbolName); found {
						ctx.SetTrace(data.BoolOrFalse(v))
					}

					// But there is a "this" variable. GetThis (compiled into
					// the String() method's own prologue) now reads the
					// pending receiver staged here rather than popping the
					// receiver stack directly -- see CALL-11 in
					// docs/issues/ and the SetPendingReceiver doc comment.
					ctx.SetPendingReceiver(v)

					// Run the String function. If it fails, return error as the string
					// @TODO fix this with proper return next.
					if err := ctx.Run(); err != nil {
						return "", err
					}

					return data.String(ctx.Result()), nil
				}
			}
		}
	}

	return data.FormatUnquoted(v), nil
}
