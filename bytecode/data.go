package bytecode

import (
	"github.com/tucats/ego/datatypes"
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
// The value is then written to the symbol table.
//
// If the variable name begins with "_" then it is
// considered a read-only variable, so if the stack
// contains a map then that map is marked with the
// metadata indicator that it is readonly.
func storeByteCode(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	if IsStackMarker(v) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	// Get the name. If it is the reserved name "_" it means
	// to just discard the value.
	varname := datatypes.GetString(i)
	if varname == DiscardedVariableName {
		return nil
	}

	err = c.checkType(varname, v)
	if errors.Nil(err) {
		err = c.symbolSet(varname, v)
	}

	if !errors.Nil(err) {
		return c.newError(err)
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(varname) > 1 && varname[0:1] == DiscardedVariableName {
		switch a := v.(type) {
		case datatypes.EgoMap:
			a.ImmutableKeys(true)

		case datatypes.EgoStruct:
			a.SetReadonly(true)
		}
	}

	return err
}

// StoreChan instruction processor.
func storeChanByteCode(c *Context, i interface{}) *errors.EgoError {
	// Get the value on the stack, and determine if it is a channel or a datum.
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	if IsStackMarker(v) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	sourceChan := false

	if _, ok := v.(*datatypes.Channel); ok {
		sourceChan = true
	}

	// Get the name that is to be used on the other side. If the other item is
	// already known to be a channel, then create this variable (with a nil value)
	// so it can receive the channel info regardless of its type.
	varname := datatypes.GetString(i)

	x, ok := c.symbolGet(varname)
	if !ok {
		if sourceChan {
			err = c.symbolCreate(varname)
		} else {
			err = c.newError(errors.ErrUnknownIdentifier).Context(x)
		}

		if !errors.Nil(err) {
			return err
		}
	}

	destChan := false
	if _, ok := x.(*datatypes.Channel); ok {
		destChan = true
	}

	if !sourceChan && !destChan {
		return c.newError(errors.ErrInvalidChannel)
	}

	var datum interface{}

	if sourceChan {
		datum, err = v.(*datatypes.Channel).Receive()
	} else {
		datum = v
	}

	if destChan {
		err = x.(*datatypes.Channel).Send(datum)
	} else {
		if varname != DiscardedVariableName {
			err = c.symbolSet(varname, datum)
		}
	}

	return err
}

// storeGlobalByteCode instruction processor.
func storeGlobalByteCode(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	if IsStackMarker(v) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	// Get the name.
	varname := datatypes.GetString(i)

	err = c.symbols.Root().SetAlways(varname, v)
	if !errors.Nil(err) {
		return c.newError(err)
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(varname) > 1 && varname[0:1] == DiscardedVariableName {
		switch a := v.(type) {
		case datatypes.EgoMap:
			a.ImmutableKeys(true)

		case datatypes.EgoStruct:
			a.SetReadonly(true)
		}
	}

	return err
}

// StoreViaPointer has a name as it's argument. It loads the value,
// verifies it is a pointer, and stores TOS into that pointer.
func storeViaPointerByteCode(c *Context, i interface{}) *errors.EgoError {
	name := datatypes.GetString(i)

	if i == nil || name == "" || name[0:1] == DiscardedVariableName {
		return c.newError(errors.ErrInvalidIdentifier)
	}

	dest, ok := c.symbolGet(name)
	if !ok {
		return c.newError(errors.ErrUnknownIdentifier).Context(name)
	}

	if datatypes.IsNil(dest) {
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
			d = datatypes.Coerce(src, true)
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(bool)

	case *byte:
		d := src
		if !c.Static {
			d = datatypes.Coerce(src, byte(1))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(byte)

	case *int32:
		d := src
		if !c.Static {
			d = datatypes.Coerce(src, int32(1))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(int32)

	case *int:
		d := src
		if !c.Static {
			d = datatypes.Coerce(src, int(1))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(int)

	case *int64:
		d := src
		if !c.Static {
			d = datatypes.Coerce(src, int64(1))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(int64)

	case *float64:
		d := src
		if !c.Static {
			d = datatypes.Coerce(src, float64(0))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(float64)

	case *float32:
		d := src
		if !c.Static {
			d = datatypes.Coerce(src, float32(0))
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(float32)

	case *string:
		d := src
		if !c.Static {
			d = datatypes.Coerce(src, "")
		} else if _, ok := d.(string); !ok {
			return c.newError(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(string)

	case *datatypes.EgoArray:
		*actual, ok = src.(datatypes.EgoArray)
		if !ok {
			return c.newError(errors.ErrNotAPointer).Context(name)
		}
	case *datatypes.EgoMap:
		*actual, ok = src.(datatypes.EgoMap)
		if !ok {
			return c.newError(errors.ErrNotAPointer).Context(name)
		}

	case **datatypes.Channel:
		*actual, ok = src.(*datatypes.Channel)
		if !ok {
			return c.newError(errors.ErrNotAPointer).Context(name)
		}

	default:
		return c.newError(errors.ErrNotAPointer).Context(name)
	}

	return nil
}

// storeAlwaysByteCode instruction processor.
func storeAlwaysByteCode(c *Context, i interface{}) *errors.EgoError {
	var v interface{}

	var symbolName string

	var err *errors.EgoError

	if array, ok := i.([]interface{}); ok && len(array) == 2 {
		symbolName = datatypes.GetString(array[0])
		v = array[1]
	} else {
		symbolName = datatypes.GetString(i)

		v, err = c.Pop()
		if !errors.Nil(err) {
			return err
		}

		if IsStackMarker(v) {
			return c.newError(errors.ErrFunctionReturnedVoid)
		}
	}

	err = c.symbolSetAlways(symbolName, v)
	if !errors.Nil(err) {
		return c.newError(err)
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(symbolName) > 1 && symbolName[0:1] == DiscardedVariableName {
		switch a := v.(type) {
		case *datatypes.EgoMap:
			a.ImmutableKeys(true)

		case *datatypes.EgoStruct:
			a.SetReadonly(true)
		}
	}

	return err
}

// loadByteCode instruction processor.
func loadByteCode(c *Context, i interface{}) *errors.EgoError {
	name := datatypes.GetString(i)
	if len(name) == 0 {
		return c.newError(errors.ErrInvalidIdentifier).Context(name)
	}

	v, found := c.symbolGet(name)
	if !found {
		return c.newError(errors.ErrUnknownIdentifier).Context(name)
	}

	_ = c.stackPush(v)

	return nil
}

// explodeByteCode implements Explode. This accepts a struct on the top of
// the stack, and creates local variables for each of the members of the
// struct by their name.
func explodeByteCode(c *Context, i interface{}) *errors.EgoError {
	var err *errors.EgoError

	var v interface{}

	v, err = c.Pop()
	if !errors.Nil(err) {
		return err
	}

	if IsStackMarker(v) {
		return c.newError(errors.ErrFunctionReturnedVoid)
	}

	empty := true

	if m, ok := v.(*datatypes.EgoMap); ok {
		if m.KeyType().Kind() != datatypes.StringKind {
			err = c.newError(errors.ErrWrongMapKeyType)
		} else {
			keys := m.Keys()

			for _, k := range keys {
				empty = false
				v, _, _ := m.Get(k)

				err = c.symbolSetAlways(datatypes.GetString(k), v)
				if !errors.Nil(err) {
					break
				}
			}
			if errors.Nil(err) {
				return c.stackPush(empty)
			}
		}
	} else {
		err = c.newError(errors.ErrInvalidType).Context(datatypes.TypeOf(v).String())
	}

	return err
}
