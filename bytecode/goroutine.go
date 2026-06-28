package bytecode

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// goRoutineCompletion synchronizes the bytecode execution of an Ego go routine with the
// native Go routine that hosts it. The completion wait group prevents this go routine from
// completing before the bytecode has executed.
var (
	goRoutineCompletion sync.WaitGroup
	messageMutex        sync.Mutex
)

// goByteCode instruction processor launches a new goroutine to run the
// function identified on the stack. This accepts the same arguments as
// the call function, but instead of running the function in the current
// thread, it launches a new thread to run the function.
func goByteCode(c *Context, i any) error {
	c.shared.Store(true)

	argc, err := data.Int(i)
	if err != nil {
		return c.runtimeError(err)
	}

	argc += c.argCountDelta
	c.argCountDelta = 0

	args := make([]any, argc)

	// Loop backwards through the stack to get the arguments.
	for n := 0; n < argc; n = n + 1 {
		v, err := c.Pop()
		if err != nil {
			return err
		}

		args[(argc-n)-1] = v
	}

	// Get the function name from the stack. If there is nothing on the stack,
	// it's an error. Otherwise, convert to string and launch it by name.
	if fx, err := c.Pop(); err != nil {
		return err
	} else {
		// Launch the function call as a separate thread.
		ui.Log(ui.GoRoutineLogger, "go.launch", ui.A{
			"function": fx,
			"thread":   c.threadID})

		goRoutineCompletion.Add(1)

		go GoRoutine(fx, c, data.NewList(args...))

		return nil
	}
}

// GoRoutine allows calling a named function as a go routine, using arguments. The invocation
// of GoRoutine should be in a "go" statement to run the code.
//
// # Symbol table setup for named vs. closure goroutines
//
// Named function goroutine (go namedFunc(args)):
//   fx is a *ByteCode with IsLiteral() == false, or fx is a data.Function
//   value.  The goroutine only needs access to global package-level names,
//   so functionSymbols is created as a non-boundary child of the first
//   shared ancestor of the parent context's current scope.  On most systems
//   that is the global root table, which already contains all imported packages.
//
// Closure / anonymous function goroutine (go func() { ... }()):
//   fx is a *ByteCode with IsLiteral() == true.  Its capturedScope field
//   (stamped by pushByteCode when the go statement ran in the parent context)
//   points to the parent's local scope — the scope that contains the outer
//   variables the closure wants to read or write.
//
//   Before the goroutine runs we mark the entire captured scope chain as
//   shared.  The "shared" flag causes every subsequent Get / Set call on
//   those tables to acquire a read or write lock, respectively.  Without
//   this, the parent thread and the goroutine would both access the same
//   symbol tables without any synchronisation, constituting a data race.
//
//   functionSymbols is still a non-boundary child of the first shared ancestor
//   so the goroutine's call bootstrap code can resolve global names (packages,
//   imported functions).  The closure's own variables are accessed through
//   capturedScope after callBytecodeFunction installs the closure's frame.
func GoRoutine(fx any, parentCtx *Context, args data.List) {
	messageMutex.Lock()

	fName := fmt.Sprintf("%v", fx)

	// We will need exclusive access to the parent context symbols table long enough
	// to find the next scope above the parent context past any barriers. This is the
	// "global" scope, which may be one or more layers of parent contexts.
	parentCtx.mux.Lock()
	parentSymbols := parentCtx.symbols.FindNextScope()
	parentCtx.shared.Store(false)
	parentCtx.mux.Unlock()

	// If the function being launched is a closure (anonymous function literal),
	// mark its captured scope chain as shared so that concurrent reads and writes
	// by the goroutine and the parent thread are properly serialized.
	//
	// This must happen before ctx.Run() below and before messageMutex.Unlock()
	// so it completes before the parent thread can resume and modify the tables.
	if bc, ok := fx.(*ByteCode); ok && bc.IsLiteral() {
		if captured := bc.GetCapturedScope(); captured != nil {
			// Shared(true) marks this table and all its ancestors as shared.
			// That propagation is important: the closure walks the entire
			// ancestor chain when resolving symbol names, so every table in
			// that chain must be protected.
			captured.Shared(true)
		}
	}

	// Create a new stream whose job is to invoke the function by name. We mark this
	// as a literal function so that calls to it will not generate scope barriers
	callCode := New("go " + fName).Literal(true)
	callCode.Emit(Push, fx)

	for _, arg := range args.Elements() {
		callCode.Emit(Push, arg)
	}

	callCode.Emit(Call, args.Len())

	// Make a new table that is parented only to the root table (for access to
	// packages). Copy the function definition into this new table so the invocation
	// of the function within the native go routine can locate it.
	functionSymbols := symbols.NewChildSymbolTable("Go routine ", parentSymbols.SharedParent()).Boundary(false)

	// Run the bytecode in a new context. This will be a child of the parent context.
	ctx := NewContext(functionSymbols, callCode)

	if ui.IsActive(ui.GoRoutineLogger) {
		ui.Log(ui.GoRoutineLogger, "go.native", ui.A{
			"name":   fName,
			"thread": ctx.threadID})

		text := strings.Builder{}

		for idx, arg := range args.Elements() {
			if idx > 0 {
				text.WriteString(", ")
			}

			text.WriteString(data.Format(arg))
		}

		ui.Log(ui.GoRoutineLogger, "go.args", ui.A{
			"thread": ctx.threadID,
			"args":   text.String()})
	}

	messageMutex.Unlock()

	// Run the go routine and handle any errors. If the error is not a STOP error,
	// print a message and stop the invoking context execution. This ensures that
	// the invoking context continues to run, even if the go routine encounters an error.
	err := parentCtx.runtimeError(ctx.Run())

	// Signal that the go routine has completed. This is used by the @wait directive
	// to wait for all go routines to complete, if desired, before exiting the main
	// program.
	goRoutineCompletion.Done()

	// If we had an error in the go routine, stop the invoking context execution.
	if err != nil && !err.Is(errors.ErrStop) {
		ui.Log(ui.GoRoutineLogger, "go.exit.error", ui.A{
			"thread": ctx.threadID,
			"name":   fName,
			"error":  err})

		// Protect writes to shared parentCtx fields with the context mutex,
		// since multiple goroutines may attempt to set these concurrently.
		parentCtx.mux.Lock()
		parentCtx.goErr = err
		parentCtx.running.Store(false)
		parentCtx.mux.Unlock()
	} else {
		ui.Log(ui.GoRoutineLogger, "go.exit", ui.A{
			"thread": ctx.threadID,
			"name":   fName})
	}
}
