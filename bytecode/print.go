package bytecode

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/formats"
)

// printByteCode instruction processor. If the operand is given, it represents
// the number of items to remove from the stack and print to stdout.
func printByteCode(c *Context, i interface{}) error {
	var err error

	count := 1
	skipNil := false

	if i != nil {
		count, err = data.Int(i)
		if err != nil {
			return c.runtimeError(err)
		}
	}

	// See if there is a results marker on the stack. If so, we need
	// to print everything up to that marker
	if depth := findMarker(c, "results"); depth > 0 {
		count = depth - 1
		skipNil = true
	}

	for n := 0; n < count; n = n + 1 {
		value, err := c.Pop()
		if err != nil {
			return c.runtimeError(errors.ErrMissingPrintItems).Context(count)
		}

		// If this is the last tuple item and it's nil, it is almost certainly
		// a return code, so we ignore it.
		if n == count-1 && skipNil && data.IsNil(value) {
			continue
		}

		if isStackMarker(value) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}

		s := ""

		if n > 0 {
			if c.output == nil {
				fmt.Printf(" ")
			} else {
				c.output.WriteString(" ")
			}
		}

		s = formatValueForPrinting(value)

		if c.output == nil {
			fmt.Printf("%s", s)
		} else {
			c.output.WriteString(s)
		}
	}

	// If we are instruction tracing, print out a newline anyway so the trace
	// display isn't made illegible.
	if c.output == nil && c.Tracing() {
		fmt.Println()
	}

	return nil
}

func formatValueForPrinting(value interface{}) string {
	var s string

	switch actualValue := value.(type) {
	case *data.Array:
		valueType := actualValue.Type()
		isStruct := valueType.Kind() == data.StructKind
		isStructType := valueType.Kind() == data.TypeKind && valueType.BaseType().Kind() == data.StructKind

		if isStruct || isStructType {
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

			for i := 0; i < actualValue.Len(); i++ {
				rowValue, _ := actualValue.Get(i)
				row := rowValue.(*data.Struct)

				rowItems := []string{}

				for _, key := range columns {
					v := row.GetAlways(key)
					rowItems = append(rowItems, data.String(v))
				}

				_ = t.AddRow(rowItems)
			}

			s = strings.Join(t.FormatText(), "\n")
		} else {
			r := make([]string, actualValue.Len())
			for n := 0; n < len(r); n++ {
				vvv, _ := actualValue.Get(n)
				r[n] = data.String(vvv)
			}

			s = strings.Join(r, "\n")
		}

	case *data.Struct:
		s = formats.StructAsString(actualValue, false)

	case *data.Map:
		s = formats.MapAsString(actualValue, false)

	case *data.Type:
		s = actualValue.String()

	case *data.Function:
	default:
		s = data.FormatUnquoted(value)
	}

	return s
}

// newlineByteCode instruction processor generates a newline character to stdout.
func newlineByteCode(c *Context, i interface{}) error {
	if c.output == nil {
		fmt.Printf("\n")
	} else {
		c.output.WriteString("\n")
	}

	return nil
}
