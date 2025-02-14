package bytecode

import (
	"os"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

func consoleByteCode(c *Context, i interface{}) error {
	b, err := data.Bool(i)
	if err != nil {
		return err
	}

	c.EnableConsoleOutput(b)

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
		return c.runtimeError(errors.ErrInvalidLoggerName).Context(i)
	}

	msg, err := c.Pop()
	if err != nil {
		return err
	}

	ui.Log(class, "logging.bytecode", ui.A{
		"message": msg})

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

	b, err := data.Bool(i)
	if err != nil {
		return err
	}

	if b && len(msg) == 0 {
		fmt = "%s\n"
	}

	ui.Say(fmt, msg)

	return nil
}

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
