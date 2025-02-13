package bytecode

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/profiling"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

/*
*******************************************\
*                                         *
*           P R O F I L I N G             *
*                                         *
\******************************************/

// Enable, disable, or report profiling data.
func profileByteCode(c *Context, i interface{}) error {
	var (
		err error
		op  int
	)

	if i == nil {
		return c.error(errors.ErrInvalidInstruction)
	}

	if s, ok := i.(string); ok {
		switch strings.ToLower(s) {
		case "enable", "start", "on":
			op = profiling.StartAction
		case "disable", "stop", "off":
			op = profiling.StopAction
		case "report", "print", "dump":
			op = profiling.ReportAction
		default:
			return c.error(errors.ErrInvalidProfileAction).Context(s)
		}
	} else {
		op, err = data.Int(i)
		if err != nil {
			return c.error(err)
		}
	}

	return profiling.Profile(op)
}

/******************************************\
*                                         *
*        F L O W   C O N T R O L          *
*                                         *
\******************************************/

// stopByteCode instruction processor causes the current execution context to
// stop executing immediately.
func stopByteCode(c *Context, i interface{}) error {
	c.running = false

	return errors.ErrStop
}

// panicByteCode instruction processor generates an error. The argument is
// used to add context to the runtime error message generated. Note that this
// normally will stop execution of the Ego program and report an error (with
// an Ego stack trace). If the ego.runtieme.panics configuration is set to
// "true", then a native Go panic will be generated.
func panicByteCode(c *Context, i interface{}) error {
	var panicMessage string

	c.running = false

	if i != nil {
		panicMessage = data.String(i)
	} else {
		if v, err := c.Pop(); err != nil {
			return err
		} else {
			panicMessage = data.String(v)
		}
	}

	if settings.GetBool(defs.RuntimePanicsSetting) {
		fmt.Println("Ego call stack:")
		fmt.Println(c.FormatFrames(ShowAllCallFrames))
		panic(panicMessage)
	}

	return errors.ErrPanic.Context(panicMessage)
}

// moduleBytecode sets the current context module name to the argument.
func moduleByteCode(c *Context, i interface{}) error {
	if array, ok := i.([]interface{}); ok {
		c.module = data.String(array[0])
		if t, ok := array[1].(*tokenizer.Tokenizer); ok {
			c.tokenizer = t

			// While we're here, let's ensure the tokenizer isn't holding
			// on to resources unnecessarily now that we executing this bytecode.
			t.Close()
		}
	} else {
		c.module = data.String(i)
	}

	return nil
}

// atLineByteCode sets the current context line number to the argument.
// atLineByteCode instruction processor. This identifies the start of a new statement,
// and tags the line number from the source where this was found. This is used
// in error messaging, primarily.
func atLineByteCode(c *Context, i interface{}) error {
	var (
		err  error
		line int
		text string
	)

	// If this context is temporarily being shared with a go-routine, serialize access.
	if c.shared {
		c.mux.Lock()
		defer c.mux.Unlock()
	}

	// Get the info from the argument. The argument can be just an integer
	// value, or it can be a list with an integer line number and a string
	// containing the text of the line from the tokenizer.
	if array, ok := i.([]interface{}); ok {
		if line, err = data.Int(array[0]); err != nil {
			return err
		}

		text = data.String(array[1])
	} else {
		if line, err = data.Int(i); err != nil {
			return err
		}
	}

	c.line = line
	c.source = text
	c.stepOver = false
	c.symbols.SetAlways(defs.LineVariable, c.line)
	c.symbols.SetAlways(defs.ModuleVariable, c.bc.name)

	profiling.Count(c.bc.name, c.line)

	// Are we in debug mode?
	if c.line != 0 && c.debugging {
		return errors.ErrSignalDebugger
	}

	// If we are tracing, put that out now.
	if c.Tracing() && c.tokenizer != nil && c.line != c.lastLine {
		text := c.tokenizer.GetLine(c.line)
		if len(strings.TrimSpace(text)) > 0 {
			location := fmt.Sprintf("line %d", c.line)

			ui.Log(ui.TraceLogger, "trace.line", ui.A{
				"thread":   c.threadID,
				"location": location,
				"text":     strings.TrimSpace(text)})
		}
	}

	c.lastLine = c.line

	return nil
}

// See if the top of the "this" stack is a package, and if so return
// it's symbol table. The stack is not modified.
func (c *Context) getPackageSymbols() *symbols.SymbolTable {
	if len(c.receiverStack) == 0 {
		return nil
	}

	this := c.receiverStack[len(c.receiverStack)-1]

	table := symbols.GetPackageSymbolTable(this)
	if table != nil && !c.inPackageSymbolTable(table.Package()) {
		ui.Log(ui.TraceLogger, "trace.package.symbols", ui.A{
			"thread":  c.threadID,
			"package": table.Package()})
	}

	return table
}

// Determine if the current symbol table stack is already within
// the named package symbol table structure.
func (c *Context) inPackageSymbolTable(name string) bool {
	p := c.symbols
	for p != nil {
		if p.Package() == name {
			return true
		}

		p = p.Parent()
	}

	return false
}

func waitByteCode(c *Context, i interface{}) error {
	if _, ok := i.(*sync.WaitGroup); ok {
		i.(*sync.WaitGroup).Wait()
	} else {
		goRoutineCompletion.Wait()
	}

	return nil
}

func modeCheckBytecode(c *Context, i interface{}) error {
	mode, found := c.symbols.Get(defs.ModeVariable)

	if found && (data.String(i) == data.String(mode)) {
		return nil
	}

	return c.error(errors.ErrWrongMode).Context(mode)
}

func ifErrorByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if _, ok := v.(StackMarker); ok {
		_ = c.push(v)

		return nil
	}

	if err, ok := v.(error); ok {
		return err
	}

	b, err := data.Bool(v)
	if err != nil {
		return err
	}

	if !b {
		if err, ok := i.(error); ok {
			return c.error(err)
		}

		return c.error(errors.ErrInvalidType)
	}

	return nil
}
