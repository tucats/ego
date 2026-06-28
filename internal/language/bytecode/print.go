package bytecode

// This file implements three bytecode instructions that produce user-visible
// output:
//
//   - printByteCode  (Print opcode)  — pops one or more values from the stack
//     and writes them to the context's output writer, space-separated.
//   - newlineByteCode (Newline opcode) — writes a single newline character.
//   - formatValueForPrinting — a private helper that converts any Ego runtime
//     value into a human-readable string suitable for direct output.
//
// These instructions are emitted by the compiler when it processes the `print`
// extension keyword (not part of standard Go — available only when Ego language
// extensions are enabled via ego.compiler.extensions=true).  The `print`
// keyword is similar to Go's `fmt.Print` but works at the bytecode level
// without going through the `fmt` package runtime wrappers.
//
// # Output destination
//
// Bytecode output is always written to c.output (an io.Writer).  By default
// c.output is os.Stdout.  Calling c.EnableConsoleOutput(false) redirects
// output to an internal strings.Builder that can be read with c.GetOutput().
// Tests use this capture mode to inspect what the Print instruction produced
// without letting the output leak to the terminal.

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/internal/cli/tables"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/util/formats"
)

const nilName = "nil"

// printByteCode is the instruction handler for the Print opcode.
//
// It pops one or more values from the runtime stack and writes them to the
// context's output writer.  Multiple values are separated by a single space
// character.
//
// Stack contract:
//   - The top N items are consumed (N = operand, or 1 when operand is nil).
//   - Nothing is pushed; the stack is net-reduced by N items.
//
// Operand:
//   - nil  — pop exactly one item.
//   - int  — pop that many items (e.g., operand=3 pops three items).
//   - Any other type causes a runtime error.
//
// Special case — results marker:
//
//	The Ego runtime sometimes wraps multi-return function results with a
//	StackMarker("results") sentinel below the result values.  If such a
//	marker is found on the stack, printByteCode switches to "tuple mode":
//	  - count  is reset to the number of items above the marker.
//	  - skipNil is set to true so that a nil last item (the conventional
//	    Go error return when no error occurred) is silently suppressed.
//
// Error conditions:
//   - ErrMissingPrintItems if the stack runs out before all items are popped.
//   - ErrFunctionReturnedVoid if a StackMarker is encountered mid-loop
//     (meaning the function returned no usable value).
func printByteCode(c *Context, i any) error {
	var err error

	// Default: pop exactly one item.
	count := 1
	skipNil := false

	// If an explicit count was provided as the operand, use it.
	if i != nil {
		count, err = data.Int(i)
		if err != nil {
			return c.runtimeError(err)
		}
	}

	// If a "results" marker is present on the stack, override count with
	// the number of items sitting above the marker.  Also enable nil-skipping
	// so a trailing nil error value does not appear in the output.
	if depth := findMarker(c, "results"); depth > 0 {
		count = depth - 1
		skipNil = true
	}

	for n := 0; n < count; n = n + 1 {
		value, err := c.Pop()
		if err != nil {
			return c.runtimeError(errors.ErrMissingPrintItems).Context(count)
		}

		// In tuple mode, the last item is often a nil error return from the
		// called function.  Ego prints (value, err) calls like `fmt.Sprintf`
		// suppress the nil error so only the useful value is shown.
		if n == count-1 && skipNil && data.IsNil(value) {
			continue
		}

		// A StackMarker should never appear as a printable value.  If one
		// slips through, it means the function that produced the result
		// returned void (no value) and the marker is the only thing on the
		// stack.
		if isStackMarker(value) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}

		// Items after the first are separated by a single space, matching
		// the behavior of Go's fmt.Print for multiple arguments.
		if n > 0 {
			fmt.Fprint(c.output, " ")
		}

		fmt.Fprint(c.output, formatValueForPrinting(value))
	}

	// When instruction tracing is active and output is going to the real
	// stdout (not the capture buffer), append a newline so the next trace
	// line starts on a fresh line rather than running into the printed value.
	if c.captureBuffer == nil && c.Tracing() {
		fmt.Fprintln(c.output)
	}

	return nil
}

// formatValueForPrinting converts an Ego runtime value to a string suitable
// for direct output to the user.  It is called once per stack item inside
// printByteCode.
//
// The formatting rules are:
//
//   - *data.Array whose element type is a struct (or a named type whose base
//     is a struct) — formatted as a multi-column text table, one array element
//     per row and one struct field per column.  This lets Ego programs print
//     slices of records in a readable tabular form.  Nil or non-struct elements
//     are silently skipped so a partially-initialized array does not panic
//     (PRINT-1 fix).
//
//   - *data.Array of any other element type — each element is converted to a
//     string with data.String and the results are joined with newlines.
//
//   - *data.Struct  — delegates to formats.StructAsString (two-column table).
//
//   - *data.Map     — delegates to formats.MapAsString (key/value table).
//
//   - *data.Type    — returns the type's canonical string representation.
//
//   - data.Function or *data.Function — returns an empty string.  Function
//     values are not considered printable data in Ego; passing one to print
//     produces no visible output (PRINT-2 fix).
//
//   - anything else — delegates to data.FormatUnquoted, which returns a
//     decimal or boolean string.  Plain strings are returned verbatim (without
//     quote characters).
func formatValueForPrinting(value any) string {
	var s string

	switch actualValue := value.(type) {
	case *data.Array:
		valueType := actualValue.Type()
		isStruct := valueType.Kind() == data.StructKind
		isStructType := valueType.Kind() == data.TypeKind && valueType.BaseType().Kind() == data.StructKind

		if isStruct || isStructType {
			// Collect column names from the struct field definitions.
			// For a named type (TypeKind), try the type itself first, then
			// fall back to its base type — the field definitions may live
			// on either depending on how the type was constructed.
			var columns []string

			if isStruct {
				columns = valueType.FieldNames()
			} else {
				columns = valueType.FieldNames()
				if len(columns) == 0 {
					columns = valueType.BaseType().FieldNames()
				}
			}

			t, _ := tables.New(columns)

			// Each element of the array is one row in the output table.
			// Nil and non-struct elements are silently skipped so that a
			// partially-initialized array (e.g., from data.NewArray before
			// all slots are assigned) does not panic (PRINT-1 fix).
			for i := 0; i < actualValue.Len(); i++ {
				rowValue, _ := actualValue.Get(i)
				if rowValue == nil {
					continue
				}

				row, ok := rowValue.(*data.Struct)
				if !ok {
					continue
				}

				rowItems := []string{}

				for _, key := range columns {
					v := row.GetAlways(key)
					rowItems = append(rowItems, data.String(v))
				}

				_ = t.AddRow(rowItems)
			}

			s = strings.Join(t.FormatText(), "\n")
		} else {
			// For non-struct arrays, convert each element to a string and
			// join them with newlines so each element appears on its own line.
			r := make([]string, actualValue.Len())
			for n := 0; n < len(r); n++ {
				vvv, _ := actualValue.Get(n)
				r[n] = data.String(vvv)
			}

			s = strings.Join(r, "\n")
		}

	case *data.Struct:
		// Delegates to the formats package which renders the struct as a
		// two-column key/value table (showTypes=false keeps it concise).
		s = formats.StructAsString(actualValue, false)

	case *data.Map:
		// Delegates to the formats package for a key/value table.
		s = formats.MapAsString(actualValue, false)

	case *data.Type:
		// Type objects print as their canonical declaration string,
		// e.g. "struct{ Name string; Age int }".
		s = actualValue.String()

	case data.Function:
		if actualValue.Declaration == nil {
			s = nilName
		} else {
			s = actualValue.Declaration.String()
		}

	case *data.Function:
		if actualValue == nil {
			s = nilName
		} else {
			if actualValue.Declaration == nil {
				s = nilName
			} else {
				s = actualValue.Declaration.String()
			}
		}

		// Function values — both data.Function (value) and *data.Function
		// (pointer) — are suppressed.  s stays as "" so nothing is written
		// to the output.  Functions are not printable data in Ego (PRINT-2 fix).

	default:
		// For all scalar types (int, float64, bool, string, nil, …)
		// data.FormatUnquoted is used.  Strings are returned verbatim; nil
		// formats as nilName; integers and floats format without a type
		// prefix (e.g. "42", "3.14").
		s = data.FormatUnquoted(value)
	}

	return s
}

// newlineByteCode is the instruction handler for the Newline opcode.
//
// It writes a single newline character to the context's output writer and
// returns nil (no error is possible).  The operand is ignored.
//
// The compiler emits a Newline instruction immediately after Print when the
// `print` statement did not end with a trailing comma (a trailing comma
// suppresses the newline, allowing multi-statement lines — similar to Python 2's
// `print "a",` syntax).
func newlineByteCode(c *Context, i any) error {
	fmt.Fprintln(c.output)

	return nil
}
