package bytecode

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tucats/ego/datatypes"
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
	Desc string
	Data []interface{}
}

// NewStackMarker generates a enw stack marker object, using the
// supplied label and optional list of datu.
func NewStackMarker(label string, data ...interface{}) StackMarker {
	return StackMarker{
		Desc: label,
		Data: data,
	}
}

// See if the item is a marker. If there are no marker types, this
// just returns true if the interface given is any stack marker. If
// one or more type strings are passed, then in addition to being a
// marker, the item must contain at least one of the types as one of
// it data elements.
func IsStackMarker(i interface{}, types ...string) bool {
	marker, ok := i.(StackMarker)
	if !ok || len(types) == 0 {
		return ok
	}

	for _, t := range types {
		for _, data := range marker.Data {
			if strings.EqualFold(t, datatypes.GetString(data)) {
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
	b.WriteString(sm.Desc)

	for _, data := range sm.Data {
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
func dropToMarkerByteCode(c *Context, i interface{}) *errors.EgoError {
	found := false
	target := ""

	if m, ok := i.(StackMarker); ok {
		target = m.Desc
	}

	for !found {
		// Don't drop across stack frames.
		if c.stackPointer <= c.framePointer {
			break
		}

		v, err := c.Pop()
		if !errors.Nil(err) {
			break
		}

		// Was this an error that was abandoned by the assignment operation?
		if e, ok := v.(*errors.EgoError); ok {
			if !errors.Nil(e) && c.throwUncheckedErrors {
				return e
			}
		}

		// See if we've hit a stack marker. If we were asked to
		// drop to a specific one, also test the market name.
		_, found = v.(StackMarker)
		if found && i != nil {
			found = v.(StackMarker).Desc == target
		}
	}

	return nil
}

// stackCheckByteCode has an integer argument, and verifies
// that there are this many items on the stack, which is
// used to verify that multiple return-values on the stack
// are present.
func stackCheckByteCode(c *Context, i interface{}) *errors.EgoError {
	if count := datatypes.GetInt(i); c.stackPointer <= count {
		return c.newError(errors.ErrReturnValueCount)
	} else {
		// The marker is an instance of a StackMarker object.
		v := c.stack[c.stackPointer-(count+1)]
		if IsStackMarker(v) {
			return nil
		}
	}

	return c.newError(errors.ErrReturnValueCount)
}

// pushByteCode instruction processor. This pushes the instruction operand
// onto the runtime stack.
func pushByteCode(c *Context, i interface{}) *errors.EgoError {
	return c.stackPush(i)
}

// dropByteCode instruction processor drops items from the stack and
// discards them. By default, one item is dropped, but an integer
// operand can be specified indicating how many items to drop.
func dropByteCode(c *Context, i interface{}) *errors.EgoError {
	count := 1
	if i != nil {
		count = datatypes.GetInt(i)
	}

	for n := 0; n < count; n = n + 1 {
		_, err := c.Pop()
		if !errors.Nil(err) {
			return nil
		}
	}

	return nil
}

// dupByteCode instruction processor duplicates the top stack item.
func dupByteCode(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	_ = c.stackPush(v)
	_ = c.stackPush(v)

	return nil
}

// dupByteCode instruction processor reads an item from the stack,
// without otherwise changing the stack, and then pushes it back
// on the stack. The argument must be an integer which describes the
// offset from the top-of-stack. That is, zero means just duplicate
// the ToS, while 1 means read the second item and make a dup on the
// stack of that value, etc.
func readStackByteCode(c *Context, i interface{}) *errors.EgoError {
	idx := datatypes.GetInt(i)
	if idx < 0 {
		idx = -idx
	}

	if idx > c.stackPointer {
		return c.newError(errors.ErrStackUnderflow)
	}

	return c.stackPush(c.stack[(c.stackPointer-1)-idx])
}

// swapByteCode instruction processor exchanges the top two
// stack items. It is an error if there are not at least
// two items on the stack.
func swapByteCode(c *Context, i interface{}) *errors.EgoError {
	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	_ = c.stackPush(v1)
	_ = c.stackPush(v2)

	return nil
}

// copyByteCode instruction processor makes a copy of the topmost
// object. This is different than duplicating, as it creates a
// entire deep copy of the object.
func copyByteCode(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	_ = c.stackPush(v)

	// Use JSON as a reflection-based clone operation
	var v2 interface{}

	byt, _ := json.Marshal(v)
	err = errors.New(json.Unmarshal(byt, &v2))
	_ = c.stackPush(2)

	return err
}

func getVarArgsByteCode(c *Context, i interface{}) *errors.EgoError {
	err := c.newError(errors.ErrInvalidVariableArguments)
	argPos := datatypes.GetInt(i)

	if arrayV, ok := c.symbolGet("__args"); ok {
		if args, ok := arrayV.(*datatypes.EgoArray); ok {
			// If no more args in the list to satisfy, push empty array
			if args.Len() < argPos {
				r := datatypes.NewArray(&datatypes.InterfaceType, 0)

				return c.stackPush(r)
			}

			value, err := args.GetSlice(argPos, args.Len())
			if !errors.Nil(err) {
				return err
			}

			return c.stackPush(datatypes.NewArrayFromArray(&datatypes.InterfaceType, value))
		}
	}

	return err
}
