package bytecode

import (
	"fmt"
	"reflect"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

// This manages operations on structures (structs, maps, and arrays)

// loadIndexByteCode instruction processor. If the operand is non-nil then
// it is used as the index value, else the index value comes from the
// stack.
func loadIndexByteCode(c *Context, i interface{}) *errors.EgoError {
	var err *errors.EgoError

	var index interface{}

	if i != nil {
		index = i
	} else {
		index, err = c.Pop()
		if !errors.Nil(err) {
			return err
		}
	}

	array, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	switch a := array.(type) {
	case *datatypes.EgoMap:
		var v interface{}

		// @tomcole a bit of a hack here. If this is a map index,
		// and we know the next instruction is a StackCheck 2, then
		// it is the condition that allows for an optional second
		// value indicating if the map value was found or not.
		//
		// If the list-based lvalue processor changes it's use of
		// the StackCheck opcode this will need to be revised!
		pc := c.programCounter
		opcodes := c.bc.Opcodes()
		twoValues := false

		if len(opcodes) > pc {
			next := opcodes[pc]
			if count, ok := next.Operand.(int); ok && count == 2 && (next.Operation == StackCheck) {
				found := false

				if v, found, err = a.Get(index); errors.Nil(err) {
					_ = c.stackPush(StackMarker{Desc: "results"})
					_ = c.stackPush(found)
					err = c.stackPush(v)
					twoValues = true
				}
			}
		}

		if !twoValues {
			if v, _, err = a.Get(index); errors.Nil(err) {
				err = c.stackPush(v)
			}
		}

	// Reading from a channel ignores the index value.
	case *datatypes.Channel:
		var datum interface{}

		datum, err = a.Receive()
		if errors.Nil(err) {
			err = c.stackPush(datum)
		}

	case *datatypes.EgoStruct:
		key := datatypes.GetString(index)
		v, _ := a.Get(key)
		err = c.stackPush(v)
		c.lastStruct = a

	case *datatypes.EgoArray:
		subscript := datatypes.GetInt(index)
		if subscript < 0 || subscript >= a.Len() {
			return c.newError(errors.ErrArrayIndex).Context(subscript)
		}

		v, _ := a.Get(subscript)
		err = c.stackPush(v)

	case []interface{}:
		// Needed for varars processing
		subscript := datatypes.GetInt(index)
		if subscript < 0 || subscript >= len(a) {
			return c.newError(errors.ErrArrayIndex).Context(subscript)
		}

		v := a[subscript]
		err = c.stackPush(v)

	default:
		err = c.newError(errors.ErrInvalidType)
	}

	return err
}

// loadSliceByteCode instruction processor.
func loadSliceByteCode(c *Context, i interface{}) *errors.EgoError {
	index2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	index1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	array, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	switch a := array.(type) {
	case *datatypes.EgoArray:
		subscript1 := datatypes.GetInt(index1)
		subscript2 := datatypes.GetInt(index2)

		v, err := a.GetSlice(subscript1, subscript2)
		if errors.Nil(err) {
			err = c.stackPush(v)
		}

		return err
	// Array of objects means we retrieve a slice.
	case []interface{}:
		subscript1 := datatypes.GetInt(index1)
		if subscript1 < 0 || subscript1 >= len(a) {
			return c.newError(errors.ErrInvalidSliceIndex).Context(subscript1)
		}

		subscript2 := datatypes.GetInt(index2)
		if subscript2 < subscript1 || subscript2 >= len(a) {
			return c.newError(errors.ErrInvalidSliceIndex).Context(subscript2)
		}

		v := a[subscript1 : subscript2+1]
		_ = c.stackPush(v)

	default:
		return c.newError(errors.ErrInvalidType)
	}

	return nil
}

// storeIndexByteCode instruction processor.
func storeIndexByteCode(c *Context, i interface{}) *errors.EgoError {
	index, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	destination, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	switch a := destination.(type) {
	case datatypes.Type:
		fmt.Println("DEBUG: dead code")
		a.DefineFunction(datatypes.GetString(index), v)

	case *datatypes.Type:
		a.DefineFunction(datatypes.GetString(index), v)

	case *datatypes.EgoMap:
		if _, err = a.Set(index, v); errors.Nil(err) {
			err = c.stackPush(a)
		}

		if !errors.Nil(err) {
			return errors.New(err).In(c.GetModuleName()).At(c.GetLine(), 0)
		}

	case *datatypes.EgoStruct:
		key := datatypes.GetString(index)

		err = a.Set(key, v)
		if !errors.Nil(err) {
			return c.newError(err)
		}

		_ = c.stackPush(a)

	// Index into array is integer index
	case *datatypes.EgoArray:
		subscript := datatypes.GetInt(index)
		if subscript < 0 || subscript >= a.Len() {
			return c.newError(errors.ErrArrayIndex).Context(subscript)
		}

		if c.Static {
			vv, _ := a.Get(subscript)
			if vv != nil && (reflect.TypeOf(vv) != reflect.TypeOf(v)) {
				return c.newError(errors.ErrInvalidVarType)
			}
		}

		err = a.Set(subscript, v)
		if errors.Nil(err) {
			err = c.stackPush(a)
		}

		return err

	// Index into array is integer index
	case []interface{}:
		subscript := datatypes.GetInt(index)
		if subscript < 0 || subscript >= len(a) {
			return c.newError(errors.ErrArrayIndex).Context(subscript)
		}

		if c.Static {
			vv := a[subscript]
			if vv != nil && (reflect.TypeOf(vv) != reflect.TypeOf(v)) {
				return c.newError(errors.ErrInvalidVarType)
			}
		}

		a[subscript] = v
		_ = c.stackPush(a)

	default:
		return c.newError(errors.ErrInvalidType)
	}

	return nil
}

// storeIntoByteCode instruction processor.
func storeIntoByteCode(c *Context, i interface{}) *errors.EgoError {
	index, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	destination, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	switch a := destination.(type) {
	case *datatypes.EgoMap:
		if _, err = a.Set(index, v); errors.Nil(err) {
			err = c.stackPush(a)
		}

		if !errors.Nil(err) {
			return c.newError(err)
		}

	default:
		return c.newError(errors.ErrInvalidType)
	}

	return nil
}

func flattenByteCode(c *Context, i interface{}) *errors.EgoError {
	c.argCountDelta = 0

	v, err := c.Pop()
	if errors.Nil(err) {
		if array, ok := v.(*datatypes.EgoArray); ok {
			for idx := 0; idx < array.Len(); idx = idx + 1 {
				vv, _ := array.Get(idx)
				_ = c.stackPush(vv)
				c.argCountDelta++
			}
		} else if array, ok := v.([]interface{}); ok {
			for _, vv := range array {
				_ = c.stackPush(vv)
				c.argCountDelta++
			}
		} else {
			_ = c.stackPush(v)
		}
	}

	// If we found stuff to expand, reduce the count by one (since
	// any argument list knows about the pre-flattened array value
	// in the function call count)
	if c.argCountDelta > 0 {
		c.argCountDelta--
	}

	return err
}
