package bytecode

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// storeByteCode implements the Store opcode
//
// Inputs:
//    operand    - The name of the variable in which
//				   the top of stack is stored.
//    stack+0    - The item to be "stored" is read
//                 on the stack.
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

	// Get the name. If it is the reserved name "_" it means
	// to just discard the value.
	varname := util.GetString(i)
	if varname == "_" {
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
	if len(varname) > 1 && varname[0:1] == "_" {
		switch a := v.(type) {
		case *datatypes.EgoMap:
			a.ImmutableKeys(true)

		case *datatypes.EgoStruct:
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

	sourceChan := false

	if _, ok := v.(*datatypes.Channel); ok {
		sourceChan = true
	}

	// Get the name that is to be used on the other side. If the other item is
	// already known to be a channel, then create this variable (with a nil value)
	// so it can receive the channel info regardless of its type.
	varname := util.GetString(i)

	x, ok := c.symbolGet(varname)
	if !ok {
		if sourceChan {
			err = c.symbolCreate(varname)
		} else {
			err = c.newError(errors.UnknownIdentifierError).Context(x)
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
		return c.newError(errors.InvalidChannelError)
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
		if varname != "_" {
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

	// Get the name.
	varname := util.GetString(i)

	err = c.symbols.Root().SetAlways(varname, v)
	if !errors.Nil(err) {
		return c.newError(err)
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(varname) > 1 && varname[0:1] == "_" {
		switch a := v.(type) {
		case *datatypes.EgoMap:
			a.ImmutableKeys(true)

		case *datatypes.EgoStruct:
			a.SetReadonly(true)
		}
	}

	return err
}

// StoreViaPointer has a name as it's argument. It loads the value,
// verifies it is a pointer, and stores TOS into that pointer.
func storeViaPointerByteCode(c *Context, i interface{}) *errors.EgoError {
	name := util.GetString(i)

	dest, ok := c.symbolGet(name)
	if !ok {
		return c.newError(errors.UnknownIdentifierError).Context(name)
	}

	if datatypes.IsNil(dest) {
		return c.newError(errors.NilPointerReferenceError).Context(name)
	}

	src, err := c.Pop()
	if err != nil {
		return err
	}

	switch actual := dest.(type) {
	case *interface{}:
		*actual = src

	case *bool:
		d := util.Coerce(src, true)
		*actual = d.(bool)

	case *int:
		d := util.Coerce(src, 1)
		*actual = d.(int)

	case *float64:
		d := util.Coerce(src, 1.0)
		*actual = d.(float64)

	case *string:
		d := util.Coerce(src, "")
		*actual = d.(string)

	case **datatypes.EgoArray:
		*actual, ok = src.(*datatypes.EgoArray)
		if !ok {
			return c.newError(errors.NotAPointer).Context(name)
		}
	case **datatypes.EgoMap:
		*actual, ok = src.(*datatypes.EgoMap)
		if !ok {
			return c.newError(errors.NotAPointer).Context(name)
		}

	case **datatypes.Channel:
		*actual, ok = src.(*datatypes.Channel)
		if !ok {
			return c.newError(errors.NotAPointer).Context(name)
		}

	default:
		return c.newError(errors.NotAPointer).Context(name)
	}

	return nil
}

// storeAlwaysByteCode instruction processor.
func storeAlwaysByteCode(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Get the name.
	varname := util.GetString(i)

	err = c.symbolSetAlways(varname, v)
	if !errors.Nil(err) {
		return c.newError(err)
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(varname) > 1 && varname[0:1] == "_" {
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
	name := util.GetString(i)
	if len(name) == 0 {
		return c.newError(errors.InvalidIdentifierError).Context(name)
	}

	v, found := c.symbolGet(name)
	if !found {
		return c.newError(errors.UnknownIdentifierError).Context(name)
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

	empty := true

	if m, ok := v.(*datatypes.EgoMap); ok {
		if !m.KeyType().IsType(datatypes.StringType) {
			err = c.newError(errors.InvalidStructError)
		} else {
			keys := m.Keys()

			for _, k := range keys {
				empty = false
				v, _, _ := m.Get(k)

				err = c.symbolSetAlways(util.GetString(k), v)
				if !errors.Nil(err) {
					break
				}
			}
			if errors.Nil(err) {
				return c.stackPush(empty)
			}
		}
	} else {
		err = c.newError(errors.InvalidStructError)
	}

	return err
}
