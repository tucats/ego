package bytecode

import (
	"fmt"
	"io/ioutil"
	"text/template"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*           B A S I C   I / O             *
*                                         *
\******************************************/

// PrintImpl instruction processor. If the operand is given, it represents the number of items
// to remove from the stack and print to stdout.
func PrintImpl(c *Context, i interface{}) *errors.EgoError {
	count := 1
	if i != nil {
		count = util.GetInt(i)
	}

	for n := 0; n < count; n = n + 1 {
		v, err := c.Pop()
		if !errors.Nil(err) {
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
func LogImpl(c *Context, i interface{}) *errors.EgoError {
	logger := util.GetString(i)

	msg, err := c.Pop()
	if errors.Nil(err) {
		ui.Debug(logger, "%v", msg)
	}

	return err
}

// SayImpl instruction processor. This can be used in place of NewLine to end
//buffered output, but the output is only displayed if we are not in --quiet mode.
func SayImpl(c *Context, i interface{}) *errors.EgoError {
	ui.Say("%s\n", c.output.String())
	c.output = nil

	return nil
}

// NewlineImpl instruction processor generates a newline character to stdout.
func NewlineImpl(c *Context, i interface{}) *errors.EgoError {
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
func TemplateImpl(c *Context, i interface{}) *errors.EgoError {
	name := util.GetString(i)

	t, err := c.Pop()
	if errors.Nil(err) {
		t, e2 := template.New(name).Parse(util.GetString(t))
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

// FromFileImpl loads the context tokenizer with the
// source from a file if it does not alrady exist and
// we are in debug mode.
func FromFileImpl(c *Context, i interface{}) *errors.EgoError {
	if !c.debugging {
		return nil
	}

	b, err := ioutil.ReadFile(util.GetString(i))
	if errors.Nil(err) {
		c.tokenizer = tokenizer.New(string(b))
	}

	return errors.New(err)
}

func TimerImpl(c *Context, i interface{}) *errors.EgoError {
	mode := util.GetInt(i)
	switch mode {
	case 0:
		t := time.Now()
		c.timers = append(c.timers, t)

	case 1:
		timerStack := len(c.timers)
		if timerStack == 0 {
			return c.NewError(errors.InvalidTimerError)
		}

		t := c.timers[timerStack-1]
		c.timers = c.timers[:timerStack-1]
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
		return c.NewError(errors.InvalidTimerError)
	}

	return nil
}
