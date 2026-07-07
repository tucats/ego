package bytecode

import (
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
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
		c.pushThis(name, v)
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
		c.pushThis(name, v)
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
		this    string
		byValue bool
	)

	if operands, ok := i.([]any); ok && len(operands) == 2 {
		this = data.String(operands[0])
		byValue, _ = operands[1].(bool)
	} else {
		this = data.String(i)
	}

	if v, ok := c.popThis(); ok {
		// Auto-deref: if the caller passed a pointer variable (&T) as the
		// receiver, the value on the "this" stack is *any (an Ego pointer).
		// Dereference it to the underlying value so that both value receivers
		// (which need a struct they can copy with $new) and pointer receivers
		// (whose field writes propagate through the Go *data.Struct pointer)
		// work correctly when called on a pointer variable.
		if ptr, ok := v.(*any); ok && ptr != nil {
			v = *ptr
		}

		// BUG-64 fix: a genuine pointer receiver (byValue == false) must end
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

		c.setAlways(this, v)
		c.symbols.MarkEphemeral(this)

		if ui.IsActive(ui.TraceLogger) {
			ui.Log(ui.TraceLogger, "trace.getthis", ui.A{
				"name":  this,
				"value": data.Format(v)})
		}
	}

	return nil
}

// pushThis adds a receiver value to the "this" stack.
func (c *Context) pushThis(name string, v any) {
	if c.receiverStack == nil {
		c.receiverStack = []this{}
	}

	c.receiverStack = append(c.receiverStack, this{name, v})
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
