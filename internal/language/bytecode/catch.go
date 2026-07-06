package bytecode

import (
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// handleCatch processes any try/catch state that is in effect after an instruction
// executes. If there is no error, there is no action. If there is an error, then this
// code determines if the error qualifies to be considered "caught" and if so will
// redirect execution to a new instruction (and return nil indicating there is no
// longer an error condition). If no try/catch is active, or it specifies error(s)
// different than the one found, then it simply returns the error for further processing
// in the main run loop.
func handleCatch(c *Context, err error) error {
	// If there is no error, we're done.
	if err == nil || errors.Equals(err, errors.ErrStop) {
		return nil
	}

	// A panic-in-progress must not be absorbed by try/catch; let it pass through
	// so the run loop can drive the unwind via unwindPanic().
	if errors.Equals(err, errors.ErrPanicActive) {
		return err
	}

	text := err.Error()

	// See if we are in a try/catch block that will catch this error. We must
	// search c.tryStack from the top (innermost) down rather than only ever
	// looking at the top entry, for two reasons — both are variations on the
	// same theme: the innermost frame is not always the one that ends up
	// handling the error, so a mismatch there must not be treated as "nothing
	// can catch this."
	//
	//  1. (BUG-35) A tryInfo entry with addr == 0 means that level's catch
	//     block is already executing (handleCatch zeroed it the first time it
	//     redirected control there) — it cannot be re-entered, but an
	//     *enclosing* try/catch further down the stack may still be live. An
	//     error raised while a catch block itself is running (e.g. a division
	//     by zero inside `catch { ... }`) must escape to the nearest enclosing
	//     try, not be reported as uncaught just because the innermost frame is
	//     spent.
	//  2. (selective catch escalation) A live frame (addr > 0) may have a
	//     selective catches list — used internally by the `?` optional
	//     operator and by `catch` clauses that name specific error types —
	//     that does not include this error. That does not mean the error is
	//     uncaught: an outer try/catch further down the stack may catch
	//     everything (or catch this specific error), so the search must keep
	//     going past a non-matching selective frame instead of giving up on
	//     the first live frame it finds.
	//
	// Note that if the error was fatal, the running flag is turned off, which
	// prevents the try block from being honored (i.e. you cannot catch a fatal
	// error).
	tryIndex := -1

	if c.running.Load() {
		for i := len(c.tryStack) - 1; i >= 0; i-- {
			try := c.tryStack[i]

			// addr == 0: this level's catch block is already running (or was
			// never armed); it cannot catch a new error, so keep searching
			// further down the stack for an enclosing try/catch.
			if try.addr <= 0 {
				continue
			}

			// Do we have a selective set of things we catch? The default is
			// that we catch everything, but if the try info block has a list
			// of errors, then we only catch if the error is on that specific
			// list. If it doesn't match, this frame can't help — keep
			// searching further down for an enclosing try/catch instead of
			// stopping here.
			willCatch := true

			if len(try.catches) > 0 {
				willCatch = false

				for _, e := range try.catches {
					if e.(*errors.Error).Equal(err) {
						willCatch = true

						break
					}
				}
			}

			if willCatch {
				tryIndex = i

				break
			}
		}
	}

	if tryIndex >= 0 {
		try := c.tryStack[tryIndex]

		// Normally there is exactly one "try" StackMarker to consume: the one
		// belonging to the frame we matched. But if we skipped over live
		// frames above tryIndex because their selective catches list didn't
		// match this error (reason 2 in the search above), those frames were
		// never entered, so their "try" StackMarker is still sitting on the
		// execution stack, above the marker we actually want to stop at.
		// (Frames skipped because they were already spent — addr==0,
		// re: BUG-35 — do NOT count here: their marker was already consumed the
		// first time their own catch was entered.) Count how many markers
		// must be consumed so the unwind loop below doesn't stop prematurely
		// at an inner, bypassed frame's marker.
		markersToConsume := 1

		for i := tryIndex + 1; i < len(c.tryStack); i++ {
			if c.tryStack[i].addr > 0 {
				markersToConsume++
			}
		}

		// This could be in a function call tree within the try stack. So drop
		// the items on the stack until we get to the try marker. If, along the
		// way, we find a stack frame, then pop the stack frame as well so we
		// reset the state of the context back to the frame level where the try
		// was initiated.
		for {
			v, err := c.Pop()
			if err != nil {
				return err
			}

			// If its a call frame, put it back on the stack and then do the formal
			// pop of a call frame, which updates the state of the context.
			if f, ok := v.(*CallFrame); ok {
				ui.Log(ui.TraceLogger, "trace.unwind", ui.A{
					"thread": c.threadID,
					"module": f.Module,
					"line":   f.Line,
					"frame":  f.symbols.Name})

				if err := c.push(v); err != nil {
					return err
				}

				if err := c.callFramePop(); err != nil {
					return err
				}

				continue
			}

			// See if we've hit a "try" frame marker. Each nested, bypassed live
			// frame contributes one marker that must be consumed before we
			// reach the marker for the frame we are actually redirecting to.
			if isStackMarker(v, "try") {
				markersToConsume--

				if markersToConsume == 0 {
					break
				}
			}
		}

		// We are catching, so update the PC
		c.programCounter = try.addr

		// Discard any inner tryInfo frames that sit above the level we just matched.
		// Those frames belong to try/catch blocks nested inside this one; since we
		// are jumping directly into an enclosing catch block, execution will never
		// reach their TryPop instruction (it is skipped over), so they would
		// otherwise leak on c.tryStack forever. Truncating to tryIndex+1 removes
		// them along with any of their now-meaningless state.
		c.tryStack = c.tryStack[:tryIndex+1]

		// Zero out the jump point for this try/catch block so recursive
		// errors don't occur — a subsequent error raised while this catch block
		// is running will skip this now-inert frame and be matched against an
		// enclosing try/catch, if any (see the search loop above).
		c.tryStack[tryIndex].addr = 0

		// Record the error in the "__error" variable for use in the catch block if needed.
		c.symbols.SetAlways(defs.ErrorVariable, err)

		if ui.IsActive(ui.TraceLogger) {
			ui.Log(ui.TraceLogger, "trace.branch.catch", ui.A{
				"thread": c.threadID,
				"addr":   c.programCounter,
				"error":  text})
		}

		// Successfully redirected to a catch block, so no more error state.
		return nil
	}

	// No catch, so error still active.
	return err
}
