package bytecode

import (
	"fmt"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/symbols"
)

// goRoutineCompletion synchronizes the bytecode execution of an Ego go routine with the
// native Go routine that hosts it. The completion wait group prevents this go routine from
// completing before the bytecode has executed.
var goRoutineCompletion sync.WaitGroup

// goByteCode instruction processor launches a new goroutine to run the
// function identified on the stack. This accepts the same arguments as
// the call function, but instead of running the function in the current
// thread, it launches a new thread to run the function.
func goByteCode(c *Context, i interface{}) error {
	argc := data.Int(i) + c.argCountDelta
	c.argCountDelta = 0

	args := make([]interface{}, argc)

	// Loop backwards through the stack to get the arguments.
	for n := 0; n < argc; n = n + 1 {
		v, err := c.Pop()
		if err != nil {
			return err
		}

		args[(argc-n)-1] = v
	}

	// Get the function name from the stack. If there is nothing on the stack,
	// it's an error. Otherwise, convert to string and launmch it by name.
	if fx, err := c.Pop(); err != nil {
		return err
	} else {
		// Launch the function call as a separate thread.
		ui.Log(ui.TraceLogger, "--> (%d)  Launching go routine %v", c.threadID, fx)
		goRoutineCompletion.Add(1)

		go GoRoutine(fx, c, data.NewList(args...))

		return nil
	}
}

// GoRoutine allows calling a named function as a go routine, using arguments. The invocation
// of GoRoutine should be in a "go" statement to run the code.
func GoRoutine(fx interface{}, parentCtx *Context, args data.List) {
	fName := fmt.Sprintf("%v", fx)

	// We will need exclusive access to the parent context symbols table long enough
	// to find the next scope above the parent context past any barriers. This is the
	// "global" scope, which may be one or more layers of parent contexts.
	parentCtx.mux.RLock()
	parentSymbols := parentCtx.symbols.FindNextScope()
	parentCtx.mux.RUnlock()

	ui.Log(ui.TraceLogger, "--> Starting Go routine %s", fName)
	ui.Log(ui.TraceLogger, "--> Argument list: %#v", args)

	// Create a new stream whose job is to invoke the function by name. We mark this
	// as a literal function so that calls to it will not generate scope barriers
	callCode := New("go " + fName).Literal(true)
	callCode.Emit(Push, fx)

	for _, arg := range args.Elements() {
		callCode.Emit(Push, arg)
	}

	callCode.Emit(Call, args.Len())

	// Make a new table that is parently only to the root table (for access to
	// packages). Copy the function definition into this new table so the invocation
	// of the function within the native go routine can locate it.
	functionSymbols := symbols.NewChildSymbolTable("Go routine ", parentSymbols.SharedParent()).Boundary(false)

	// Run the bytecode in a new context. This will be a child of the parent context.
	ctx := NewContext(functionSymbols, callCode)
	err := parentCtx.error(ctx.Run())

	goRoutineCompletion.Done()

	// If we had an error in the go routine, stop the invoking context execution.
	if err != nil && !err.Is(errors.ErrStop) {
		msg := fmt.Sprintf("%s", i18n.E("go.error", map[string]interface{}{"id": ctx.threadID, "name": fName, "err": err}))
		ui.Log(ui.InfoLogger, msg)

		ui.Log(ui.TraceLogger, "--> Go routine invocation (thread %d) ends with %v", ctx.threadID, err)

		parentCtx.goErr = err
		parentCtx.running = false
	}
}
