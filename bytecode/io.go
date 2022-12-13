package bytecode

import (
	"fmt"
	"io/ioutil"
	"strings"
	"text/template"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/formats"
	"github.com/tucats/ego/tokenizer"
)

/******************************************\
*                                         *
*           B A S I C   I / O             *
*                                         *
\******************************************/

// printByteCode instruction processor. If the operand is given, it represents the number of items
// to remove from the stack and print to stdout.
func printByteCode(c *Context, i interface{}) *errors.EgoError {
	count := 1
	if i != nil {
		count = datatypes.GetInt(i)
	}

	for n := 0; n < count; n = n + 1 {
		v, err := c.Pop()
		if !errors.Nil(err) {
			return err
		}

		s := ""

		// If it's an array, print each element as a row in the output.
		if vv, ok := v.(*datatypes.EgoArray); ok {
			r := make([]string, vv.Len())
			for n := 0; n < len(r); n++ {
				vvv, _ := vv.Get(n)
				r[n] = datatypes.GetString(vvv)
			}

			s = strings.Join(r, "\n")
		} else if vv, ok := v.(*datatypes.EgoPackage); ok {
			s = formats.PackageAsString(vv)
		} else if vv, ok := v.(*datatypes.EgoStruct); ok {
			s = formats.StructAsString(vv)
		} else if vv, ok := v.(*datatypes.EgoMap); ok {
			s = formats.MapAsString(vv)
		} else {
			s = datatypes.FormatUnquoted(v)
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
func logByteCode(c *Context, i interface{}) *errors.EgoError {
	var class int

	if id, ok := i.(int); ok {
		class = id
	} else {
		class = ui.Logger(datatypes.GetString(i))
	}

	if class < 0 {
		return c.newError(errors.ErrInvalidLoggerName).Context(i)
	}

	msg, err := c.Pop()
	if errors.Nil(err) {
		ui.Debug(class, "%v", msg)
	}

	return err
}

// sayByteCode instruction processor. If the operand is true, output the string as-is,
// else output it adding a trailing newline. The Say opcode  can be used in place
// of NewLine to end buffered output, but the output is only displayed if we are
// not in --quiet mode.
//
// This is used by the code generated from @test and @pass, for example, to allow
// test logging to be quiet if necessary.
func sayByteCode(c *Context, i interface{}) *errors.EgoError {
	msg := ""
	if c.output != nil {
		msg = c.output.String()
		c.output = nil
	}

	fmt := "%s\n"
	if datatypes.GetBool(i) && len(msg) > 0 {
		fmt = "%s"
	}

	ui.Say(fmt, msg)

	return nil
}

// newlineByteCode instruction processor generates a newline character to stdout.
func newlineByteCode(c *Context, i interface{}) *errors.EgoError {
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
func templateByteCode(c *Context, i interface{}) *errors.EgoError {
	name := datatypes.GetString(i)

	t, err := c.Pop()
	if errors.Nil(err) {
		t, e2 := template.New(name).Parse(datatypes.GetString(t))
		if e2 == nil {
			err = c.stackPush(t)
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
func fromFileByteCode(c *Context, i interface{}) *errors.EgoError {
	if !c.debugging {
		return nil
	}

	b, err := ioutil.ReadFile(datatypes.GetString(i))
	if errors.Nil(err) {
		c.tokenizer = tokenizer.New(string(b))
	}

	return errors.New(err)
}

func timerByteCode(c *Context, i interface{}) *errors.EgoError {
	mode := datatypes.GetInt(i)
	switch mode {
	case 0:
		t := time.Now()
		c.timerStack = append(c.timerStack, t)

	case 1:
		timerStack := len(c.timerStack)
		if timerStack == 0 {
			return c.newError(errors.ErrInvalidTimer)
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

		_ = c.stackPush(msText)

	default:
		return c.newError(errors.ErrInvalidTimer)
	}

	return nil
}
