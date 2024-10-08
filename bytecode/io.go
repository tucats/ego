package bytecode

import (
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/formats"
	"github.com/tucats/ego/tokenizer"
)

/******************************************\
*                                         *
*           B A S I C   I / O             *
*                                         *
\******************************************/

func consoleByteCode(c *Context, i interface{}) error {
	c.EnableConsoleOutput(data.Bool(i))

	return nil
}

// printByteCode instruction processor. If the operand is given, it represents
// the number of items to remove from the stack and print to stdout.
func printByteCode(c *Context, i interface{}) error {
	count := 1

	if i != nil {
		count = data.Int(i)
	}

	// See if there is a results marker on the stack. If so, we need
	// to print everything up to that marker
	if depth := findMarker(c, "results"); depth > 0 {
		count = depth - 1
	}

	for n := 0; n < count; n = n + 1 {
		value, err := c.Pop()
		if err != nil {
			return c.error(errors.ErrMissingPrintItems).Context(count)
		}

		if isStackMarker(value) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		s := ""

		if n > 0 {
			if c.output == nil {
				fmt.Printf(" ")
			} else {
				c.output.WriteString(" ")
			}
		}

		switch actualValue := value.(type) {
		case *data.Array:
			// Is this an array of a single type that is a structure?
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

		case *data.Package:
			s = formats.PackageAsString(actualValue)

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

// logByteCode implements the Log directive, which outputs the top stack
// item to the logger named in the operand. The operand can either by a logger
// by name or by class id.
func logByteCode(c *Context, i interface{}) error {
	var class int

	if id, ok := i.(int); ok {
		class = id
	} else {
		class = ui.LoggerByName(data.String(i))
	}

	if class <= ui.NoSuchLogger {
		return c.error(errors.ErrInvalidLoggerName).Context(i)
	}

	msg, err := c.Pop()
	if err != nil {
		return err
	}

	ui.Log(class, "%v", msg)

	return nil
}

// sayByteCode instruction processor. If the operand is true, output the string as-is,
// else output it adding a trailing newline. The Say opcode  can be used in place
// of NewLine to end buffered output, but the output is only displayed if we are
// not in --quiet mode.
//
// This is used by the code generated from @test and @pass, for example, to allow
// test logging to be quiet if necessary.
func sayByteCode(c *Context, i interface{}) error {
	msg := ""
	if c.output != nil {
		msg = c.output.String()
		c.output = nil

		if len(msg) > 0 {
			ui.Say("%s", strings.TrimSuffix(msg, "\n"))
		}

		return nil
	}

	fmt := "%s"
	if data.Bool(i) && len(msg) == 0 {
		fmt = "%s\n"
	}

	ui.Say(fmt, msg)

	return nil
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

/******************************************\
*                                         *
*           T E M P L A T E S             *
*                                         *
\******************************************/

// templateByteCode compiles a template string from the stack and stores it in
// the template manager for the execution context.
func templateByteCode(c *Context, i interface{}) error {
	name := data.String(i)

	t, err := c.Pop()
	if err == nil {
		if isStackMarker(t) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}

		t, e2 := template.New(name).Parse(data.String(t))
		if e2 == nil {
			err = c.push(t)
		} else {
			err = c.error(e2)
		}
	}

	return err
}

/******************************************\
*                                         *
*             U T I L I T Y               *
*                                         *
\******************************************/

// fromFileByteCode loads the context tokenizer with the
// source from a file if it does not already exist and
// we are in debug mode.
func fromFileByteCode(c *Context, i interface{}) error {
	if !c.debugging {
		return nil
	}

	if b, err := os.ReadFile(data.String(i)); err == nil {
		c.tokenizer = tokenizer.New(string(b), false)

		return nil
	} else {
		return errors.New(err)
	}
}

func timerByteCode(c *Context, i interface{}) error {
	mode := data.Int(i)
	switch mode {
	case 0:
		t := time.Now()
		c.timerStack = append(c.timerStack, t)

		return nil

	case 1:
		timerStack := len(c.timerStack)
		if timerStack == 0 {
			return c.error(errors.ErrInvalidTimer)
		}

		t := c.timerStack[timerStack-1]
		c.timerStack = c.timerStack[:timerStack-1]
		now := time.Now()
		elapsed := now.Sub(t)
		ms := elapsed.Milliseconds()
		unit := "s"

		// If the unit scale is too large or too small, then
		// adjust it down to millisends or up to minutes.
		if ms == 0 {
			ms = elapsed.Microseconds()
			unit = "ms"
		} else if ms > 60000 {
			ms = ms / 1000
			unit = "m"
		}

		msText := fmt.Sprintf("%4.3f%s", float64(ms)/1000.0, unit)

		return c.push(msText)

	default:
		return c.error(errors.ErrInvalidTimer).Context(mode)
	}
}
