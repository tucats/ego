package bytecode

import (
	"fmt"
	"os"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/symbols"
)

// OpcodeHandler defines a function that implements an opcode.
type OpcodeHandler func(b *Context, i interface{}) error

// DispatchMap is a map that is used to locate the function for an opcode.
type DispatchMap map[Opcode]OpcodeHandler

var dispatch DispatchMap
var dispatchMux sync.Mutex
var waitGroup sync.WaitGroup

// GrowStackBy indicates the number of elements to add to the stack when
// it runs out of space.
const GrowStackBy = 50

func (c *Context) GetName() string {
	if c.bc != nil {
		return c.bc.name
	}

	return defs.Main
}

func (c *Context) StepOver(b bool) {
	c.stepOver = b
}

func (c *Context) GetSymbols() *symbols.SymbolTable {
	return c.symbols
}

// Run executes a bytecode context.
func (c *Context) Run() error {
	return c.RunFromAddress(0)
}

// Used to resume execution after an event like the debugger being invoked.
func (c *Context) Resume() error {
	return c.RunFromAddress(c.programCounter)
}

func (c *Context) IsRunning() bool {
	return c.running
}

// RunFromAddress executes a bytecode context from a given starting address.
func (c *Context) RunFromAddress(addr int) error {
	var err error

	// Make sure globals are initialized. Because this updates a global, let's
	// do it in a thread-safe fashion.
	dispatchMux.Lock()
	initializeDispatch()
	dispatchMux.Unlock()

	// Reset the runtime context.
	c.programCounter = addr
	c.running = true

	ui.Log(ui.TraceLogger, "*** Tracing %s (%d)  ", c.Name, c.threadID)

	// Loop over the bytecodes and run.
	for c.running {
		if c.programCounter >= len(c.bc.instructions) {
			c.running = false

			break
		}

		i := c.bc.instructions[c.programCounter]

		if c.Tracing() {
			instruction := FormatInstruction(i)

			stack := c.formatStack(c.symbols, c.fullStackTrace)
			if !c.fullStackTrace && len(stack) > 80 {
				stack = stack[:80]
			}

			ui.Log(ui.TraceLogger, "(%d) %18s %3d: %-30s stack[%2d]: %s",
				c.threadID, c.GetModuleName(), c.programCounter, instruction, c.stackPointer, stack)
		}

		c.programCounter = c.programCounter + 1

		imp, found := dispatch[i.Operation]
		if !found {
			return c.error(errors.ErrUnimplementedInstruction).Context(i.Operation)
		}

		err = imp(c, i.Operand)
		if err != nil {
			text := err.Error()

			// See if we are in a try/catch block. IF there is a Try/Catch stack
			// and the jump point on top is non-zero, then we can transfer control.
			// Note that if the error was fatal, the running flag is turned off, which
			// prevents the try block from being honored (i.e. you cannot catch a fatal
			// error).
			if len(c.tryStack) > 0 && c.tryStack[len(c.tryStack)-1].addr > 0 && c.running {
				// Do we have a selective set of things we catch?
				willCatch := true

				try := c.tryStack[len(c.tryStack)-1]
				if len(try.catches) > 0 {
					willCatch = false

					for _, e := range try.catches {
						if e.(*errors.Error).Equal(err) {
							willCatch = true

							break
						}
					}
				}

				// If we aren't catching it, just percolate the error
				if !willCatch {
					return errors.NewError(err)
				}

				// We are catching, so update the PC
				c.programCounter = try.addr

				// Zero out the jump point for this try/catch block so recursive
				// errors don't occur.
				c.tryStack[len(c.tryStack)-1].addr = 0

				// Implicit pop-scope done here.
				c.symbols.SetAlways(ErrorVariableName, err)

				if ui.IsActive(ui.TraceLogger) {
					ui.Log(ui.TraceLogger, "(%d)  *** Branch to %d on error: %s", c.threadID, c.programCounter, text)
				}
			} else {
				if !errors.Equals(err, errors.ErrSignalDebugger) && !errors.Equals(err, errors.ErrStop) {
					ui.Log(ui.TraceLogger, "(%d)  *** Return error: %s", c.threadID, err)
				}

				if err != nil {
					err = errors.NewError(err)
				}

				return err
			}
		}
	}

	ui.Log(ui.TraceLogger, "*** End tracing %s (%d) ", c.Name, c.threadID)

	if err != nil {
		return errors.NewError(err)
	}

	return nil
}

// GoRoutine allows calling a named function as a go routine, using arguments. The invocation
// of GoRoutine should be in a "go" statement to run the code.
func GoRoutine(fName string, parentCtx *Context, args []interface{}) {
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

			for _, arg := range args {
				callCode.Emit(Push, arg)
			}

			callCode.Emit(Call, len(args))

			// Only the root symbol table is thread-safe, so each go routine is isolated from the
			// symbol scope it was run from. But we need the function definitions, etc. so copy the
			// function values from the previous symbol table.
			functionSymbols := symbols.NewChildSymbolTable("Go routine "+fName, parentSymbols)

			ctx := NewContext(functionSymbols, callCode)
			err = parentCtx.error(ctx.Run())

			waitGroup.Done()
		}
	}

	if err != nil && !err.Is(errors.ErrStop) {
		fmt.Printf("%s\n", i18n.E("go.error", map[string]interface{}{"name": fName, "err": err}))

		ui.Log(ui.TraceLogger, "--> Go routine invocation ends with %v", err)
		os.Exit(55)
	}
}
