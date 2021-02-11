package bytecode

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// StoreImpl instruction processor.

// StoreImpl implements the Store opcode
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
func StoreImpl(c *Context, i interface{}) *errors.EgoError {
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
		err = c.Set(varname, v)
	}

	if !errors.Nil(err) {
		return c.NewError(err)
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(varname) > 1 && varname[0:1] == "_" {
		switch a := v.(type) {
		case *datatypes.EgoMap:
			a.ImmutableKeys(true)

		case map[string]interface{}:
			datatypes.SetMetadata(a, datatypes.ReadonlyMDKey, true)
		}
	}

	return err
}

// StoreChan instruction processor.
func StoreChanImpl(c *Context, i interface{}) *errors.EgoError {
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

	x, ok := c.Get(varname)
	if !ok {
		if sourceChan {
			err = c.Create(varname)
		} else {
			err = c.NewError(errors.UnknownIdentifierError).Context(x)
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
		return c.NewError(errors.InvalidChannelError)
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
			err = c.Set(varname, datum)
		}
	}

	return err
}

// StoreGlobalImpl instruction processor.
func StoreGlobalImpl(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Get the name.
	varname := util.GetString(i)

	err = c.SetGlobal(varname, v)
	if !errors.Nil(err) {
		return c.NewError(err)
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(varname) > 1 && varname[0:1] == "_" {
		switch a := v.(type) {
		case *datatypes.EgoMap:
			a.ImmutableKeys(true)

		case map[string]interface{}:
			datatypes.SetMetadata(a, datatypes.ReadonlyMDKey, true)
		}
	}

	return err
}

// StoreAlwaysImpl instruction processor.
func StoreAlwaysImpl(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Get the name.
	varname := util.GetString(i)

	err = c.SetAlways(varname, v)
	if !errors.Nil(err) {
		return c.NewError(err)
	}

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(varname) > 1 && varname[0:1] == "_" {
		switch a := v.(type) {
		case *datatypes.EgoMap:
			a.ImmutableKeys(true)

		case map[string]interface{}:
			datatypes.SetMetadata(a, datatypes.ReadonlyMDKey, true)
		}
	}

	return err
}

// LoadImpl instruction processor.
func LoadImpl(c *Context, i interface{}) *errors.EgoError {
	name := util.GetString(i)
	if len(name) == 0 {
		return c.NewError(errors.InvalidIdentifierError).Context(name)
	}

	v, found := c.Get(util.GetString(i))
	if !found {
		return c.NewError(errors.UnknownIdentifierError).Context(name)
	}

	_ = c.Push(v)

	return nil
}
