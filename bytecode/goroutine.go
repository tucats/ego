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

// GoRoutine allows calling a named function as a go routine, using arguments. The invocation
// of GoRoutine should be in a "go" statement to run the code.
func GoRoutine(fName string, parentCtx *Context, args data.List) {
	// We will need exclusive access to the parent context symbols table.
	parentCtx.mux.RLock()
	parentSymbols := parentCtx.symbols
	parentCtx.mux.RUnlock()

	err := parentCtx.error(errors.ErrInvalidFunctionCall)

	ui.Log(ui.TraceLogger, "--> Starting Go routine \"%s\"", fName)
	ui.Log(ui.TraceLogger, "--> Argument list: %#v", args)

	// Locate the bytecode for the function. It must be a symbol defined as bytecode.
	if fCode, ok := parentSymbols.Get(fName); ok {
		if bc, ok := fCode.(*ByteCode); ok {
			bc.Disasm()
			// Create a new stream whose job is to invoke the function by name.
			callCode := New("go " + fName)
			callCode.Emit(Load, fName)

			for _, arg := range args.Elements() {
				callCode.Emit(Push, arg)
			}

			callCode.Emit(Call, args.Len())

			// Make a new table that is parently only to the root table (for access to
			// packages). Copy the function definition into this new table so the invocation
			// of the function within the native go routine can locate it.
			functionSymbols := symbols.NewChildSymbolTable("Go routine "+fName, parentSymbols.SharedParent())
			functionSymbols.SetAlways(fName, bc)

			// Run the bytecode in a new context. This will be a child of the parent context.
			ctx := NewContext(functionSymbols, callCode)
			err = parentCtx.error(ctx.Run())

			goRoutineCompletion.Done()
		}
	}

	// If we had an error in the go routine, stop the invoking context execution.
	if err != nil && !err.Is(errors.ErrStop) {
		fmt.Printf("%s\n", i18n.E("go.error", map[string]interface{}{"name": fName, "err": err}))
		ui.Log(ui.TraceLogger, "--> Go routine invocation ends with %v", err)

		parentCtx.goErr = err
		parentCtx.running = false
	}
}
