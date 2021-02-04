package bytecode

import (
	"fmt"

	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// Type of object pushed/popped from stack describes a call frame
type CallFrame struct {
	Module     string
	Line       int
	Symbols    *symbols.SymbolTable
	Bytecode   *ByteCode
	Tokenizer  *tokenizer.Tokenizer
	This       interface{}
	SingleStep bool
	PC         int
	FP         int
}

// PushFrame pushes a single object on the stack that represents the state of
// the current execution. This is done as part of setting up a call to a new
// routine, so it can be restored when a return is executed.
func (c *Context) PushFrame(tableName string, bc *ByteCode, pc int) {
	_ = c.Push(CallFrame{
		Symbols:    c.symbols,
		Bytecode:   c.bc,
		SingleStep: c.singleStep,
		Tokenizer:  c.tokenizer,
		This:       c.this,
		PC:         c.pc,
		FP:         c.fp,
		Module:     c.bc.Name,
		Line:       c.line,
	})

	c.fp = c.sp
	c.result = nil
	c.symbols = symbols.NewChildSymbolTable(tableName, c.symbols)
	c.line = 0
	c.bc = bc
	c.pc = pc

	// Now that we've saved state on the stack, if we are in step-over mode,
	// then turn of single stepping
	if c.singleStep && c.stepOver {
		c.singleStep = false
	}
}

// PopFrame retrieves the call frame information from the stack, and updates
// the current bytecode context to reflect the previously-stored state.
func (c *Context) PopFrame() error {
	// First, is there stuff on the stack we want to preserve?
	topOfStackSlice := c.stack[c.fp : c.sp+1]

	// Now retrieve the runtime context stored on the stack and
	// indicated by the fp (frame pointer)
	c.sp = c.fp
	cx, err := c.Pop()

	if err != nil {
		return err
	}

	if callFrame, ok := cx.(CallFrame); ok {
		c.line = callFrame.Line
		c.symbols = callFrame.Symbols
		c.singleStep = callFrame.SingleStep
		c.tokenizer = callFrame.Tokenizer
		c.this = callFrame.This
		c.bc = callFrame.Bytecode
		c.pc = callFrame.PC
		c.fp = callFrame.FP
	} else {
		return c.NewError(InvalidCallFrame)
	}

	// Finally, if there _was_ stuff on the stack after the call,
	// it might be a multi-value return, so push that back.
	if len(topOfStackSlice) > 0 {
		c.stack = append(c.stack[:c.sp], topOfStackSlice...)
		c.sp = c.sp + len(topOfStackSlice)
	} else {
		// Alternatively, it could be a single-value return using the
		// result holder. If so, push that on the stack and clear it.
		if c.result != nil {
			err = c.Push(c.result)
			c.result = nil
		}
	}

	return err
}

func (c *Context) FormatFrames(maxDepth int) string {
	f := c.fp
	depth := 1
	r := fmt.Sprintf("Call frames:\n  at: %12s  (%s)\n",
		formatLocation(c.GetModuleName(), c.line), c.symbols.Name)

	for (maxDepth < 0 || depth < maxDepth) && f > 0 {
		fx := c.stack[f-1]

		if frame, ok := fx.(CallFrame); ok {
			r = r + fmt.Sprintf("from: %12s  (%s)\n",
				formatLocation(frame.Module, frame.Line), frame.Symbols.Name)
			f = frame.FP

			depth++
		} else {
			break
		}
	}

	return r
}

func formatLocation(module string, line int) string {
	return fmt.Sprintf("%s %d", module, line)
}
