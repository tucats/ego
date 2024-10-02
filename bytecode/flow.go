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
)

/*
*******************************************\
*                                         *
*           P R O F I L I N G             *
*                                         *
\******************************************/

// Enable, disable, or report profiling data.
func profileByteCode(c *Context, i interface{}) error {
	if i == nil {
		return c.error(errors.ErrInvalidInstruction)
	}

	var op int

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
		op = data.Int(i)
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
	c.module = data.String(i)

	return nil
}

// atLineByteCode sets the current context line number to the argument.
// atLineByteCode instruction processor. This identifies the start of a new statement,
// and tags the line number from the source where this was found. This is used
// in error messaging, primarily.
func atLineByteCode(c *Context, i interface{}) error {
	var (
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
		line = data.Int(array[0])
		text = data.String(array[1])
	} else {
		line = data.Int(i)
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
			ui.Log(ui.TraceLogger, "(%d) >>> %-19s %s", c.threadID, location, strings.TrimSpace(text))
		}
	}

	c.lastLine = c.line

	return nil
}

// See if the top of the "this" stack is a package, and if so return
// it's symbol table. The stack is not modified.
func (c *Context) getPackageSymbols() *symbols.SymbolTable {
	if len(c.thisStack) == 0 {
		return nil
	}

	this := c.thisStack[len(c.thisStack)-1]

	if pkg, ok := this.value.(*data.Package); ok {
		if s, ok := pkg.Get(data.SymbolsMDKey); ok {
			if table, ok := s.(*symbols.SymbolTable); ok {
				if !c.inPackageSymbolTable(table.Package()) {
					ui.Log(ui.TraceLogger, "(%d)  Using symbol table from package %s", c.threadID, table.Package())

					return table
				}
			}
		}
	}

	return nil
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

	if !data.Bool(v) {
		if err, ok := i.(error); ok {
			return c.error(err)
		}

		return c.error(errors.ErrInvalidType)
	}

	return nil
}
