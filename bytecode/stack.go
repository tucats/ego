package bytecode

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	data "github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

/******************************************\
*                                         *
*    S T A C K   M A N A G E M E N T      *
*                                         *
\******************************************/

// StackMarker is a special object used to mark a location on the
// stack. It is used, for example, to mark locations to where a stack
// should be flushed. The marker contains a text description, and
// optionally any additional desired data.
type StackMarker struct {
	label  string
	values []any
}

// NewStackMarker generates a enw stack marker object, using the
// supplied label and optional list of data.
func NewStackMarker(label string, values ...any) StackMarker {
	if label == "" {
		label = defs.Anon
	}

	return StackMarker{
		label:  label,
		values: values,
	}
}

// See if the item is a marker. If there are no marker types, this
// just returns true if the interface given is any stack marker. If
// one or more value strings are passed, then in addition to being a
// marker, the item must contain at least one of the values as one of
// it data elements.
func isStackMarker(i any, values ...string) bool {
	// First, check special case of a call frame, which acts
	// as a marker but has lots of other data in it as well.
	if frame, ok := i.(*CallFrame); ok {
		ui.Log(ui.TraceLogger, "trace.unexp.callframe", ui.A{
			"module": frame.Module,
			"line":   frame.Line})

		return true
	}

	// Okay, see if it is a StackMarker. If not, we're done here.
	marker, ok := i.(StackMarker)
	if !ok {
		return false
	}

	if len(values) == 0 {
		return ok
	}

	for _, value := range values {
		if strings.EqualFold(value, marker.label) {
			return true
		}

		for _, datum := range marker.values {
			if strings.EqualFold(value, data.String(datum)) {
				return true
			}
		}
	}

	return false
}

// Produce a string representation of a stack marker.
func (sm StackMarker) String() string {
	b := strings.Builder{}
	b.WriteString("Marker<")
	b.WriteString(sm.label)

	for _, data := range sm.values {
		b.WriteString(", ")
		b.WriteString(fmt.Sprintf("%v", data))
	}

	b.WriteString(">")

	return b.String()
}

// findMarker returns the depth at which a marker is found on the stack.
// if there is no marker found, the depth returned is zero.
func findMarker(c *Context, i any) int {
	depth := 0
	found := false
	target := ""

	if m, ok := i.(StackMarker); ok {
		target = m.label
	} else if m, ok := i.(string); ok {
		target = m
	}

	for c.stackPointer > c.framePointer && c.stackPointer-depth >= 0 {
		sp := c.stackPointer - depth

		// Make sure we're not beyond end of stack in starting point for search.
		if sp >= len(c.stack) {
			depth++

			continue
		}

		v := c.stack[c.stackPointer-(depth+0)]

		_, found = v.(StackMarker)
		if found && target != "" {
			found = v.(StackMarker).label == target
		}

		if found {
			return depth
		}

		depth++
	}

	return 0
}

// dropToMarkerByteCode discards items on the stack until it
// finds a marker value, at which point it stops. This is
// used to discard unused return values on the stack. If there
// is no marker, this drains the stack.
func dropToMarkerByteCode(c *Context, i any) error {
	found := false
	target := ""

	if m, ok := i.(StackMarker); ok {
		target = m.label
	}

	for !found {
		// Don't drop across stack frames.
		if c.stackPointer <= c.framePointer {
			break
		}

		v, err := c.Pop()
		if err != nil {
			break
		}

		// Was this an error that was abandoned by the assignment operation?
		if e, ok := v.(error); ok {
			if !errors.Nil(e) && c.throwUncheckedErrors {
				return e
			}
		}

		// See if we've hit a stack marker. If we were asked to
		// drop to a specific one, also test the market name.
		_, found = v.(StackMarker)
		if found && i != nil {
			found = v.(StackMarker).label == target
		}
	}

	return nil
}

// stackCheckByteCode has an integer argument, and verifies
// that there are this many items on the stack, which is
// used to verify that multiple return-values on the stack
// are present.
func stackCheckByteCode(c *Context, i any) error {
	if count, err := data.Int(i); err != nil || c.stackPointer <= count {
		return c.runtimeError(errors.ErrReturnValueCount)
	} else {
		// Is there a stack marker on the stack at all?
		for i := c.stackPointer - (count - 1); i >= 0; i-- {
			v := c.stack[i]
			if isStackMarker(v) {
				return nil
			}
		}

		// The marker is an instance of a StackMarker object.
		v := c.stack[c.stackPointer-(count+1)]
		if isStackMarker(v) {
			return nil
		}
	}

	return c.runtimeError(errors.ErrReturnValueCount)
}

// pushByteCode instruction processor. This pushes the instruction operand
// onto the runtime stack. When the operand is a literal *ByteCode (a function
// literal / closure), it is cloned and the current symbol table is captured
// onto the clone so the closure retains access to variables from its defining
// scope even after that scope is popped.
func pushByteCode(c *Context, i any) error {
	if bc, ok := i.(*ByteCode); ok && bc.IsLiteral() {
		clone := bc.Clone()
		clone.capturedScope = c.symbols
		
		return c.push(clone)
	}

	return c.push(i)
}

// dropByteCode instruction processor drops items from the stack and discards
// them.  By default one item is dropped, but an integer operand can be given
// to drop that many items at once.
//
// STACK-3 fix: the original code returned nil when Pop() reported an underflow,
// silently accepting an over-drop.  This was inconsistent with every other
// stack-consuming instruction in the package, which all propagate underflow
// errors.  The fix returns the error so callers can detect a mismatch between
// the number of items requested and the number actually present.
func dropByteCode(c *Context, i any) error {
	var err error

	count := 1
	if i != nil {
		count, err = data.Int(i)
		if err != nil {
			return c.runtimeError(err)
		}
	}

	for n := 0; n < count; n++ {
		if _, err = c.Pop(); err != nil {
			// Propagate stack underflow rather than silently swallowing it
			// (STACK-3 fix).
			return err
		}
	}

	return nil
}

// dupByteCode instruction processor duplicates the top stack item.
func dupByteCode(c *Context, i any) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	_ = c.push(v)
	_ = c.push(v)

	return nil
}

// readStackByteCode reads an item from the stack without removing any existing
// items, then pushes a copy of it.  The operand is a non-negative integer offset
// from the top-of-stack: 0 means the topmost item (same as Dup), 1 means the
// item one below TOS, and so on.  Negative operands are treated as their
// absolute value.
//
// STACK-2 fix: the original guard was `idx > c.stackPointer`, which failed to
// catch the case where idx == c.stackPointer (e.g. idx=0 on an empty stack or
// idx=1 with one item).  In those cases the computed slice index
// (c.stackPointer-1)-idx went negative, causing a runtime panic.
// Changed to `idx >= c.stackPointer` so the boundary is correctly rejected.
func readStackByteCode(c *Context, i any) error {
	idx, err := data.Int(i)
	if err != nil {
		return c.runtimeError(err)
	}

	if idx < 0 {
		idx = -idx
	}

	// Guard: idx must be strictly less than stackPointer so that
	// (c.stackPointer-1)-idx remains a valid non-negative slice index.
	if idx >= c.stackPointer {
		return c.runtimeError(errors.ErrStackUnderflow)
	}

	return c.push(c.stack[(c.stackPointer-1)-idx])
}

// swapByteCode instruction processor exchanges the top two
// stack items. It is an error if there are not at least
// two items on the stack.
func swapByteCode(c *Context, i any) error {
	v1, err := c.Pop()
	if err != nil {
		return err
	}

	v2, err := c.Pop()
	if err != nil {
		return err
	}

	_ = c.push(v1)
	_ = c.push(v2)

	return nil
}

// copyByteCode instruction processor makes a copy of the topmost object.
// This is different from duplicating: it creates an entirely new value via a
// JSON round-trip, so the copy and the original share no underlying memory.
// After this instruction the stack contains [original, deep_copy] with the
// deep copy on top.
//
// STACK-1 fix: the original code pushed the integer literal 2 instead of v2
// (the unmarshalled copy).  Changed c.push(2) to c.push(v2).
func copyByteCode(c *Context, i any) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Keep the original on the stack first.
	_ = c.push(v)

	// JSON round-trip to produce a deep copy that shares no memory with v.
	// Note: json.Unmarshal always produces float64 for numeric values,
	// map[string]any for objects, and []any for arrays — the copy may have
	// a different Go type than the original.
	var v2 any

	byt, _ := json.Marshal(v)
	err = json.Unmarshal(byt, &v2)

	// Push the deep copy, not the integer literal 2 (STACK-1 fix).
	_ = c.push(v2)

	if err != nil {
		err = errors.New(err)
	}

	return err
}

func getVarArgsByteCode(c *Context, i any) error {
	argPos, err := data.Int(i)
	if err != nil {
		return c.runtimeError(err)
	}

	err = c.runtimeError(errors.ErrInvalidVariableArguments)

	if arrayV, ok := c.get(defs.ArgumentListVariable); ok {
		if args, ok := arrayV.(*data.Array); ok {
			// If no more args in the list to satisfy, push empty array
			if args.Len() < argPos {
				r := data.NewArray(data.InterfaceType, 0)

				return c.push(r)
			}

			value, err := args.GetSlice(argPos, args.Len())
			if err != nil {
				return err
			}

			return c.push(data.NewArrayFromInterfaces(data.InterfaceType, value...))
		}
	}

	return err
}
