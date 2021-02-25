package bytecode

import (
	"fmt"
	"os"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// OpcodeHandler defines a function that implements an opcode.
type OpcodeHandler func(b *Context, i interface{}) *errors.EgoError

// DispatchMap is a map that is used to locate the function for an opcode.
type DispatchMap map[OpcodeID]OpcodeHandler

var dispatch DispatchMap
var dispatchMux sync.Mutex

// GrowStackBy indicates the number of eleemnts to add to the stack when
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
	return c.RunFromAddress(c.pc)
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
	c.pc = addr
	c.running = true

	if c.Tracing {
		ui.Debug(ui.TraceLogger, "*** Tracing "+c.Name)
	}

	fullStackListing := util.GetBool(c.configGet("full_stack_listing"))

	// Loop over the bytecodes and run.
	for c.running {
		if c.pc >= len(c.bc.instructions) {
			c.running = false

			break
		}

		i := c.bc.instructions[c.pc]

		if c.Tracing {
			s := FormatInstruction(i)

			s2 := FormatStack(c.symbols, c.stack[:c.sp], fullStackListing)
			if !fullStackListing && len(s2) > 50 {
				s2 = s2[:50]
			}

			ui.Debug(ui.TraceLogger, "%8s%3d: %-30s stack[%2d]: %s",
				c.GetModuleName(), c.pc, s, c.sp, s2)
		}

		c.pc = c.pc + 1

		imp, found := dispatch[i.Operation]
		if !found {
			return c.NewError(errors.UnimplementedInstructionError).Context(i.Operation)
		}

		err = imp(c, i.Operand)
		if !errors.Nil(err) {
			text := err.Error()

			// See if we are in a try/catch block. IF there is a Try/Catch stack
			// and the jump point on top is non-zero, then we can transfer control.
			// Note that if the error was fatal, the running flag is turned off, which
			// prevents the try block from being honored (i.e. you cannot catch a fatal
			// error).
			if len(c.try) > 0 && c.try[len(c.try)-1] > 0 && c.running {
				c.pc = c.try[len(c.try)-1]

				// Zero out the jump point for this try/catch block so recursive
				// errors don't occur.
				c.try[len(c.try)-1] = 0

				// Implicit pop-scope done here.
				_ = c.symbols.SetAlways(ErrorVariableName, err)

				if c.Tracing {
					ui.Debug(ui.TraceLogger, "*** Branch to %d on error: %s", c.pc, text)
				}
			} else {
				if !err.Is(errors.SignalDebugger) && !err.Is(errors.Stop) && c.Tracing {
					ui.Debug(ui.TraceLogger, "*** Return error: %s", text)
				}

				return errors.New(err)
			}
		}
	}

	if c.Tracing {
		ui.Debug(ui.TraceLogger, "*** End tracing "+c.Name)
	}

	return errors.New(err)
}

// GoRoutine allows calling a named function as a go routine, using arguments. The invocation
// of GoRoutine should be in a "go" statement to run the code.
func GoRoutine(fName string, parentCtx *Context, args []interface{}) {
	syms := parentCtx.symbols
	err := parentCtx.NewError(errors.InvalidFunctionCallError)

	ui.Debug(ui.TraceLogger, "--> Starting Go routine \"%s\"", fName)
	ui.Debug(ui.TraceLogger, "--> Argument list: %#v", args)

	// Locate the bytecode for the function. It must be a symbol defined as bytecode.
	if fCode, ok := syms.Get(fName); ok {
		if bc, ok := fCode.(*ByteCode); ok {
			if true {
				ui.DebugMode = true

				bc.Disasm()
			}
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
			funcSyms := symbols.NewChildSymbolTable("Go routine "+fName, syms)
			funcSyms.Merge(syms)

			ctx := NewContext(funcSyms, callCode)
			ctx.Tracing = true
			ui.DebugMode = true

			err = parentCtx.NewError(ctx.Run())
		}
	}

	if !err.Is(errors.Stop) {
		fmt.Printf("Go routine  %s failed, %v\n", fName, err)
		ui.Debug(ui.TraceLogger, "--> Go routine invocation ends with %v", err)
		os.Exit(55)
	}
}
