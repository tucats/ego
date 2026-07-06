package bytecode

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/util/profiling"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokenizer"
)

/*
*******************************************\
*                                         *
*           P R O F I L I N G             *
*                                         *
\******************************************/

// profileByteCode enables, disables, or prints a report of Ego's built-in
// profiler, which counts how many times each source line executes.
//
// The operand i can be:
//   - A string like "enable", "start", or "on"  — to begin profiling.
//   - A string like "disable", "stop", or "off" — to stop profiling.
//   - A string like "report", "print", or "dump" — to print the results.
//   - An integer matching one of the profiling.XxxAction constants.
//
// The string forms are accepted because they are more readable in compiled
// Ego source; the integer form is used internally when the compiler emits the
// instruction with a pre-computed action code.
//
// strings.ToLower is called so the comparison is case-insensitive — "Enable"
// and "ENABLE" both work the same as "enable".
func profileByteCode(c *Context, i any) error {
	var (
		err error
		op  int
	)

	if i == nil {
		return c.runtimeError(errors.ErrInvalidInstruction)
	}

	// Type assertion: "i.(string)" asks "is i actually a string?".
	// If the assertion succeeds, ok is true and s holds the string value.
	// This two-value form is the safe way to assert — it never panics.
	if s, ok := i.(string); ok {
		switch strings.ToLower(s) {
		case "enable", "start", "on":
			op = profiling.StartAction
		case "disable", "stop", "off":
			op = profiling.StopAction
		case "report", "print", "dump":
			op = profiling.ReportAction
		default:
			// .Context(s) appends the unexpected value to the error message
			// so the programmer knows what string was rejected.
			return c.runtimeError(errors.ErrInvalidProfileAction).Context(s)
		}
	} else {
		// The operand was not a string, so try to convert it to an integer.
		// data.Int handles many underlying types (int, int32, float64, string
		// digits, etc.) and returns an error if the conversion is not possible.
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

// stopByteCode halts execution of the current Ego program immediately.
//
// It works in two ways:
//  1. c.running is an atomic.Bool (a boolean that is safe to read and write
//     from multiple goroutines without a mutex). Calling .Store(false) sets
//     it to false, which tells the execution loop to stop after this
//     instruction finishes.
//  2. Returning errors.ErrStop signals to any caller that execution ended
//     normally (not due to a bug), so callers can distinguish a clean stop
//     from an unexpected failure.
//
// The operand i is unused — this instruction takes no argument.
func stopByteCode(c *Context, i any) error {
	c.running.Store(false)

	return errors.ErrStop
}

// panicByteCode handles the Ego @fail directive, which unconditionally
// terminates the running program with an error message.
//
// The message is obtained in one of two ways:
//   - If the operand i is not nil, it is used directly as the message.
//   - If i is nil, the message is popped from the value stack. This lets
//     the compiler push a dynamic expression result and then emit this
//     instruction with no operand.
//
// After capturing the message, execution is halted in one of two ways
// depending on the ego.runtime.panics configuration setting:
//   - Normally (setting is false): the function returns errors.ErrPanic with
//     the message attached via .Context(). The execution loop sees the error
//     and stops cleanly, printing an Ego-level error report.
//   - If the setting is true: a native Go panic() is triggered. This unwinds
//     the entire Go call stack rather than just the Ego stack, and is
//     primarily useful for debugging the Ego runtime itself — it produces a
//     full Go goroutine stack trace.
//
// Note: this instruction is NOT catchable by an Ego try/catch block. For
// catchable errors, see signalByteCode. The distinction is intentional:
// @fail is a hard stop; @error generates a recoverable error.
func panicByteCode(c *Context, i any) error {
	var panicMessage string

	c.running.Store(false)

	if i != nil {
		panicMessage = data.String(i)
	} else {
		// Two-value "if" in Go: run the assignment, then check the condition.
		// If Pop() returns an error (e.g., the stack is empty), the inner
		// branch returns that error immediately rather than panicking on a
		// nil value. The "else" clause is needed here because panicMessage is
		// used after the if/else block.
		if v, err := c.Pop(); err != nil {
			return err
		} else {
			panicMessage = data.String(v)
		}
	}

	if settings.GetBool(defs.RuntimePanicsSetting) {
		fmt.Println("Ego call stack:")
		// ShowAllCallFrames (-1) tells FormatFrames to include every frame,
		// not just the topmost few.
		fmt.Println(c.FormatFrames(ShowAllCallFrames))
		// Built-in Go panic: unwinds the Go stack, not just the Ego stack.
		panic(panicMessage)
	}

	return errors.ErrPanic.Context(panicMessage)
}

// moduleByteCode records the name of the Ego source module (typically a
// filename or a function name) currently being executed. This name appears
// in error messages and debug output so the user knows which file a problem
// came from.
//
// The operand i can be:
//   - A plain string: only the module name is provided. c.module is updated
//     and c.tokenizer is left unchanged.
//   - A []any slice with two elements: [moduleName, *tokenizer.Tokenizer].
//     The tokenizer holds the original source text of the module, which is
//     used by the trace and debugger to print source lines alongside
//     bytecode execution. After storing it, t.Close() is called to release
//     any internal buffers that were only needed during parsing.
func moduleByteCode(c *Context, i any) error {
	// Check whether the operand is a []any slice. In Go, []any is a slice
	// whose elements can each be a different type. The compiler packs the
	// module name and optional tokenizer into one of these slices so a single
	// operand can carry both pieces of information.
	if array, ok := i.([]any); ok {
		c.module = data.String(array[0])

		// Only look for the tokenizer when a second element is actually present.
		// A one-element array sets the module name and leaves c.tokenizer unchanged.
		if len(array) > 1 {
			// *tokenizer.Tokenizer is a pointer type. The type assertion here
			// checks that array[1] holds a pointer to a Tokenizer specifically.
			if t, ok := array[1].(*tokenizer.Tokenizer); ok {
				c.tokenizer = t

				// Release any resources the tokenizer was holding while parsing;
				// we no longer need random-access into the token stream.
				t.Close()
			}
		}
	} else {
		// The operand is a plain value (usually a string). data.String converts
		// any supported type to its string representation.
		c.module = data.String(i)
	}

	return nil
}

// atLineByteCode records that execution has reached a new source-code line.
// The compiler emits this instruction at the start of each statement so that
// error messages, the debugger, and trace output can report the correct line.
//
// The operand i can be:
//   - A plain integer: the source line number only.
//   - A []any slice with two elements: [lineNumber, sourceText], where
//     sourceText is the original text of that source line. This text is
//     stored on the context and printed alongside bytecode traces.
//
// After updating the line number this function:
//   - Resets c.stepOver (a debugger flag) to false, because a new statement
//     is beginning.
//   - Publishes the current line and module name into the symbol table so
//     Ego programs can read them via the __line and __module variables.
//   - Counts the line visit in the profiler (a no-op when profiling is off).
//   - If debugger mode is active, returns errors.ErrSignalDebugger to
//     pause execution and hand control back to the interactive debugger.
//   - If tracing is active and the line changed, prints the source text
//     so the developer can follow execution interactively.
func atLineByteCode(c *Context, i any) error {
	var (
		err  error
		line int
		text string
	)

	// If this context is being shared with a goroutine (c.shared is an
	// atomic.Bool), acquire the mutex before modifying c.line and c.source.
	// "defer c.mux.Unlock()" schedules the unlock to run automatically when
	// this function returns, regardless of which return path is taken. This
	// is Go's standard idiom for ensuring a mutex is always released.
	if c.shared.Load() {
		c.mux.Lock()
		defer c.mux.Unlock()
	}

	// Unpack the operand into a line number and optional source text.
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
	// stepOver is a debugger flag: when true the debugger skips into called
	// functions. Reset it here so each new statement starts with step-over off.
	c.stepOver = false

	// Publish the current source location into the Ego symbol table so that
	// Ego programs can inspect __line and __module at runtime.
	c.symbols.SetAlways(defs.LineVariable, c.line)
	c.symbols.SetAlways(defs.ModuleVariable, c.bc.name)

	// Record this line visit in the profiler. When profiling is disabled this
	// call compiles away to almost nothing.
	profiling.Count(c.bc.name, c.line)

	// In debugger mode, returning this special error pauses execution and
	// signals the interactive debugger to prompt for the next command.
	// Line 0 is a synthetic line used during initialization — do not pause there.
	if c.line != 0 && c.debugging {
		return errors.ErrSignalDebugger
	}

	// Trace mode: print the source line the first time we reach it so the
	// developer can watch execution. c.lastLine guards against printing the
	// same line twice when the same source line generates multiple AtLine
	// instructions (e.g., a multi-expression statement).
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

// traceLine emits a single source-line trace entry, routing the output to
// the right destination depending on whether the trace logger is active and
// whether the context has a captured output writer.
//
// Two output paths exist:
//  1. If c.output is set (the context's output is being captured, e.g., by
//     the web dashboard) OR if the TraceLogger is not active, the trace line
//     is formatted as a human-readable string and written to c.output directly.
//     This ensures trace output ends up in the same place as program output
//     when both are being captured together.
//  2. If the TraceLogger is active and c.output is nil, the entry is written
//     through the structured logging system (ui.Log), which formats it as a
//     JSON log entry or plain text depending on the server configuration.
//
// In both paths, ui.A is a shorthand type for map[string]any — a map of
// named substitution values that are inserted into the localized message
// template. The "thread", "location", and "text" keys correspond to
// placeholders in the i18n message string.
func (c *Context) traceLine(location string, text string) {
	if c.output != nil || !ui.IsActive(ui.TraceLogger) {
		// Format the trace message using the i18n template, then convert the
		// resulting JSON log entry back to a plain human-readable string before
		// writing it to c.output. The c.output field implements io.Writer, the
		// standard Go interface for anything that accepts a stream of bytes.
		text := ui.FormatLogMessage(ui.TraceLogger, "log.trace.line", ui.A{
			"thread":   c.threadID,
			"location": location,
			"text":     strings.TrimSpace(text)})

		text = ui.FormatJSONLogEntryAsText(text)

		c.output.Write([]byte(text + "\n"))
	} else {
		// The trace logger is active and we are not capturing output, so let
		// the logging system handle formatting and routing.
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
// In Ego, when a method is called on a package value (e.g., myPkg.DoThing()),
// the package is pushed onto the receiver stack so the method body can find
// the package's exported symbols. This helper looks at the top of that stack
// and returns the associated symbol table, which the call machinery then
// inserts into the symbol table chain for the duration of the method call.
//
// The receiver stack stores "this" structs that bundle a name with a value.
// GetPackageSymbolTable expects a *data.Package, so we must pass the unwrapped
// value field, not the outer "this" struct itself. Passing the struct directly
// (the original bug, CALL-5) caused the type assertion inside
// GetPackageSymbolTable to always fail silently, making the package-method
// clone path in callBytecodeFunction permanently unreachable.
func (c *Context) getPackageSymbols() *symbols.SymbolTable {
	if len(c.receiverStack) == 0 {
		return nil
	}

	// In Go, slice[len(slice)-1] is the idiomatic way to peek at the last
	// (top) element of a slice used as a stack without removing it.
	receiver := c.receiverStack[len(c.receiverStack)-1]

	// Pass the concrete value, not the "this" wrapper struct.
	table := symbols.GetPackageSymbolTable(receiver.value)

	// Log when we are about to enter a package symbol table that is not
	// already in the current symbol table chain. This helps trace calls
	// into imported packages.
	if table != nil && !c.inPackageSymbolTable(table.Package()) {
		if ui.IsActive(ui.TraceLogger) {
			ui.Log(ui.TraceLogger, "trace.package.symbols", ui.A{
				"thread":  c.threadID,
				"package": table.Package()})
		}
	}

	return table
}

// inPackageSymbolTable reports whether any symbol table in the current chain
// already belongs to the named package. This prevents inserting the same
// package's symbol table twice when a package method calls another method
// on the same package.
//
// Ego's symbol tables are organized as a linked chain: each table has a
// Parent() pointer to the enclosing scope. This loop walks that chain from
// the innermost (most local) scope outward to the global root, returning
// true as soon as a matching package name is found.
func (c *Context) inPackageSymbolTable(name string) bool {
	p := c.symbols
	for p != nil {
		if p.Package() == name {
			return true
		}
		// Walk up to the enclosing scope.
		p = p.Parent()
	}

	return false
}

// waitByteCode blocks the current Ego thread until one or more goroutines
// finish. It implements the Ego analog of Go's sync.WaitGroup.Wait().
//
// Two behaviors depending on the operand i:
//   - If i is a *sync.WaitGroup: waits on that specific WaitGroup. This is
//     used when the Ego program explicitly called sync.NewWaitGroup() and
//     passed the result to a wait statement.
//   - Otherwise: waits on the package-level goRoutineCompletion WaitGroup,
//     which tracks every goroutine started by the "go" statement in Ego code.
//     This is the common case for programs that use "go func()" without an
//     explicit WaitGroup.
//
// sync.WaitGroup is a standard Go synchronization primitive. Its Wait()
// method blocks until the group's internal counter reaches zero. The counter
// is incremented with Add(n) before launching goroutines and decremented with
// Done() when each one finishes. This pattern guarantees the main program
// does not exit before all goroutines complete.
func waitByteCode(c *Context, i any) error {
	if _, ok := i.(*sync.WaitGroup); ok {
		// The blank identifier _ discards the first return value of the type
		// assertion because we only need the boolean ok to decide the branch;
		// we access i.(*sync.WaitGroup) directly on the next line.
		i.(*sync.WaitGroup).Wait()
	} else {
		// Fall back to the global WaitGroup that tracks all Ego goroutines.
		goRoutineCompletion.Wait()
	}

	return nil
}

// modeCheckBytecode verifies that the current execution mode matches the
// expected mode stored in the operand i. If they do not match, it returns a
// runtime error so the caller knows the operation is not allowed in this mode.
//
// Ego programs can run in different modes (for example, "server" mode vs.
// "script" mode). Some operations are only valid in specific modes. The
// compiler emits this instruction at the start of mode-restricted code
// paths, passing the required mode name as the operand.
//
// The current mode is read from the symbol table via defs.ModeVariable (a
// well-known symbol name like "__mode"). If the variable does not exist
// (found == false) or its value does not match the required mode, execution
// stops with ErrWrongMode.
func modeCheckBytecode(c *Context, i any) error {
	mode, found := c.symbols.Get(defs.ModeVariable)

	if found && (data.String(i) == data.String(mode)) {
		return nil
	}

	// Attach the actual mode to the error so the message reads, for example:
	// "wrong execution mode: server"
	return c.runtimeError(errors.ErrWrongMode).Context(mode)
}

// ifErrorByteCode pops the top of the stack and conditionally raises an error.
// It is emitted after operations that can produce either a valid value or an
// indication of failure, such as a type assertion or a map lookup that might
// not find a key.
//
// The stack value is interpreted as follows:
//
//  1. StackMarker: a sentinel value that marks a boundary on the stack (used
//     to separate groups of values, e.g., function call argument lists). If
//     a StackMarker is on top, it is pushed back and this instruction is a
//     no-op. Markers must never be consumed by instructions that are looking
//     for real values.
//
//  2. error: the previous operation already produced an error value. Return
//     it directly so the Ego error-handling machinery can propagate it.
//
//  3. Any other value: convert it to bool with data.Bool. A false result
//     means the condition that this instruction is guarding did not hold
//     (e.g., a type assertion returned its zero value and ok=false). In
//     that case return the error provided in the operand i, or a generic
//     ErrInvalidType if i is not an error.
//
// The operand i is typically the specific error the compiler wants to report
// when the condition is false (e.g., ErrInvalidType for a failed assertion).
func ifErrorByteCode(c *Context, i any) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Case 1: StackMarker — push it back untouched and return.
	// The blank identifier _ suppresses the "result not used" compiler error
	// for the push return value, which we do not need here.
	if _, ok := v.(StackMarker); ok {
		_ = c.push(v)

		return nil
	}

	// Case 2: the value is already an error — propagate it.
	// Note: this shadows the outer "err" variable declared above. The inner
	// "err" here holds the typed error extracted from the interface value.
	if err, ok := v.(error); ok {
		return err
	}

	// Case 3: convert the value to bool and treat false as a failure.
	b, err := data.Bool(v)
	if err != nil {
		return err
	}

	if !b {
		// The operand may carry a specific error for this failure case.
		if err, ok := i.(error); ok {
			return c.runtimeError(err)
		}

		return c.runtimeError(errors.ErrInvalidType)
	}

	return nil
}
