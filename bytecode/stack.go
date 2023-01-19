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
	values []interface{}
}

// NewStackMarker generates a enw stack marker object, using the
// supplied label and optional list of datu.
func NewStackMarker(label string, values ...interface{}) StackMarker {
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
func isStackMarker(i interface{}, values ...string) bool {
	// First, check special case of a call frame, which acts
	// as a marker but has lots of other data in it as well.
	frame, ok := i.(CallFrame)
	if ok {
		ui.Log(ui.TraceLogger, "                 >>> Unexpected call frame found: %s:%d", frame.Module, frame.Line)

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

// Produce a string reprsentation of a stack marker.
func (sm StackMarker) String() string {
	b := strings.Builder{}
	b.WriteString("M<")
	b.WriteString(sm.label)

	for _, data := range sm.values {
		b.WriteString(", ")
		b.WriteString(fmt.Sprintf("%v", data))
	}

	b.WriteString(">")

	return b.String()
}

// dropToMarkerByteCode discards items on the stack until it
// finds a marker value, at which point it stops. This is
// used to discard unused return values on the stack. IF there
// is no marker, this drains the stack.
func dropToMarkerByteCode(c *Context, i interface{}) error {
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
func stackCheckByteCode(c *Context, i interface{}) error {
	if count := data.Int(i); c.stackPointer <= count {
		return c.error(errors.ErrReturnValueCount)
	} else {
		// The marker is an instance of a StackMarker object.
		v := c.stack[c.stackPointer-(count+1)]
		if isStackMarker(v) {
			return nil
		}
	}

	return c.error(errors.ErrReturnValueCount)
}

// pushByteCode instruction processor. This pushes the instruction operand
// onto the runtime stack.
func pushByteCode(c *Context, i interface{}) error {
	return c.push(i)
}

// dropByteCode instruction processor drops items from the stack and
// discards them. By default, one item is dropped, but an integer
// operand can be specified indicating how many items to drop.
func dropByteCode(c *Context, i interface{}) error {
	count := 1
	if i != nil {
		count = data.Int(i)
	}

	for n := 0; n < count; n = n + 1 {
		_, err := c.Pop()
		if err != nil {
			return nil
		}
	}

	return nil
}

// dupByteCode instruction processor duplicates the top stack item.
func dupByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	_ = c.push(v)
	_ = c.push(v)

	return nil
}

// dupByteCode instruction processor reads an item from the stack,
// without otherwise changing the stack, and then pushes it back
// on the stack. The argument must be an integer which describes the
// offset from the top-of-stack. That is, zero means just duplicate
// the ToS, while 1 means read the second item and make a dup on the
// stack of that value, etc.
func readStackByteCode(c *Context, i interface{}) error {
	idx := data.Int(i)
	if idx < 0 {
		idx = -idx
	}

	if idx > c.stackPointer {
		return c.error(errors.ErrStackUnderflow)
	}

	return c.push(c.stack[(c.stackPointer-1)-idx])
}

// swapByteCode instruction processor exchanges the top two
// stack items. It is an error if there are not at least
// two items on the stack.
func swapByteCode(c *Context, i interface{}) error {
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

// copyByteCode instruction processor makes a copy of the topmost
// object. This is different than duplicating, as it creates a
// entire deep copy of the object.
func copyByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	_ = c.push(v)

	// Use JSON as a reflection-based clone operation
	var v2 interface{}

	byt, _ := json.Marshal(v)
	err = json.Unmarshal(byt, &v2)
	_ = c.push(2)

	if err != nil {
		err = errors.NewError(err)
	}

	return err
}

func getVarArgsByteCode(c *Context, i interface{}) error {
	err := c.error(errors.ErrInvalidVariableArguments)
	argPos := data.Int(i)

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

			return c.push(data.NewArrayFromArray(data.InterfaceType, value))
		}
	}

	return err
}
