package bytecode

import (
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
)

// setThisByteCode implements the SetThis opcode. Given a named value,
// the current value is pushed on the "this" stack as part of setting
// up a call, to be retrieved later by the body of the call. If there
// is no name operand, assume the top stack value is to be used, and
// synthesize a name for it.
func setThisByteCode(c *Context, i any) error {
	var name string

	if i == nil {
		v, err := c.Pop()
		if err != nil {
			return err
		}

		_ = c.push(v)
		name = data.GenerateName()
		c.setAlways(name, v)
	} else {
		name = data.String(i)
	}

	if v, ok := c.get(name); ok {
		c.PushThis(name, v)
	}

	return nil
}

// loadThisByteCode implements the LoadThis opcode. This combines the
// functionality of the Load followed by the SetThis opcodes.
func loadThisByteCode(c *Context, i any) error {
	var (
		found bool
		this  any
	)

	// If the operand is a name, look up the value in the
	// symbol table. If it's not a name, then it's a value for
	// a receiver and we use it as-is.
	if name, ok := i.(string); ok {
		if len(name) == 0 {
			return c.runtimeError(errors.ErrInvalidIdentifier)
		}

		this, found = c.get(name)
		if !found {
			return c.runtimeError(errors.ErrUnknownIdentifier).Context(name)
		}
	} else {
		this = i
	}

	// Assign the value to a generated name and put it on the receiver stack.
	_ = c.push(this)
	name := data.GenerateName()
	c.setAlways(name, this)

	if v, ok := c.get(name); ok {
		c.PushThis(name, v)
	}

	return nil
}

// getThisByteCode implements the GetThis opcode. Given a value name,
// get the top-most item from the "this" stack and store it in the
// named value. This is done as part of prologue of a function that
// has a receiver.
//
// The operand is either a plain name (for backward compatibility with any
// other caller that never needs the BUG-64 fix below), or a two-element
// []any{name, byValue} as emitted by generateFunctionBytecode in
// internal/language/compiler/function.go, where byValue reports whether the
// receiver was declared WITHOUT a pointer (e.g. "func (b Builder) …", as
// opposed to "func (b *Builder) …").
func getThisByteCode(c *Context, i any) error {
	var (
		this      string
		slotIndex = -1
		byValue   bool
	)

	// docs/SLOTS.md Phase 2: in a slot-eligible method the receiver is bound to
	// a slot instead of by name. The compiler signals this by passing an int
	// slot index as operand[0] (a plain string name otherwise).
	if operands, ok := i.([]any); ok && len(operands) == 2 {
		if idx, isInt := operands[0].(int); isInt {
			slotIndex = idx
		} else {
			this = data.String(operands[0])
		}

		byValue, _ = operands[1].(bool)
	} else {
		this = data.String(i)
	}

	// Consume the value callBytecodeFunction staged for this call (see the
	// comment on Context.pendingReceiver and CALL-11 in docs/ISSUES.md).
	// This used to pop c.receiverStack directly, which had no way to
	// distinguish "nothing was ever pushed for me" from "an unrelated call's
	// leftover entry is sitting here" -- callBytecodeFunction now guarantees
	// pendingReceiver/pendingReceiverOK reflect exactly this call, and only
	// this call, every single time.
	if v, ok := c.pendingReceiver, c.pendingReceiverOK; ok {
		c.pendingReceiver = nil
		c.pendingReceiverOK = false

		// Auto-deref: if the caller passed a pointer variable (&T) as the
		// receiver, the value on the "this" stack is *any (an Ego pointer).
		// Dereference it to the underlying value so that both value receivers
		// (which need a struct they can copy with $new) and pointer receivers
		// (whose field writes propagate through the Go *data.Struct pointer)
		// work correctly when called on a pointer variable.
		if ptr, ok := v.(*any); ok && ptr != nil {
			v = *ptr
		}

		// Fix BUG-64: a genuine pointer receiver (byValue == false) must end
		// up bound to a value the Ego type system recognizes as a pointer —
		// data.TypeOf only recognizes a boxed *any as a pointer type; a bare
		// *data.Struct (or other reference type) on its own resolves to its
		// plain (non-pointer) type. Without this, a pointer-receiver method
		// that returns the receiver as its declared "*T" type would fail a
		// strict-mode return-type check, because the receiver's runtime type
		// no longer matched what was declared.
		//
		// This covers two call shapes with the same code:
		//
		//   - Called via an explicit pointer variable (pb.add(...)): the
		//     deref above already unwrapped the *any, so v here is the bare
		//     *data.Struct that was inside it. Re-boxing restores the
		//     pointer marker.
		//   - Called via Go's "auto-address" rule, directly on an
		//     addressable, non-pointer value (b.add(...) where "add" has a
		//     pointer receiver): v was never *any to begin with (the deref
		//     above was a no-op), so it is boxed here for the first time —
		//     matching what Go does implicitly by taking &b at the call.
		//
		// Either way, boxing does not change the underlying value's
		// identity — "boxed" holds the very same *data.Struct (or other
		// reference type) pointer that field mutation already propagates
		// through — it only adds the outer *any wrapper that marks it as an
		// Ego pointer, exactly matching how an ordinary "*T" parameter (not
		// a receiver) is already represented. Ego's member-access bytecode
		// already transparently dereferences a boxed *any for field
		// read/write (this is exactly how mutation through a plain "*T"
		// parameter has always worked), so this has no effect on mutation
		// behavior.
		//
		// A value receiver (byValue == true) is left as the bare
		// dereferenced/original value, exactly as before, because the
		// $new() copy that generateFunctionBytecode emits immediately after
		// GetThis (see function.go) expects a plain value to copy, not a
		// boxed pointer.
		if !byValue {
			boxed := v
			v = &boxed
		}

		// docs/SLOTS.md Phase 2: a slotted receiver is written into the slot
		// bank. No MarkEphemeral is needed -- the bank belongs to this one call
		// activation and is discarded with the boundary table, and proxy/clone
		// tables copy the symbols map (not the bank), so a slotted receiver is
		// ephemeral by construction.
		if slotIndex >= 0 {
			bank := c.symbols.LocalsBank()
			if bank == nil {
				return c.runtimeError(errors.ErrInternalCompiler).Context("GetThis: no locals bank in scope")
			}

			if !bank.SetRegister(slotIndex, v) {
				return c.runtimeError(errors.ErrInternalCompiler).Context("GetThis: slot index out of range")
			}
		} else {
			c.setAlways(this, v)
			c.symbols.MarkEphemeral(this)
		}

		if ui.IsActive(ui.TraceLogger) {
			ui.Log(ui.TraceLogger, "trace.getthis", ui.A{
				"name":  this,
				"value": data.Format(v)})
		}
	}

	return nil
}

// PushThis adds a receiver value to the "this" stack. This has
// been made an exported function because it's used when the
// formatter needs to call a String() function on a receiver
// (and possibly other cases).
func (c *Context) PushThis(name string, v any) {
	if c.receiverStack == nil {
		c.receiverStack = []this{}
	}

	c.receiverStack = append(c.receiverStack, this{name, v})
}

// SetPendingReceiver stages v as the receiver value for the *next* GetThis
// opcode this Context executes, exactly as if callBytecodeFunction had just
// been called with (v, true) -- see the Context.pendingReceiver comment and
// CALL-11 in docs/ISSUES.md.
//
// This exists for the one caller (internal/runtime/fmt's formatUsingString,
// which formats a value by directly invoking its String() method's compiled
// bytecode) that constructs and runs a *bytecode.Context by hand instead of
// going through the normal callByteCode/callBytecodeFunction dispatch path.
// That caller has no Call instruction of its own for callByteCode to gate a
// receiver-stack pop on, so it must stage the receiver value directly, the
// same way callBytecodeFunction would have. Do not use this from within the
// bytecode package itself -- everything reached through the normal Call
// dispatch already gets this staged automatically.
func (c *Context) SetPendingReceiver(v any) {
	c.pendingReceiver = v
	c.pendingReceiverOK = true
}

// popThis removes a receiver value from this "this" stack.
func (c *Context) popThis() (any, bool) {
	if len(c.receiverStack) == 0 {
		return nil, false
	}

	this := c.receiverStack[len(c.receiverStack)-1]
	c.receiverStack = c.receiverStack[:len(c.receiverStack)-1]

	return this.value, true
}
