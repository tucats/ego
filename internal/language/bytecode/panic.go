package bytecode

import (
	"fmt"

	"github.com/tucats/ego/internal/errors"
)

// userPanicByteCode implements the UserPanic opcode, which is emitted for the
// Ego panic() built-in. Unlike the Panic opcode (used by @fail and fatal
// conditions), UserPanic starts a recoverable unwind: it sets the panic state
// on the context and returns ErrPanicActive so that the run loop can drive the
// frame-by-frame unwind via unwindPanic().
//
// If an operand is provided it is used as the panic value; otherwise the value
// is popped from the stack.
func userPanicByteCode(c *Context, i any) error {
	var panicValue any

	if i != nil {
		panicValue = i
	} else {
		v, err := c.Pop()
		if err != nil {
			return err
		}

		panicValue = v
	}

	c.panicActive = true
	c.panicValue = panicValue

	return errors.ErrPanicActive
}

// recoverByteCode implements the Recover opcode. recover() is only meaningful
// inside a deferred function that was invoked during panic unwinding. It walks
// the panicContext chain from the current (child) context back to the context
// that is actively panicking, clears the panic state there, and pushes the
// panic value onto the current stack. If no panic is in progress it pushes nil.
func recoverByteCode(c *Context, i any) error {
	// Walk the panicContext chain to find a context that is actively panicking.
	panicking := findPanickingContext(c)

	if panicking == nil {
		// Not called during panic unwinding: recover() returns nil.
		return c.push(nil)
	}

	// Retrieve and clear the panic state on the panicking context.
	value := panicking.panicValue
	panicking.panicActive = false
	panicking.panicValue = nil

	// Push the panic value so the caller can inspect it.
	return c.push(value)
}

// findPanickingContext walks the panicContext chain starting from ctx and returns
// the first context that has panicActive == true. Returns nil if not found.
func findPanickingContext(ctx *Context) *Context {
	for ctx != nil {
		if ctx.panicActive {
			return ctx
		}

		ctx = ctx.panicContext
	}

	return nil
}

// unwindPanic is called by the run loop when it receives ErrPanicActive. It
// drives the frame-by-frame unwinding: for each frame on the call stack it
// runs any deferred functions (giving them a chance to call recover()), then
// pops the frame.
//
// If a deferred function calls recover() and clears the panic state, unwindPanic
// clears the defer stack (so it is not re-run by a later RunDefers opcode), pops
// the current call frame, and returns nil so the run loop resumes in the caller
// of the panicking function. Otherwise, once all frames are exhausted, it prints
// a fatal panic message and returns ErrStop.
func (c *Context) unwindPanic() error {
	for {
		// Run the deferred functions for the current frame; any of them may call
		// recover() to clear panicActive on the context.
		if len(c.deferStack) > 0 {
			if err := c.invokePanicDefers(); err != nil {
				return err
			}
		}

		if !c.panicActive {
			// A deferred function called recover(). Clear the defer stack so that
			// the RunDefers opcode (which is about to execute in the resumed
			// function) does not re-run them.
			c.deferStack = []deferStatement{}

			// Pop the current call frame so execution resumes in the caller of
			// the panicking function rather than continuing inside it.
			if c.framePointer == 0 {
				// No call frame to pop — we are at the top level. Stop normally.
				c.running.Store(false)

				return errors.ErrStop
			}

			// Discard the panicking function's locals and any partial expression
			// results that were left on the stack when the panic interrupted
			// execution.  We reset to the frame pointer before synthesizing
			// return values so that callFramePop's topOfStackSlice logic picks
			// up only those synthesized values and not stale stack slots.
			c.stackPointer = c.framePointer

			// Synthesize return values for the recovered function (BUG-04).
			//
			// In Go, a function that panics and has recover() called in a
			// deferred function returns normally to its caller.  The caller
			// receives:
			//   • Named return variables — their current value in the symbol
			//     table (the deferred function may have modified them).
			//   • Unnamed return values  — nil, to unambiguously indicate that
			//     no useful value was set before the panic.
			//   • Void functions         — nothing (this branch is unreachable).
			//
			// Without this block, callFramePop finds an empty stack and no
			// c.result, so the caller's assignment instruction (CreateAndStore
			// or StackCheck) sees the stack marker from before the call and
			// reports "function did not return the expected number of values".
			//
			// Implementation notes:
			//   • c.bc is still the panicking function's bytecode; its
			//     Declaration() carries the return-type list and its
			//     GetReturnVarNames() carries the names of any named return
			//     variables (set by the compiler in generateFunctionBytecode).
			//   • c.symbols still points into the panicking function's scope
			//     chain, so Get() can walk up to the named-return scope even
			//     when the panic happened inside a nested if/for block.
			//   • For a single return value we use the c.result / c.resultSet
			//     fields, which callFramePop pushes onto the caller's stack.
			//     For multiple return values we push the values above the frame
			//     pointer directly; callFramePop collects them as topOfStackSlice.
			if decl := c.bc.Declaration(); decl != nil && len(decl.Returns) > 0 {
				varNames := c.bc.GetReturnVarNames() // nil when returns are unnamed

				if len(decl.Returns) == 1 {
					// Single return — use the c.result shortcut.
					if len(varNames) == 1 {
						// Named single return: read the current value from the
						// symbol table.  The variable was zero-initialized at
						// function entry and may have been modified by the
						// deferred function that called recover().
						val, found := c.symbols.Get(varNames[0])
						if !found {
							// Defensive fallback: the scope was somehow unavailable.
							val = nil
						}

						c.result = val
					} else {
						// Unnamed single return: nil signals "no useful value".
						c.result = nil
					}

					c.resultSet = true
				} else {
					// Multiple returns — push the function's stack marker followed
					// by the values in reverse declaration order, mirroring what
					// compileReturn emits for the normal (non-panic) return path.
					//
					// The stack marker is required: StackCheck (which the caller's
					// multi-assignment compilation emits) scans down from the top
					// looking for any StackMarker to verify the return values are
					// present.  Without it, StackCheck fails with
					// ErrReturnValueCount even when the value count is correct.
					//
					// Reverse order ensures the first declared variable ends up on
					// top so the caller's left-to-right assignment pops in the
					// right order.
					_ = c.push(NewStackMarker(c.bc.name, len(decl.Returns)))

					for i := len(decl.Returns) - 1; i >= 0; i-- {
						// val is nil by default.  For named return variables we
						// replace nil with the variable's current symbol-table
						// value, which may be the compiler-initialized zero value
						// or a value the deferred function explicitly set.
						// For unnamed returns val stays nil — that unambiguously
						// signals "no useful value was set before the panic".
						var val any

						if i < len(varNames) {
							// Named return variable: read its current value.
							// Get() walks up the scope-parent chain, reaching the
							// named-return scope even when the panic occurred inside
							// a nested if/for/switch block.
							val, _ = c.symbols.Get(varNames[i])
						}

						// Push above the frame pointer.  c.push is safe here
						// because c.stackPointer was just reset to c.framePointer.
						_ = c.push(val)
					}
				}
			}

			// Restore the caller's context; any synthesized return values are
			// now either in c.result (single return) or on the stack above the
			// frame pointer (multiple returns) and will be transferred to the
			// caller's stack by callFramePop.
			return c.callFramePop()
		}

		// No recovery happened in this frame. Pop the frame and try the caller.
		if c.framePointer == 0 {
			break
		}

		// Discard anything above the frame pointer (locals, partial results).
		c.stackPointer = c.framePointer

		if err := c.callFramePop(); err != nil {
			break
		}
	}

	// Panic reached the top of the call stack with no recover() — fatal.
	c.running.Store(false)

	panicMessage := fmt.Sprintf("%v", c.panicValue)
	c.panicActive = false
	c.panicValue = nil

	fmt.Fprintf(c.output, "panic: %s\n", panicMessage)
	fmt.Fprint(c.output, c.FormatFrames(IncludeSymbolTableNames))

	return errors.ErrStop
}
