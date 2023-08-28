package bytecode

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// defs.DiscardedVariable is the reserved name for the variable
// whose value is not used. This is place-holder in some Ego/Go
// syntax constructs where a value is not used. It can also be
// used to discard the result of a function call.

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
	var (
		value interface{}
		err   error
		name  string
	)

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

	if len(name) > 1 && name[0:1] == defs.DiscardedVariable {
		oldValue, found := c.get(name)
		if !found {
			return c.error(errors.ErrReadOnly).Context(name)
		}

		if _, ok := oldValue.(symbols.UndefinedValue); !ok {
			return c.error(errors.ErrReadOnly).Context(name)
		}
	}

	if isStackMarker(value) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Get the name. If it is the reserved name "_" it means
	// to just discard the value.
	if name == defs.DiscardedVariable {
		return nil
	}

	value, err = c.checkType(name, value)
	if err != nil {
		return c.error(err)
	}

	if strings.HasPrefix(name, defs.DiscardedVariable) {
		return c.set(name, data.Constant(value))
	}

	return c.set(name, value)
}

// StoreChan instruction processor.
func storeChanByteCode(c *Context, i interface{}) error {
	// Get the value on the stack, and determine if it is a channel or a datum.
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	sourceChan := false

	if _, ok := v.(*data.Channel); ok {
		sourceChan = true
	}

	// Get the name that is to be used on the other side. If the other item is
	// already known to be a channel, then create this variable (with a nil value)
	// so it can receive the channel info regardless of its type.
	varname := data.String(i)

	x, found := c.get(varname)
	if !found {
		if sourceChan {
			err = c.create(varname)
		} else {
			err = c.error(errors.ErrUnknownIdentifier).Context(x)
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
		return c.error(errors.ErrInvalidChannel)
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
		if varname != defs.DiscardedVariable {
			err = c.set(varname, datum)
		}
	}

	return err
}

// storeGlobalByteCode instruction processor. This function
// is used to store a value in the global symbol table.
func storeGlobalByteCode(c *Context, i interface{}) error {
	value, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(value) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Get the name and set it in the global table.
	name := data.String(i)

	// Is this a readonly variable that is a complex native type?
	// If so, mark it as readonly.
	if len(name) > 1 && name[0:1] == defs.DiscardedVariable {
		constantValue := data.DeepCopy(value)
		switch a := constantValue.(type) {
		case *data.Map:
			a.SetReadonly(true)

		case *data.Array:
			a.SetReadonly(true)

		case *data.Struct:
			a.SetReadonly(true)
		}

		c.symbols.Root().SetAlways(name, constantValue)
	} else {
		c.symbols.Root().SetAlways(name, value)
	}

	return err
}

// StoreViaPointer has a name as it's argument. It loads the value,
// verifies it is a pointer, and stores TOS into that pointer.
func storeViaPointerByteCode(c *Context, i interface{}) error {
	var (
		dest interface{}
		name string
		ok   bool
	)

	if i != nil {
		name = data.String(i)

		if name == "" || name[0:1] == defs.DiscardedVariable {
			return c.error(errors.ErrInvalidIdentifier)
		}

		if d, ok := c.get(name); !ok {
			return c.error(errors.ErrUnknownIdentifier).Context(name)
		} else {
			dest = d
		}
	} else {
		if d, err := c.Pop(); err != nil {
			return err
		} else {
			dest = d
		}
	}

	if data.IsNil(dest) {
		return c.error(errors.ErrNilPointerReference).Context(name)
	}

	// IF the destination is a pointer type and it's a pointer to an
	// immutable object, we don't allow that. If we have a name, add
	// that to the context of the error we create.
	if x, ok := dest.(*interface{}); ok {
		z := *x
		if _, ok := z.(data.Immutable); ok {
			e := c.error(errors.ErrReadOnlyValue)
			if name != "" {
				e = e.Context("*" + name)
			}

			return e
		}
	}

	// Get the value we are going to store from the stack. if it's
	// a stack marker, there was no return value on the stack.
	src, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(src) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Based on the type, do the store.
	switch actual := dest.(type) {
	case *data.Immutable:
		return c.error(errors.ErrReadOnlyValue)

	case *interface{}:
		*actual = src

	case *bool:
		d := src
		if c.typeStrictness > defs.RelaxedTypeEnforcement {
			d = data.Coerce(src, true)
		} else if _, ok := d.(string); !ok {
			return c.error(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(bool)

	case *byte:
		d := src
		if c.typeStrictness > defs.RelaxedTypeEnforcement {
			d = data.Coerce(src, byte(1))
		} else if _, ok := d.(string); !ok {
			return c.error(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(byte)

	case *int32:
		d := src
		if c.typeStrictness > defs.RelaxedTypeEnforcement {
			d = data.Coerce(src, int32(1))
		} else if _, ok := d.(string); !ok {
			return c.error(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(int32)

	case *int:
		d := src
		if c.typeStrictness > defs.RelaxedTypeEnforcement {
			d = data.Coerce(src, int(1))
		} else if _, ok := d.(string); !ok {
			return c.error(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(int)

	case *int64:
		d := src
		if c.typeStrictness > defs.RelaxedTypeEnforcement {
			d = data.Coerce(src, int64(1))
		} else if _, ok := d.(string); !ok {
			return c.error(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(int64)

	case *float64:
		d := src
		if c.typeStrictness > defs.RelaxedTypeEnforcement {
			d = data.Coerce(src, float64(0))
		} else if _, ok := d.(string); !ok {
			return c.error(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(float64)

	case *float32:
		d := src
		if c.typeStrictness > defs.RelaxedTypeEnforcement {
			d = data.Coerce(src, float32(0))
		} else if _, ok := d.(string); !ok {
			return c.error(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(float32)

	case *string:
		d := src
		if c.typeStrictness > defs.RelaxedTypeEnforcement {
			d = data.Coerce(src, "")
		} else if _, ok := d.(string); !ok {
			return c.error(errors.ErrInvalidVarType).Context(name)
		}

		*actual = d.(string)

	case *data.Array:
		*actual, ok = src.(data.Array)
		if !ok {
			return c.error(errors.ErrNotAPointer).Context(name)
		}

	case **data.Channel:
		*actual, ok = src.(*data.Channel)
		if !ok {
			return c.error(errors.ErrNotAPointer).Context(name)
		}

	default:
		return c.error(errors.ErrNotAPointer).Context(name)
	}

	return nil
}

// storeAlwaysByteCode instruction processor. This function
// is used to store a value in a symbol table regardless of
// whether the value is readonly or protected.
func storeAlwaysByteCode(c *Context, i interface{}) error {
	var (
		v          interface{}
		symbolName string
		err        error
	)

	if array, ok := i.([]interface{}); ok && len(array) == 2 {
		symbolName = data.String(array[0])
		v = array[1]
	} else {
		symbolName = data.String(i)

		v, err = c.Pop()
		if err != nil {
			return err
		}

		if isStackMarker(v) {
			return c.error(errors.ErrFunctionReturnedVoid)
		}
	}

	c.setAlways(symbolName, v)

	// Is this a readonly variable that is a structure? If so, mark it
	// with the embedded readonly flag.
	if len(symbolName) > 1 && symbolName[0:1] == defs.DiscardedVariable {
		switch a := v.(type) {
		case *data.Map:
			a.SetReadonly(true)

		case *data.Array:
			a.SetReadonly(true)

		case *data.Struct:
			a.SetReadonly(true)
		}
	}

	return err
}

// loadByteCode instruction processor. This function loads a value
// from the symbol table and pushes it onto the stack.
func loadByteCode(c *Context, i interface{}) error {
	name := data.String(i)
	if len(name) == 0 {
		return c.error(errors.ErrInvalidIdentifier).Context(name)
	}

	v, found := c.get(name)
	if !found {
		return c.error(errors.ErrUnknownIdentifier).Context(name)
	}

	return c.push(data.UnwrapConstant(v))
}

// explodeByteCode implements Explode. This accepts a struct on the top of
// the stack, and creates local variables for each of the members of the
// struct by their name.
func explodeByteCode(c *Context, i interface{}) error {
	var (
		err error
		v   interface{}
	)

	v, err = c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	empty := true

	if m, ok := v.(*data.Map); ok {
		if !m.KeyType().IsString() {
			err = c.error(errors.ErrWrongMapKeyType)
		} else {
			keys := m.Keys()

			for _, k := range keys {
				empty = false
				v, _, _ := m.Get(k)

				c.setAlways(data.String(k), v)
			}

			if err == nil {
				return c.push(empty)
			}
		}
	} else {
		err = c.error(errors.ErrInvalidType).Context(data.TypeOf(v).String())
	}

	return err
}
