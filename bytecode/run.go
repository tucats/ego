package bytecode

import (
	"fmt"
	"os"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// OpcodeHandler defines a function that implements an opcode.
type OpcodeHandler func(b *Context, i interface{}) *errors.EgoError

// DispatchMap is a map that is used to locate the function for an opcode.
type DispatchMap map[OpcodeID]OpcodeHandler

var dispatch DispatchMap
var dispatchMux sync.Mutex
var waitGroup sync.WaitGroup

// GrowStackBy indicates the number of elements to add to the stack when
// it runs out of space.
const GrowStackBy = 50

func (c *Context) GetName() string {
	if c.bc != nil {
		return c.bc.Name
	}

	return "main"
}

func (c *Context) StepOver(b bool) {
	c.stepOver = b
}

func (c *Context) GetSymbols() *symbols.SymbolTable {
	return c.symbols
}

// Run executes a bytecode context.
func (c *Context) Run() *errors.EgoError {
	return c.RunFromAddress(0)
}

// Used to resume execution after an event like the debugger being invoked.
func (c *Context) Resume() *errors.EgoError {
	return c.RunFromAddress(c.programCounter)
}

func (c *Context) IsRunning() bool {
	return c.running
}

// RunFromAddress executes a bytecode context from a given starting address.
func (c *Context) RunFromAddress(addr int) *errors.EgoError {
	var err *errors.EgoError

	// Make sure globals are initialized. Because this updates a global, let's
	// do it in a thread-safe fashion.
	dispatchMux.Lock()
	initializeDispatch()
	dispatchMux.Unlock()

	// Reset the runtime context.
	c.programCounter = addr
	c.running = true

	if ui.LoggerIsActive(ui.TraceLogger) {
		ui.Debug(ui.TraceLogger, "*** Tracing %s (%d)  ", c.Name, c.threadID)
	}

	// Loop over the bytecodes and run.
	for c.running {
		if c.programCounter >= len(c.bc.instructions) {
			c.running = false

			break
		}

		i := c.bc.instructions[c.programCounter]

		if c.Tracing() {
			s := FormatInstruction(i)

			s2 := FormatStack(c.symbols, c.stack[:c.stackPointer], c.fullStackTrace)
			if !c.fullStackTrace && len(s2) > 80 {
				s2 = s2[:80]
			}

			ui.Debug(ui.TraceLogger, "(%d) %18s %3d: %-30s stack[%2d]: %s",
				c.threadID, c.GetModuleName(), c.programCounter, s, c.stackPointer, s2)
		}

		c.programCounter = c.programCounter + 1

		imp, found := dispatch[i.Operation]
		if !found {
			return c.newError(errors.ErrUnimplementedInstruction).Context(i.Operation)
		}

		err = imp(c, i.Operand)
		if !errors.Nil(err) {
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
						if e.Equal(err) {
							willCatch = true

							break
						}
					}
				}

				// If we aren't catching it, just percolate the error
				if !willCatch {
					return errors.New(err)
				}

				// We are catching, so update the PC
				c.programCounter = try.addr

				// Zero out the jump point for this try/catch block so recursive
				// errors don't occur.
				c.tryStack[len(c.tryStack)-1].addr = 0

				// Implicit pop-scope done here.
				_ = c.symbols.SetAlways(ErrorVariableName, err)

				if ui.LoggerIsActive(ui.TraceLogger) {
					ui.Debug(ui.TraceLogger, "(%d)  *** Branch to %d on error: %s", c.threadID, c.programCounter, text)
				}
			} else {
				if !err.Is(errors.ErrSignalDebugger) && !err.Is(errors.ErrStop) {
					ui.Debug(ui.TraceLogger, "(%d)  *** Return error: %s", c.threadID, err)
				}

				return errors.New(err)
			}
		}
	}

	ui.Debug(ui.TraceLogger, "*** End tracing %s (%d) ", c.Name, c.threadID)

	return errors.New(err)
}

// GoRoutine allows calling a named function as a go routine, using arguments. The invocation
// of GoRoutine should be in a "go" statement to run the code.
func GoRoutine(fName string, parentCtx *Context, args []interface{}) {
	parentSymbols := parentCtx.symbols
	err := parentCtx.newError(errors.ErrInvalidFunctionCall)

	ui.Debug(ui.TraceLogger, "--> Starting Go routine \"%s\"", fName)
	ui.Debug(ui.TraceLogger, "--> Argument list: %#v", args)

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
			err = parentCtx.newError(ctx.Run())

			waitGroup.Done()
		}
	}

	if !err.Is(errors.ErrStop) {
		fmt.Printf("Go routine  %s failed, %v\n", fName, err)
		ui.Debug(ui.TraceLogger, "--> Go routine invocation ends with %v", err)
		os.Exit(55)
	}
}
