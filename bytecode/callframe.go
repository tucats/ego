package bytecode

import (
	"fmt"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// CallFrame is an object used to store state of the bytecode runtime
// environment just before making a call to a bytecode subroutine. This
// preserves the state of the stack, PC, and other data at the time
// of the call. When a bytecode subroutine returns, this object is
// removed from the stack and used to reset the bytecode runtime state.
//
// Note that this is exported (as are Module and Line within it) to support
// formatting of trace data using reflection.
type CallFrame struct {
	Module        string
	Line          int
	symbols       *symbols.SymbolTable
	bytecode      *ByteCode
	tokenizer     *tokenizer.Tokenizer
	thisStack     []This
	singleStep    bool
	breakOnReturn bool
	blockDepth    int
	pc            int
	fp            int
}

// callframePush pushes a single object on the stack that represents the state of
// the current execution. This is done as part of setting up a call to a new
// routine, so it can be restored when a return is executed.
func (c *Context) callframePush(tableName string, bc *ByteCode, pc int, boundary bool) {
	_ = c.stackPush(CallFrame{
		symbols:    c.symbols,
		bytecode:   c.bc,
		singleStep: c.singleStep,
		tokenizer:  c.tokenizer,
		thisStack:  c.thisStack,
		pc:         c.programCounter,
		fp:         c.framePointer,
		Module:     c.bc.Name,
		Line:       c.line,
		blockDepth: c.blockDepth,
	})

	ui.Debug(ui.SymbolLogger, "(%d) push symbol table \"%s\" <= \"%s\"",
		c.threadID, tableName, c.symbols.Name)

	c.framePointer = c.stackPointer
	c.result = nil
	c.symbols = symbols.NewChildSymbolTable(tableName, c.symbols)
	c.symbols.ScopeBoundary = boundary
	c.bc = bc
	c.programCounter = pc

	// Now that we've saved state on the stack, if we are in step-over mode,
	// then turn off single stepping
	if c.singleStep && c.stepOver {
		c.singleStep = false
	}
}

// callFramePop retrieves the call frame information from the stack, and updates
// the current bytecode context to reflect the previously-stored state.
func (c *Context) callFramePop() error {
	// First, is there stuff on the stack we want to preserve?
	topOfStackSlice := []interface{}{}

	if c.framePointer <= c.stackPointer+1 {
		topOfStackSlice = c.stack[c.framePointer : c.stackPointer+1]
	}

	// Now retrieve the runtime context stored on the stack and
	// indicated by the fp (frame pointer)
	c.stackPointer = c.framePointer
	cx, err := c.Pop()

	if err != nil {
		return err
	}

	// Before we toss away this, check to see if there are package symbols
	// that need updating in the package object.
	if c.symbols.Parent != nil && c.symbols.Parent.Package != "" {
		packageSymbols := c.symbols.Parent
		packageName := c.symbols.Parent.Package

		if pkg, ok := c.symbols.Root().Get(packageName); ok {
			if _, ok := pkg.(*datatypes.EgoStruct); ok {
				ui.Log(ui.InternalLogger, "ERROR: callFramePop(), map/struct confusion")

				return errors.EgoError(errors.ErrStop)
			}

			if m, ok := pkg.(*datatypes.EgoPackage); ok {
				for _, k := range packageSymbols.Names() {
					if util.HasCapitalizedName(k) {
						pkgSymbol, _ := packageSymbols.Get(k)

						m.Set(k, pkgSymbol)
					}
				}
			}
		}
	}

	if callFrame, ok := cx.(CallFrame); ok {
		ui.Debug(ui.SymbolLogger, "(%d) pop symbol table; \"%s\" => \"%s\"",
			c.threadID, c.symbols.Name, callFrame.symbols.Name)

		c.line = callFrame.Line
		c.symbols = callFrame.symbols
		c.singleStep = callFrame.singleStep
		c.tokenizer = callFrame.tokenizer
		c.thisStack = callFrame.thisStack
		c.bc = callFrame.bytecode
		c.programCounter = callFrame.pc
		c.framePointer = callFrame.fp
		c.blockDepth = callFrame.blockDepth
		c.breakOnReturn = callFrame.breakOnReturn
	} else {
		return c.newError(errors.ErrInvalidCallFrame)
	}

	// Finally, if there _was_ stuff on the stack after the call,
	// it might be a multi-value return, so push that back.
	if len(topOfStackSlice) > 0 {
		c.stack = append(c.stack[:c.stackPointer], topOfStackSlice...)
		c.stackPointer = c.stackPointer + len(topOfStackSlice)
	} else {
		// Alternatively, it could be a single-value return using the
		// result holder. If so, push that on the stack and clear it.
		if c.result != nil {
			err = c.stackPush(c.result)
			c.result = nil
		}
	}

	return err
}

func (c *Context) SetBreakOnReturn() {
	cx := c.stack[c.framePointer]
	if callFrame, ok := cx.(CallFrame); ok {
		ui.Debug(ui.SymbolLogger, "(%d) setting break-on-return", c.threadID)

		callFrame.breakOnReturn = true
		c.stack[c.framePointer] = callFrame
	} else {
		ui.Debug(ui.SymbolLogger, "(%d) failed setting break-on-return; call frame invalid", c.threadID)
	}
}

// FormatFrames is called from the runtime debugger to print out the
// current call frames stored on the stack. It chases the stack using
// the frame pointer (FP) in the current context which points to the
// saved frame. Its FP points to the previous saved frame, and so on.
func (c *Context) FormatFrames(maxDepth int) string {
	f := c.framePointer
	depth := 1
	r := fmt.Sprintf("Call frames:\n  at: %12s  (%s)\n",
		formatLocation(c.GetModuleName(), c.line), c.symbols.Name)

	for (maxDepth < 0 || depth < maxDepth) && f > 0 {
		fx := c.stack[f-1]

		if frame, ok := fx.(CallFrame); ok {
			r = r + fmt.Sprintf("from: %12s  (%s)\n",
				formatLocation(frame.Module, frame.Line), frame.symbols.Name)
			f = frame.fp

			depth++
		} else {
			break
		}
	}

	return r
}

// Utility function that abstracts out how we format a location using
// a module name and line number.
func formatLocation(module string, line int) string {
	return fmt.Sprintf("%s %d", module, line)
}
