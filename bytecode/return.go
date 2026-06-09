package bytecode

// return.go implements the Return bytecode instruction, which exits the
// currently-executing Ego function and transfers control back to the caller.
//
// # How Ego function calls work (brief recap)
//
// When the Call instruction invokes an Ego function, callFramePush saves a
// snapshot of the entire runtime context (program counter, bytecode stream,
// symbol table, etc.) as a *CallFrame object pushed onto the stack.  The
// frame pointer (c.framePointer) is then advanced past the frame slot so the
// callee's own stack values always live above it.
//
//	stack layout during a call
//	──────────────────────────
//	stack[fp-1]  = *CallFrame   ← saved caller state
//	stack[fp]    = first slot callee can use
//	stack[...]   = local variables, temporaries, return values
//
// When Return executes, it undoes this by calling callFramePop, which:
//  1. Collects any values sitting above the frame pointer (multi-return path).
//  2. Restores the context from the saved *CallFrame.
//  3. Re-pushes collected values (or the single c.result) onto the caller's
//     stack.
//
// # Operand forms
//
// The compiler emits one of four operand forms:
//
//	Return(nil)      — void return; discard everything on the callee stack.
//	Return(true)     — single-value non-named return; pop one value from the
//	                   top of the stack and store it in c.result.
//	Return(int(1))   — single-value named return; same as bool=true but the
//	                   compiler may have placed a StackMarker below the value
//	                   to delimit the named-return slot.
//	Return(int(N))   — multi-value return (N >= 2); the N values are already
//	                   on the stack above the frame pointer; callFramePop will
//	                   transport them to the caller.
//
// # Top-level (framePointer == 0)
//
// When the main program body returns (there is no caller frame), framePointer
// is 0 and callFramePop is not called.  Instead, c.running is set to false,
// which signals the run loop to stop.

import "github.com/tucats/ego/errors"

// returnByteCode is the instruction handler for the Return opcode.
//
// It is responsible for:
//  1. Extracting the return value(s) from the callee's stack based on the
//     operand form (nil, bool, or int).
//  2. Syncing any package-level symbols back to the package object so that
//     side-effects of the function are visible to the caller.
//  3. Popping the saved *CallFrame and restoring the caller's context via
//     callFramePop — or stopping execution if we are at the top level.
//  4. Optionally signaling the debugger if breakOnReturn was set.
func returnByteCode(c *Context, i any) error {
	var err error

	// ── Determine how many values to return ──────────────────────────────────

	if b, ok := i.(bool); ok && b {
		// bool(true) operand: single non-named return value.
		//
		// The expression that computes the return value was already evaluated
		// and pushed onto the stack by the caller.  Pop it now and store it in
		// c.result so callFramePop can push it onto the caller's stack.
		c.result, err = c.Pop()

		// RETURN-2 fix: propagate a Pop failure immediately rather than letting
		// it be silently overwritten by callFramePop's error below.
		if err != nil {
			return c.runtimeError(err)
		}

		// RETURN-1 fix: check the field c.result (lowercase), not the bound
		// method c.Result.  A StackMarker on the stack where a value was
		// expected means the called function returned void — for example,
		//   x := voidFunc()
		// where voidFunc() left nothing meaningful on the stack.
		if isStackMarker(c.result) {
			return c.runtimeError(errors.ErrFunctionReturnedVoid)
		}

		c.resultSet = true

	} else if b, ok := i.(int); ok && b > 0 {
		// int(N) operand: N named return values (N >= 1).

		if b == 1 {
			// Single named return: pop the top value and store it.
			// Named-return functions place a StackMarker *below* the single
			// return value as a delimiter.  If the marker is present, discard
			// it so that callFramePop does not mistake it for a second return
			// value.
			c.result, err = c.Pop()
			c.resultSet = true

			if err == nil && c.stackPointer > c.framePointer {
				if isStackMarker(c.stack[c.stackPointer-1]) {
					_, err = c.Pop()
				}
			}
		} else {
			// Multiple named returns (N >= 2): the values are already sitting
			// on the callee's stack above the frame pointer.  callFramePop will
			// collect them via its topOfStackSlice logic and push them back onto
			// the caller's stack.  We do not pop them here.
			c.result = nil
			c.resultSet = false
		}

	} else {
		// nil operand (or int(0)): void return.
		//
		// Discard everything the callee left on the stack by resetting the
		// stack pointer.  Setting sp to fp-1 moves it below the *CallFrame
		// slot; callFramePop resets sp to fp before popping the frame, so
		// this pre-discard does not interfere with frame recovery.
		c.stackPointer = c.framePointer - 1
		c.result = nil
		c.resultSet = false
	}

	// ── Sync package symbols back to the package object ───────────────────────
	//
	// If this function was running inside a package scope (c.symbols.Parent
	// is the package's symbol table), any exported values that were modified
	// must be written back to the *data.Package object so the changes are
	// visible to code that holds a reference to the package variable.
	if err := c.syncPackageSymbols(); err != nil {
		return errors.New(err)
	}

	// ── Pop the call frame or stop execution ──────────────────────────────────
	//
	// callFramePop restores everything the caller had before the call:
	// program counter, bytecode stream, symbol table, module name, etc.
	// It also pushes any return values (from c.result or topOfStackSlice) onto
	// the newly-restored caller stack.
	//
	// If there is no frame (framePointer == 0), this is the top-level program
	// returning.  Setting c.running to false signals the run loop to stop.
	if c.framePointer > 0 {
		err = c.callFramePop()
	} else {
		c.running.Store(false)
	}

	// ── Debugger hook ─────────────────────────────────────────────────────────
	//
	// If the debugger had set breakOnReturn on the current frame (via "step
	// out" / "finish"), signal it now that the function has returned.
	// ErrSignalDebugger is a non-fatal sentinel that the run loop intercepts
	// to pause execution and present the debugger prompt.
	if err == nil && c.breakOnReturn {
		c.breakOnReturn = false

		return errors.ErrSignalDebugger
	}

	if err == nil {
		return err
	}

	return c.runtimeError(err)
}
