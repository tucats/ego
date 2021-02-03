package bytecode

import (
	"encoding/json"
	"fmt"

	"github.com/tucats/ego/util"
)

type StackMarker struct {
	Desc string
}

func NewStackMarker(label string, count int) StackMarker {
	return StackMarker{
		Desc: fmt.Sprintf("%s %d items", label, count),
	}
}

/******************************************\
*                                         *
*    S T A C K   M A N A G E M E N T      *
*                                         *
\******************************************/

// DropToMarkerImpl discards items on the stack until it
// finds a marker value, at which point it stops. This is
// used to discard unused return values on the stack. IF there
// is no marker, this drains the stack.
func DropToMarkerImpl(c *Context, i interface{}) error {
	found := false
	for !found {
		v, err := c.Pop()
		if err != nil {
			break
		}
		_, found = v.(StackMarker)
	}

	return nil
}

// StackCheckImpl has an integer argument, and verifies
// that there are this many items on the stack, which is
// used to verify that multiple return-values on the stack
// are present.
func StackCheckImpl(c *Context, i interface{}) error {
	count := util.GetInt(i)
	if c.sp <= count {
		return c.NewError(IncorrectReturnValueCount)
	}

	// The marker is an instance of a StackMarker object.
	v := c.stack[c.sp-(count+1)]
	if _, ok := v.(StackMarker); ok {
		return nil
	}

	return c.NewError(IncorrectReturnValueCount)
}

// PushImpl instruction processor. This pushes the instruction operand
// onto the runtime stack.
func PushImpl(c *Context, i interface{}) error {
	return c.Push(i)
}

// DropImpl instruction processor drops items from the stack and
// discards them. By default, one item is dropped, but an integer
// operand can be specified indicating how many items to drop.
func DropImpl(c *Context, i interface{}) error {
	count := 1
	if i != nil {
		count = util.GetInt(i)
	}

	for n := 0; n < count; n = n + 1 {
		_, err := c.Pop()
		if err != nil {
			return nil
		}
	}

	return nil
}

// DupImpl instruction processor duplicates the top stack item.
func DupImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}
	_ = c.Push(v)
	_ = c.Push(v)

	return nil
}

// SwapImpl instruction processor exchanges the top two
// stack items. It is an error if there are not at least
// two items on the stack.
func SwapImpl(c *Context, i interface{}) error {
	v1, err := c.Pop()
	if err != nil {
		return err
	}
	v2, err := c.Pop()
	if err != nil {
		return err
	}
	_ = c.Push(v1)
	_ = c.Push(v2)

	return nil
}

// CopyImpl instruction processor makes a copy of the topmost
// object. This is different than duplicating, as it creates a
// entire deep copy of the object.
func CopyImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}
	_ = c.Push(v)

	// Use JSON as a reflection-based cloner
	var v2 interface{}
	byt, _ := json.Marshal(v)
	err = json.Unmarshal(byt, &v2)
	_ = c.Push(2)

	return err
}

func GetVarArgsImpl(c *Context, i interface{}) error {
	err := c.NewError(VarArgError)
	argPos := util.GetInt(i)
	if arrayV, ok := c.Get("__args"); ok {
		if args, ok := arrayV.([]interface{}); ok {
			// If no more args in the list to satisfy, push empty array
			if len(args) < argPos {
				r := make([]interface{}, 0)

				return c.Push(r)
			} else {
				return c.Push(args[argPos:])
			}
		}
	}

	return err
}
