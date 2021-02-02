package bytecode

import (
	"fmt"
	"io/ioutil"
	"text/template"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*           B A S I C   I / O             *
*                                         *
\******************************************/

// PrintImpl instruction processor. If the operand is given, it represents the number of items
// to remove from the stack and print to stdout
func PrintImpl(c *Context, i interface{}) error {
	count := 1
	if i != nil {
		count = util.GetInt(i)
	}

	for n := 0; n < count; n = n + 1 {
		v, err := c.Pop()
		if err != nil {
			return err
		}
		s := util.FormatUnquoted(v)
		if c.output == nil {
			fmt.Printf("%s", s)
		} else {
			c.output.WriteString(s)
		}
	}

	// If we are instruction tracing, print out a newline anyway so the trace
	// display isn't made illegible.
	if c.output == nil && c.Tracing {
		fmt.Println()
	}

	return nil
}

// LogImpl imeplements the Log directive, which outputs the top stack
// item to the logger named in the operand.
func LogImpl(c *Context, i interface{}) error {
	logger := util.GetString(i)
	msg, err := c.Pop()
	if err == nil {
		ui.Debug(logger, "%v", msg)
	}

	return err
}

// SayImpl instruction processor. This can be used in place of NewLine to end
//buffered output, but the output is only displayed if we are not in --quiet mode.
func SayImpl(c *Context, i interface{}) error {
	ui.Say("%s\n", c.output.String())
	c.output = nil

	return nil
}

// NewlineImpl instruction processor generates a newline character to stdout
func NewlineImpl(c *Context, i interface{}) error {
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

// TemplateImpl compiles a template string from the stack and stores it in
// the template manager for the execution context.
func TemplateImpl(c *Context, i interface{}) error {
	name := util.GetString(i)
	t, err := c.Pop()
	if err == nil {
		t, err = template.New(name).Parse(util.GetString(t))
		if err == nil {
			err = c.Push(t)
		}
	}

	return err
}

/******************************************\
*                                         *
*             U T I L I T Y               *
*                                         *
\******************************************/

// FromFileImpl loads the context tokenizer with the
// source from a file if it does not alrady exist and
// we are in debug mode.
func FromFileImpl(c *Context, i interface{}) error {
	if !c.debugging {
		return nil
	}

	b, err := ioutil.ReadFile(util.GetString(i))
	if err == nil {
		c.tokenizer = tokenizer.New(string(b))
	}

	return err
}
