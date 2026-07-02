package bytecode

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// rangeDefinition holds all state for a single active for-range loop.
//
// One instance is created by RangeInit and pushed onto c.rangeStack.  The
// RangeNext opcode reads the top entry on every iteration to decide whether
// to continue or to branch past the loop.  Nesting works naturally because
// each loop pushes its own entry: the inner loop is always on top.
type rangeDefinition struct {
	// indexName / valueName are the names of the Ego variables that receive the
	// loop index (key) and loop value on each iteration.  Either may be ""
	// (not declared) or defs.DiscardedVariable ("_"), in which case the
	// corresponding symbol is skipped — it was never created and must not be
	// written to.
	indexName string
	valueName string

	// value is the thing being iterated.  Its concrete type determines which
	// rangeNext* helper is called.  For integer ranges the original value is
	// coerced to a plain int so the comparison arithmetic is uniform.
	value any

	// keySet holds the ordered set of keys for types where the full key list
	// is known up-front: map keys (arbitrary order, snapshotted at init time)
	// and string byte-offsets.
	keySet []any

	// runes holds the decoded rune values for a string range.  keySet[i] is the
	// byte offset of the rune; runes[i] is the rune itself.  Both slices are the
	// same length and are indexed by r.index.
	runes []rune

	// index is the current position within the range.  It starts at 0 and is
	// incremented by every successful rangeNext* call.
	index int

	// scopeDepth records the value of c.blockDepth at the moment RangeInit ran.
	// popScopeByteCode uses this to detect range entries that belong to the
	// scope being popped, so it can release their resources even when the loop
	// exits early via break or return (RANGE-3 fix).
	scopeDepth int

	// cleanup is an optional function called exactly once when this range entry
	// is retired — either because the iterator is exhausted normally, or because
	// the enclosing scope is popped before exhaustion (early break/return).
	//
	// Currently only *data.Map ranges set a cleanup: they lock the map readonly
	// during iteration and must unlock it on exit.  All other types leave
	// cleanup nil.
	cleanup func()
}

// release calls the cleanup function once, then clears it so it can never
// fire a second time.  It is safe to call on a rangeDefinition that has no
// cleanup (cleanup == nil).
//
// Why nil-out after calling?  Without this guard, the same entry could be
// released both by rangeNextMap on exhaustion AND by popScopeByteCode on
// the subsequent PopScope, calling SetReadonly(false) twice and under-
// decrementing the map's immutability semaphore.
func (r *rangeDefinition) release() {
	if r.cleanup != nil {
		r.cleanup()
		r.cleanup = nil
	}
}

// rangeInitByteCode implements the RangeInit opcode.
//
// Inputs:
//
//	operand  - a []any{indexVarName, valueVarName} slice naming the loop
//	           variables.  Either name can be "" (not used) or "_" (discarded).
//	           If the operand is nil or not a two-element slice, both names
//	           default to "" — the loop body runs but no variables are updated.
//	stack+0  - the value to iterate: string, *data.Map, *data.Array,
//	           *data.Channel, or any integer-width numeric type.
//
// What this instruction does:
//  1. Parses the variable names from the operand and creates the corresponding
//     symbols in the current scope (skipping "" and "_").
//  2. Pops the iteration target from the stack and validates its type.
//  3. For strings: pre-computes a parallel (byte-offset, rune) pair list so
//     each step can work in O(1).
//  4. For maps: snapshots the key set and marks the map readonly to prevent
//     structural changes during iteration.
//  5. Pushes a rangeDefinition onto c.rangeStack so RangeNext can access it.
//
// RANGE-5 fix: the rangeDefinition is only pushed when no error occurred.
// Previously it was pushed unconditionally, leaving a stale entry on the stack
// even when an unsupported type was encountered.
func rangeInitByteCode(c *Context, i any) error {
	var (
		v   any
		err error
		r   = rangeDefinition{}
	)

	// ── Parse the variable-name operand ───────────────────────────────────────
	//
	// The operand is expected to be []any{"indexName", "valueName"}.  If it is
	// anything else (nil, wrong length, wrong type) we silently treat both
	// names as empty — which means no loop variables will be written.  The
	// iteration itself still runs.
	if list, ok := i.([]any); ok && len(list) == 2 {
		r.indexName = data.String(list[0])
		r.valueName = data.String(list[1])

		// Create the index symbol in the current scope (not "_" and not "").
		if r.indexName != "" && r.indexName != defs.DiscardedVariable {
			err = c.symbols.Create(r.indexName)
		}

		// Only create the value symbol if the index creation succeeded.
		if err == nil && r.valueName != "" && r.valueName != defs.DiscardedVariable {
			err = c.symbols.Create(r.valueName)
		}
	}

	// ── Pop and validate the iteration target ────────────────────────────────
	if err == nil {
		if v, err = c.Pop(); err == nil {
			// A StackMarker on the stack means a function returned no value.
			// That is never a valid iteration target.
			if isStackMarker(v) {
				return c.runtimeError(errors.ErrFunctionReturnedVoid)
			}

			r.value = v

			switch actual := v.(type) {
			case string:
				// For strings, build parallel slices: keySet[i] = byte offset of
				// rune i, runes[i] = the rune itself.  Go's range-over-string
				// yields byte offsets, not sequential indices, so a multi-byte
				// UTF-8 character like '日' (3 bytes) gives offset 0 while '本'
				// gives offset 3, not 1.
				keySet := make([]any, 0)
				runes := make([]rune, 0)

				for i, ch := range actual {
					keySet = append(keySet, i)
					runes = append(runes, ch)
				}

				r.keySet = keySet
				r.runes = runes

			case *data.Map:
				// Snapshot the map's keys now.  If the loop body adds or removes
				// keys, the snapshot is unaffected — we iterate the original set.
				//
				// Lock the map readonly so that structural changes inside the loop
				// body raise an error rather than silently corrupting the iterator.
				//
				// RANGE-3 fix: store a cleanup closure that unlocks the map.  The
				// closure is called by release(), which fires either when the
				// iterator is exhausted normally (in rangeNextMap) or when the
				// enclosing scope is popped early by popScopeByteCode — e.g. via
				// break or return.  Without this, an early exit left the map locked
				// for the rest of the function's lifetime.
				r.keySet = actual.Keys()
				actual.SetReadonly(true)
				
				r.cleanup = func() { actual.SetReadonly(false) }

			case *data.Array:
				// Arrays are fixed-size in Ego, so structural modification during
				// iteration is impossible.  Element writes (a[i] = v) inside the
				// loop body are allowed and match Go semantics.

			case *data.Channel:
				// Channels deliver values on demand via Receive().  No pre-
				// computation needed; rangeNextChannel pulls one item per step.

			case int, int32, uint32, uint16, byte, int16, int64, int8, float32, float64:
				// Integer (and float) ranges produce indices 0 .. n-1.
				// Coerce to plain int so rangeNextInteger can use simple integer
				// comparison without reflection or type-switching.
				r.value, _ = data.Int(actual)

			default:
				// Unsupported type — return a runtime error with module/line info.
				err = c.runtimeError(errors.ErrInvalidType)
			}

			// ── Push the range definition onto the context's range stack ──────
			//
			// RANGE-5 fix: only push when no error occurred.  Previously, the push
			// was unconditional: even when the type-switch default set err, both
			// r.index = 0 and the append ran.  That left a stale entry on the stack
			// pointing at an invalid value type, which could cause confusing
			// behavior in the extremely unlikely scenario where the error was caught
			// by a try/catch block and a subsequent RangeNext was executed.
			if err == nil {
				r.index = 0

				// Record the scope depth so popScopeByteCode can find this entry
				// if the loop exits early without exhausting the iterator (RANGE-3).
				r.scopeDepth = c.blockDepth

				c.rangeStack = append(c.rangeStack, &r)
			}
		}
	}

	return err
}

// rangeNextByteCode implements the RangeNext opcode.
//
// Inputs:
//
//	operand  - the bytecode address to branch to when the range is exhausted.
//
// On each call, this opcode examines the topmost entry on c.rangeStack.  If
// that entry is exhausted (index past the end) it:
//   - sets c.programCounter to the destination address (branching past the loop)
//   - removes the entry from c.rangeStack
//   - calls the entry's cleanup (if any)
//
// If the entry is not exhausted, it updates the loop variables in the symbol
// table and increments the internal index, then returns without branching.
func rangeNextByteCode(c *Context, i any) error {
	var err error

	// The operand must be an integer bytecode address.
	destination, err := data.Int(i)
	if err != nil {
		return c.runtimeError(err)
	}

	if stackSize := len(c.rangeStack); stackSize == 0 {
		// No active range loop — branch to destination as a safe no-op.
		// This should not happen in well-formed bytecode, but guards against
		// the case where RangeNext is somehow executed without a prior RangeInit.
		c.programCounter = destination
	} else {
		r := c.rangeStack[stackSize-1]

		switch actual := r.value.(type) {
		case string:
			err = rangeNextString(c, r, destination, stackSize)

		case *data.Map:
			err = rangeNextMap(c, r, actual, destination, stackSize)

		case *data.Channel:
			err = rangeNextChannel(c, r, actual, destination, stackSize)

		case *data.Array:
			err = rangeNextArray(c, r, actual, destination, stackSize)

		case []any:
			// []any cannot be pushed onto the rangeStack by rangeInitByteCode
			// (which rejects it with ErrInvalidType).  This case is therefore
			// unreachable in well-formed programs, but is kept as a defensive
			// guard.
			//
			// RANGE-4 fix: use c.runtimeError() so the returned error carries
			// the current module name and source line, consistent with every
			// other error return in this package.  Previously this returned a
			// raw errors.Error with no location context.
			return c.runtimeError(errors.ErrInvalidType)

		case int:
			err = rangeNextInteger(c, r, actual, destination, stackSize)

		default:
			// Unknown value type.  Branch to destination (end the loop) and pop
			// the stale range entry so it does not corrupt outer loops.
			//
			// RANGE-2 fix: the original code set c.programCounter but did NOT trim
			// c.rangeStack, leaving a stale entry that could interfere with an
			// enclosing for-range loop on its next RangeNext call.  Now we always
			// pop the entry, matching the behavior of every other exhaustion path.
			c.programCounter = destination
			c.rangeStack = c.rangeStack[:stackSize-1]
		}
	}

	return err
}

// rangeNextInteger advances a for-range loop over an integer count.
//
// Integer ranges express "repeat N times" semantics: indices run 0 .. N-1.
// The r.value field holds the upper bound N (already coerced to int by
// rangeInitByteCode).
//
// RANGE-1 fix: the original code called c.symbols.Set(r.indexName, r.index)
// unconditionally.  If the index variable was discarded ("_") or empty (""),
// rangeInitByteCode skipped creating the symbol — so the subsequent Set call
// failed with ErrUnknownSymbol and the loop aborted.
//
// All other rangeNext* helpers guard this with:
//
//	if r.indexName != "" && r.indexName != defs.DiscardedVariable { ... }
//
// rangeNextInteger now has the same guard, making the behavior consistent.
func rangeNextInteger(c *Context, r *rangeDefinition, actual int, destination int, stackSize int) error {
	var err error

	// Exhaust when the bound is zero-or-negative (no iterations at all) or when
	// the running index has reached the bound.
	if (actual <= 0) || (r.index >= actual) {
		c.programCounter = destination
		c.rangeStack = c.rangeStack[:stackSize-1]
	} else {
		// Only store the index when the caller declared a real variable for it.
		// Skip when the name is "" (not declared) or "_" (deliberately discarded).
		if r.indexName != "" && r.indexName != defs.DiscardedVariable {
			err = c.symbols.Set(r.indexName, r.index)
		}

		r.index++
	}

	return err
}

// rangeNextArray advances a for-range loop over a *data.Array.
//
// On each step the index variable receives the zero-based element index and
// the value variable receives the element value.  Either variable is skipped
// if its name is "" or "_".
func rangeNextArray(c *Context, r *rangeDefinition, actual *data.Array, destination int, stackSize int) error {
	var err error

	if r.index >= actual.Len() {
		c.programCounter = destination
		c.rangeStack = c.rangeStack[:stackSize-1]
	} else {
		if r.indexName != "" && r.indexName != defs.DiscardedVariable {
			err = c.symbols.Set(r.indexName, r.index)
		}

		if err == nil && r.valueName != "" && r.valueName != defs.DiscardedVariable {
			var d any

			d, err = actual.Get(r.index)
			if err == nil {
				err = c.symbols.Set(r.valueName, d)
			}
		}

		r.index++
	}

	return err
}

// rangeNextChannel advances a for-range loop over a *data.Channel.
//
// Each step calls Receive() on the channel.  Any receive error (including
// the "channel is closed and drained" case) is treated as normal loop
// termination — the program counter jumps to the destination and the range
// entry is popped.
//
// # Variable assignment for channel ranges
//
// Channel ranges have different assignment semantics depending on how many
// loop variables the caller declared:
//
// Two-variable form:  for i, v := range ch { ... }
//
//	The compiler sets r.indexName="i" and r.valueName="v".
//	i receives the loop counter (r.index: 0, 1, 2, …) and v receives
//	the channel value (datum).  This mirrors how two-variable array ranges
//	work (index, element).
//
// Single-variable form:  for v := range ch { ... }
//
//	The compiler sets r.indexName="v" and r.valueName="".
//	Because channels have no meaningful positional index — values arrive
//	in send order and a counter adds no information — the sole declared
//	variable must receive the channel value (datum).  Assigning the loop
//	counter to it instead was BUG-01: the variable would read 0, 1, 2, …
//	instead of the transmitted values.
//
// No-variable form:  for range ch { ... }
//
//	Both names are "" or "_".  The value is consumed and discarded; no
//	symbols are written.
func rangeNextChannel(c *Context, r *rangeDefinition, actual *data.Channel, destination int, stackSize int) error {
	datum, err := actual.Receive()
	if err != nil {
		// Receive failed — treat this as normal exhaustion regardless of the
		// specific error code (closed channel, timeout, etc.).
		c.programCounter = destination
		c.rangeStack = c.rangeStack[:stackSize-1]

		return nil
	}

	valueNamePresent := r.valueName != "" && r.valueName != defs.DiscardedVariable

	if valueNamePresent {
		// Two-variable form (for i, v := range ch):
		//   indexName → loop counter, valueName → received value.
		if r.indexName != "" && r.indexName != defs.DiscardedVariable {
			if err = c.symbols.Set(r.indexName, r.index); err != nil {
				return err
			}
		}

		if err = c.symbols.Set(r.valueName, datum); err != nil {
			return err
		}
	} else {
		// Single-variable form (for v := range ch) or no-variable form (for range ch):
		//   indexName → received value.  The loop counter is meaningless for
		//   channel receives and is not exposed to the caller.
		if r.indexName != "" && r.indexName != defs.DiscardedVariable {
			if err = c.symbols.Set(r.indexName, datum); err != nil {
				return err
			}
		}
	}

	r.index++

	return nil
}

// rangeNextMap advances a for-range loop over a *data.Map.
//
// The keys were snapshotted by rangeInitByteCode into r.keySet, so this
// function simply walks that slice rather than querying the live map on every
// step.  If a key was deleted inside the loop body the live Get will fail, and
// the value variable is set to nil to indicate "key gone" rather than aborting.
//
// On exhaustion, r.release() is called to unlock the map's readonly state.
// The RANGE-3 fix stores the unlock call in r.cleanup (set by rangeInitByteCode)
// rather than calling actual.SetReadonly(false) directly, so the same path works
// whether exhaustion is reached normally here or via an early break (which
// triggers release through popScopeByteCode).
func rangeNextMap(c *Context, r *rangeDefinition, actual *data.Map, destination int, stackSize int) error {
	var err error

	if r.index >= len(r.keySet) {
		// Iteration is complete.  Pop the range entry and release resources.
		c.programCounter = destination
		c.rangeStack = c.rangeStack[:stackSize-1]

		// Release the map's readonly lock (RANGE-3 fix: uses r.release() so
		// double-release is impossible even if popScopeByteCode also fires).
		r.release()
	} else {
		key := r.keySet[r.index]

		if r.indexName != "" && r.indexName != defs.DiscardedVariable {
			err = c.symbols.Set(r.indexName, key)
		}

		if err == nil && r.valueName != "" && r.valueName != defs.DiscardedVariable {
			var value any

			ok := false
			if value, ok, err = actual.Get(key); ok && err == nil {
				err = c.symbols.Set(r.valueName, value)
			} else {
				// The key was deleted inside the loop body.  Set the value variable
				// to nil rather than propagating an error.
				err = c.symbols.Set(r.valueName, nil)
			}
		}

		r.index++
	}

	return err
}

// rangeNextString advances a for-range loop over a string.
//
// The byte-offset/rune pairs were pre-computed by rangeInitByteCode into
// r.keySet (byte offsets) and r.runes (decoded runes).  The index variable
// receives the byte offset (matching Go semantics — multi-byte UTF-8
// characters produce non-consecutive offsets) and the value variable receives
// the decoded rune itself.
//
// fixed BUG-19: in Go, `for i, ch := range someString` gives `ch` the type
// `rune`, which is just an alias for `int32` — it holds the Unicode code
// point number (e.g. 65 for 'A'), not a one-character string. The previous
// Ego implementation converted the rune to a `string(value)` before storing
// it, so `ch` came out as a single-character string like "A" instead of the
// integer 65. That silently diverged from documented Go behavior and could
// trip up anyone porting Go code, or anyone who expected to do arithmetic on
// the loop value (e.g. checking if a rune is in the range of digits).
//
// `value` below is already of Go type `rune`, which — because `rune` is
// defined as `type rune = int32` in the Go language — is stored in the
// symbol table as a plain `int32`. No explicit conversion is needed; we
// simply stop wrapping it in `string(...)`.
func rangeNextString(c *Context, r *rangeDefinition, destination int, stackSize int) error {
	var err error

	if r.index >= len(r.keySet) {
		c.programCounter = destination
		c.rangeStack = c.rangeStack[:stackSize-1]
	} else {
		key := r.keySet[r.index]  // byte offset of the current rune
		value := r.runes[r.index] // the rune itself (Go type: rune == int32)

		if r.indexName != "" && r.indexName != defs.DiscardedVariable {
			err = c.symbols.Set(r.indexName, key)
		}

		if err == nil && r.valueName != "" && r.valueName != defs.DiscardedVariable {
			// Store the rune as-is (an int32 code point), matching Go's
			// for _, ch := range s { ... } where ch has type rune.
			err = c.symbols.Set(r.valueName, value)
		}

		r.index++
	}

	return err
}
