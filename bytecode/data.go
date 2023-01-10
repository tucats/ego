package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

// DiscardedVariableName is the reserved name for the variable
// whose value is not used. This is place-holder in some Ego/Go
// syntax constructs where a value is not used. It can also be
// used to discard the result of a function call.
const DiscardedVariableName = "_"

// storeByteCode implements the Store opcode
//
// Inputs:
//
//	   operand    - The name of the variable in which
//					   the top of stack is stored.
//	   stack+0    - The item to be "stored" is read
//	                on the stack.
//
// The value to be stored is popped from the stack. The
// variable name and value are used to do a type check
// to ensure that the value is compatible if we are in
// static type mode.
//
// Note that if the operand is actually an interface
// array, then the first item is the name and the second
// is the value, and the stack is not popped.
//
// The value is then written to the symbol table.
//
// If the variable name begins with "_" then it is
// considered a read-only variable, so if the stack
// contains a map then that map is marked with the
// metadata indicator that it is readonly.
func storeByteCode(c *Context, i interface{}) error {
	var value interface{}

	var err error

	var name string

	if operands, ok := i.([]interface{}); ok && len(operands) == 2 {
		name = data.String(operands[0])
		value = operands[1]
	} else {
		name = data.String(i)

		value, err = c.Pop()
		if err != nil {
			return err
		}
	}

	if IsStackMarker(value) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	// Get the name. If it is the reserved name "_" it means
	// to just discard the value.
	if name == DiscardedVariableName {
		return nil
	}

	err = c.checkType(name, value)
	if err == nil {
		err = c.symbolSet(name, value)
	} else {
		return c.newError(err)
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(name) > 1 && name[0:1] == DiscardedVariableName {
		switch a := value.(type) {
		case *data.Map:
			a.ImmutableKeys(true)

		case *data.Struct:
			a.SetReadonly(true)
		}
	}

	return err
}

// StoreChan instruction processor.
func storeChanByteCode(c *Context, i interface{}) error {
	// Get the value on the stack, and determine if it is a channel or a datum.
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(v) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	sourceChan := false

	if _, ok := v.(*data.Channel); ok {
		sourceChan = true
	}

	// Get the name that is to be used on the other side. If the other item is
	// already known to be a channel, then create this variable (with a nil value)
	// so it can receive the channel info regardless of its type.
	varname := data.String(i)

	x, found := c.symbolGet(varname)
	if !found {
		if sourceChan {
			err = c.symbolCreate(varname)
		} else {
			err = c.newError(errors.ErrUnknownIdentifier).Context(x)
		}

		if err != nil {
			return err
		}
	}

	destChan := false
	if _, ok := x.(*data.Channel); ok {
		destChan = true
	}

	if !sourceChan && !destChan {
		return c.newError(errors.ErrInvalidChannel)
	}

	var datum interface{}

	if sourceChan {
		datum, err = v.(*data.Channel).Receive()
	} else {
		datum = v
	}

	if destChan {
		err = x.(*data.Channel).Send(datum)
	} else {
		if varname != DiscardedVariableName {
			err = c.symbolSet(varname, datum)
		}
	}

	return err
}

// storeGlobalByteCode instruction processor.
func storeGlobalByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(v) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	// Get the name.
	varname := data.String(i)

	c.symbols.Root().SetAlways(varname, v)

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(varname) > 1 && varname[0:1] == DiscardedVariableName {
		switch a := v.(type) {
		case *data.Map:
			a.ImmutableKeys(true)

		case *data.Struct:
			a.SetReadonly(true)
		}
	}

	return err
}

// StoreViaPointer has a name as it's argument. It loads the value,
// verifies it is a pointer, and stores TOS into that pointer.
func storeViaPointerByteCode(c *Context, i interface{}) error {
	name := data.String(i)

	if i == nil || name == "" || name[0:1] == DiscardedVariableName {
		return c.newError(errors.ErrInvalidIdentifier)
	}

	dest, ok := c.symbolGet(name)
	if !ok {
		return c.newError(errors.ErrUnknownIdentifier).Context(name)
	}

	if data.IsNil(dest) {
		return c.newError(errors.ErrNilPointerReference).Context(name)
	}

	src, err := c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(src) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	switch actual := dest.(type) {
	case *interface{}:
		*actual = src

	case *bool:
		d := src
		if !c.Static {
			d = data.Coerce(src, true)
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(bool)

	case *byte:
		d := src
		if !c.Static {
			d = data.Coerce(src, byte(1))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(byte)

	case *int32:
		d := src
		if !c.Static {
			d = data.Coerce(src, int32(1))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(int32)

	case *int:
		d := src
		if !c.Static {
			d = data.Coerce(src, int(1))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(int)

	case *int64:
		d := src
		if !c.Static {
			d = data.Coerce(src, int64(1))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(int64)

	case *float64:
		d := src
		if !c.Static {
			d = data.Coerce(src, float64(0))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(float64)

	case *float32:
		d := src
		if !c.Static {
			d = data.Coerce(src, float32(0))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(float32)

	case *string:
		d := src
		if !c.Static {
			d = data.Coerce(src, "")
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(string)

	case *data.Array:
		*actual, ok = src.(data.Array)
		if !ok {
			return c.newError(errors.ErrNotAPointer).Context(name)
		}

	case **data.Channel:
		*actual, ok = src.(*data.Channel)
		if !ok {
			return c.newError(errors.ErrNotAPointer).Context(name)
		}

	default:
		return c.newError(errors.ErrNotAPointer).Context(name)
	}

	return nil
}

// storeAlwaysByteCode instruction processor.
func storeAlwaysByteCode(c *Context, i interface{}) error {
	var v interface{}

	var symbolName string

	var err error

	if array, ok := i.([]interface{}); ok && len(array) == 2 {
		symbolName = data.String(array[0])
		v = array[1]
	} else {
		symbolName = data.String(i)

		v, err = c.Pop()
		if err != nil {
			return err
		}

		if IsStackMarker(v) {
			return c.newError(errors.ErrFunctionReturnedVoid)
		}
	}

	c.symbolSetAlways(symbolName, v)

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(symbolName) > 1 && symbolName[0:1] == DiscardedVariableName {
		switch a := v.(type) {
		case *data.Map:
			a.ImmutableKeys(true)

		case *data.Struct:
			a.SetReadonly(true)
		}
	}

	return err
}

// loadByteCode instruction processor.
func loadByteCode(c *Context, i interface{}) error {
	name := data.String(i)
	if len(name) == 0 {
		return c.newError(errors.ErrInvalidIdentifier).Context(name)
	}

	v, found := c.symbolGet(name)
	if !found {
		return c.newError(errors.ErrUnknownIdentifier).Context(name)
	}

	return c.stackPush(v)
}

// explodeByteCode implements Explode. This accepts a struct on the top of
// the stack, and creates local variables for each of the members of the
// struct by their name.
func explodeByteCode(c *Context, i interface{}) error {
	var err error

	var v interface{}

	v, err = c.Pop()
	if err != nil {
		return err
	}

	if IsStackMarker(v) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	empty := true

	if m, ok := v.(*data.Map); ok {
		if m.KeyType().Kind() != data.StringKind {
			err = c.newError(errors.ErrWrongMapKeyType)
		} else {
			keys := m.Keys()

			for _, k := range keys {
				empty = false
				v, _, _ := m.Get(k)

				c.symbolSetAlways(data.String(k), v)
			}

			if err == nil {
				return c.stackPush(empty)
			}
		}
	} else {
		err = c.newError(errors.ErrInvalidType).Context(data.TypeOf(v).String())
	}

	return err
}
