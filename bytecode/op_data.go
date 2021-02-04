package bytecode

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*         D A T A   A C C E S S           *
*                                         *
\******************************************/

// StoreImpl instruction processor.
func StoreImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Get the name. If it is the reserved name "_" it means
	// to just discard the value.
	varname := util.GetString(i)
	if varname == "_" {
		return nil
	}

	err = c.checkType(varname, v)
	if err == nil {
		err = c.Set(varname, v)
	}

	if err != nil {
		return c.NewError(err.Error())
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
func StoreChanImpl(c *Context, i interface{}) error {
	// Get the value on the stack, and determine if it is a channel or a datum.
	v, err := c.Pop()
	if err != nil {
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
			err = c.NewError(UnknownIdentifierError, x)
		}

		if err != nil {
			return err
		}
	}

	destChan := false
	if _, ok := x.(*datatypes.Channel); ok {
		destChan = true
	}

	if !sourceChan && !destChan {
		return c.NewError(InvalidChannel)
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
func StoreGlobalImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Get the name.
	varname := util.GetString(i)

	err = c.SetGlobal(varname, v)
	if err != nil {
		return c.NewError(err.Error())
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
func StoreAlwaysImpl(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	// Get the name.
	varname := util.GetString(i)

	err = c.SetAlways(varname, v)
	if err != nil {
		return c.NewError(err.Error())
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
func LoadImpl(c *Context, i interface{}) error {
	name := util.GetString(i)
	if len(name) == 0 {
		return c.NewError(InvalidIdentifierError, name)
	}

	v, found := c.Get(util.GetString(i))
	if !found {
		return c.NewError(UnknownIdentifierError, name)
	}

	_ = c.Push(v)

	return nil
}
