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
func profileByteCode(c *Context, i any) error {
	var (
		err error
		op  int
	)

	if i == nil {
		return c.runtimeError(errors.ErrInvalidInstruction)
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
			return c.runtimeError(errors.ErrInvalidProfileAction).Context(s)
		}
	} else {
		op, err = data.Int(i)
		if err != nil {
			return c.runtimeError(err)
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
func stopByteCode(c *Context, i any) error {
	c.running.Store(false)

	return errors.ErrStop
}

// panicByteCode instruction processor generates an error. The argument is
// used to add context to the runtime error message generated. Note that this
// normally will stop execution of the Ego program and report an error (with
// an Ego stack trace). If the ego.runtime.panics configuration is set to
// "true", then a native Go panic will be generated.
func panicByteCode(c *Context, i any) error {
	var panicMessage string

	c.running.Store(false)

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

// moduleByteCode sets the current context module name to the operand.
//
// The operand may be:
//   - a plain string: sets c.module to that string.
//   - a []any{moduleName, *tokenizer.Tokenizer}: sets c.module and stores the
//     tokenizer on the context so the debugger and trace output can read source
//     lines from it.
//
// FLOW-2: The original code accessed array[1] unconditionally.  If the
// compiler ever emits a one-element array operand (e.g. when the source has
// no tokenizer), this would panic with "index out of range".  The len check
// below makes the tokenizer slot optional so a one-element array is handled
// safely.
func moduleByteCode(c *Context, i any) error {
	if array, ok := i.([]any); ok {
		c.module = data.String(array[0])

		// Only look for the tokenizer when a second element is actually present.
		// A one-element array sets the module name and leaves c.tokenizer unchanged.
		if len(array) > 1 {
			if t, ok := array[1].(*tokenizer.Tokenizer); ok {
				c.tokenizer = t

				// Release any resources the tokenizer was holding while parsing;
				// we no longer need random-access into the token stream.
				t.Close()
			}
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
func atLineByteCode(c *Context, i any) error {
	var (
		err  error
		line int
		text string
	)

	// If this context is temporarily being shared with a go-routine, serialize access.
	if c.shared.Load() {
		c.mux.Lock()
		defer c.mux.Unlock()
	}

	// Get the info from the argument.  The operand may be:
	//   - a plain integer: the line number only.
	//   - []any{lineNumber, sourceText}: line number and the original source text
	//     of that line (used by the trace output to print the source alongside the
	//     execution).
	//
	// FLOW-2: The original code accessed array[1] unconditionally.  A
	// one-element array operand would panic with "index out of range".
	// The len check makes the source-text slot optional so single-element
	// arrays are handled safely.
	if array, ok := i.([]any); ok {
		if line, err = data.Int(array[0]); err != nil {
			return err
		}

		// Only read the source text when a second element is present.
		if len(array) > 1 {
			text = data.String(array[1])
		}
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

			c.traceLine(location, text)
		}
	}

	c.lastLine = c.line

	return nil
}

func (c *Context) traceLine(location string, text string) {
	// if trace logger is inactive, we should send the output to the context io.Writer
	// So it can be gathered up by the dashboard.  Use the i18n package to format the
	// line for the output.
	if c.output != nil || !ui.IsActive(ui.TraceLogger) {
		text := ui.FormatLogMessage(ui.TraceLogger, "log.trace.line", ui.A{
			"thread":   c.threadID,
			"location": location,
			"text":     strings.TrimSpace(text)})

		text = ui.FormatJSONLogEntryAsText(text)

		// c.output is a Writer, so write the text to the output.
		c.output.Write([]byte(text + "\n"))
	} else {
		ui.Log(ui.TraceLogger, "trace.line", ui.A{
			"thread":   c.threadID,
			"location": location,
			"text":     strings.TrimSpace(text)})
	}
}

// getPackageSymbols returns the symbol table embedded in the package that is
// the current method receiver, or nil if the receiver stack is empty or the
// top receiver is not a *data.Package.
//
// The receiver stack stores this{name, value} structs.  GetPackageSymbolTable
// expects a *data.Package, so we must pass the unwrapped value field, not the
// this struct itself.  Passing the struct directly (the original bug, CALL-5)
// caused the type assertion inside GetPackageSymbolTable to always fail,
// making the package-method clone path in callBytecodeFunction permanently
// unreachable.
func (c *Context) getPackageSymbols() *symbols.SymbolTable {
	if len(c.receiverStack) == 0 {
		return nil
	}

	// Extract the receiver's concrete value.  this.value is the *data.Package
	// (or whatever other type the receiver holds); the outer this struct is
	// the bookkeeping wrapper and must not be passed to GetPackageSymbolTable.
	receiver := c.receiverStack[len(c.receiverStack)-1]

	table := symbols.GetPackageSymbolTable(receiver.value)
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

func waitByteCode(c *Context, i any) error {
	if _, ok := i.(*sync.WaitGroup); ok {
		i.(*sync.WaitGroup).Wait()
	} else {
		goRoutineCompletion.Wait()
	}

	return nil
}

func modeCheckBytecode(c *Context, i any) error {
	mode, found := c.symbols.Get(defs.ModeVariable)

	if found && (data.String(i) == data.String(mode)) {
		return nil
	}

	return c.runtimeError(errors.ErrWrongMode).Context(mode)
}

func ifErrorByteCode(c *Context, i any) error {
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
			return c.runtimeError(err)
		}

		return c.runtimeError(errors.ErrInvalidType)
	}

	return nil
}
