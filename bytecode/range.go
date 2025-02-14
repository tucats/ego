package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// rangeDefinition describes what we know about the (current) for..range loop. This
// is created by the RangeInit instruction and pushed on a stack in the
// context. The RangeNext instruction uses this information to advance
// through the range, and determine when the range is exhausted.
type rangeDefinition struct {
	indexName string
	valueName string
	value     interface{}
	keySet    []interface{}
	runes     []rune
	index     int
}

// rangeInitByteCode implements the RangeInit opcode
//
// Inputs:
//
//	operand    - an array of two strings containing
//	             the names of the index and value
//	             variables.
//	stack+0    - The item to be "ranged" is stored
//	             on the stack. This can be a map,
//	             an array, a structure, or a channel
//
// The RangeInit opcode sets up the runtime context for
// a for..range operation. The index and value variables
// create created in a new symbol scope for the range,
// and for map types, a keyset is derived that will be
// used to step through the map.
//
// This information describing the range operation is
// pushed on a stack in the runtime context where it
// can be accessed by the RangeNext opcode. The stack
// allows nested for...range statements.
func rangeInitByteCode(c *Context, i interface{}) error {
	var (
		v   interface{}
		err error
		r   = rangeDefinition{}
	)

	if list, ok := i.([]interface{}); ok && len(list) == 2 {
		r.indexName = data.String(list[0])
		r.valueName = data.String(list[1])

		if r.indexName != "" && r.indexName != defs.DiscardedVariable {
			err = c.symbols.Create(r.indexName)
		}

		if err == nil && r.valueName != "" && r.valueName != defs.DiscardedVariable {
			err = c.symbols.Create(r.valueName)
		}
	}

	if err == nil {
		if v, err = c.Pop(); err == nil {
			if isStackMarker(v) {
				return c.runtimeError(errors.ErrFunctionReturnedVoid)
			}

			r.value = v

			switch actual := v.(type) {
			case string:
				keySet := make([]interface{}, 0)
				runes := make([]rune, 0)

				for i, ch := range actual {
					keySet = append(keySet, i)
					runes = append(runes, ch)
				}

				r.keySet = keySet
				r.runes = runes

			case *data.Map:
				r.keySet = actual.Keys()
				actual.SetReadonly(true)

			case *data.Array:
				actual.SetReadonly(true)

			case *data.Channel:
				// No further init required

			case int, int32, int64, int8, float32, float64:
				r.value, _ = data.Int(actual)
				r.index = 0

			default:
				err = c.runtimeError(errors.ErrInvalidType)
			}

			r.index = 0
			c.rangeStack = append(c.rangeStack, &r)
		}
	}

	return err
}

// rangeNextByteCode implements the RangeNext opcode
//
// Inputs:
//
//	operand    - The bytecode address to branch to
//	             when the range is exhausted.
//
// The RangeNext opcode fetches the top of the range
// stack from the runtime context, and evaluates the
// type of the item being ranged. For each type, the
// operations are similar:
//
//  1. Determine if the index is already outside the
//     range, in which case the branch is taken. The
//     topmost item on the range stack is discarded.
//
//  2. The range is incremented and value is read.
//     The value (map member, array index, channel)
//     is stored in the value variable. The index
//     number is also stored in the index variable.
func rangeNextByteCode(c *Context, i interface{}) error {
	var err error

	destination, err := data.Int(i)
	if err != nil {
		return c.runtimeError(err)
	}

	if stackSize := len(c.rangeStack); stackSize == 0 {
		c.programCounter = destination
	} else {
		r := c.rangeStack[stackSize-1]

		switch actual := r.value.(type) {
		case string:
			err = rangeNextString(c, r, destination, stackSize)

		case *data.Map:
			err = rangeNextMap(c, r, actual, destination, stackSize)

		case *data.Channel:
			err = rangeNextChannel(c, r, actual, destination, stackSize)

		case *data.Array:
			err = rangeNextArray(c, r, actual, destination, stackSize)

		case []interface{}:
			return errors.ErrInvalidType.Context("[]interface{}")

		case int:
			err = rangeNextInteger(c, r, actual, destination, stackSize)

		default:
			c.programCounter = destination
		}
	}

	return err
}

// Range over the next available integer in an integer range.
func rangeNextInteger(c *Context, r *rangeDefinition, actual int, destination int, stackSize int) error {
	var err error

	// If we've hit the end of the range of integer values, we're done.
	if (actual <= 0) || (r.index >= actual) {
		c.programCounter = destination
		c.rangeStack = c.rangeStack[:stackSize-1]
	} else {
		// store the index in index variable
		err = c.symbols.Set(r.indexName, r.index)
		r.index += 1
	}

	return err
}

// Range over the next element of an array object.
func rangeNextArray(c *Context, r *rangeDefinition, actual *data.Array, destination int, stackSize int) error {
	var err error

	if r.index >= actual.Len() {
		c.programCounter = destination
		c.rangeStack = c.rangeStack[:stackSize-1]

		actual.SetReadonly(false)
	} else {
		if r.indexName != "" && r.indexName != defs.DiscardedVariable {
			err = c.symbols.Set(r.indexName, r.index)
		}

		if err == nil && r.valueName != "" && r.valueName != defs.DiscardedVariable {
			var d interface{}

			d, err = actual.Get(r.index)
			if err == nil {
				err = c.symbols.Set(r.valueName, d)
			}
		}

		r.index++
	}

	return err
}

// Range over the next available data item in a channel object.
func rangeNextChannel(c *Context, r *rangeDefinition, actual *data.Channel, destination int, stackSize int) error {
	var (
		datum interface{}
		err   error
	)

	if actual.IsEmpty() {
		c.programCounter = destination
		c.rangeStack = c.rangeStack[:stackSize-1]
	} else {
		datum, err = actual.Receive()
		if err == nil {
			if r.indexName != "" && r.indexName != defs.DiscardedVariable {
				err = c.symbols.Set(r.indexName, r.index)
			}

			if err == nil && r.valueName != "" && r.valueName != defs.DiscardedVariable {
				err = c.symbols.Set(r.valueName, datum)
			}

			r.index++
		} else {
			c.programCounter = destination
			c.rangeStack = c.rangeStack[:stackSize-1]
		}
	}

	return err
}

// Range over the next member in a map.
func rangeNextMap(c *Context, r *rangeDefinition, actual *data.Map, destination int, stackSize int) error {
	var err error

	if r.index >= len(r.keySet) {
		c.programCounter = destination
		c.rangeStack = c.rangeStack[:stackSize-1]

		actual.SetReadonly(false)
	} else {
		key := r.keySet[r.index]

		if r.indexName != "" && r.indexName != defs.DiscardedVariable {
			err = c.symbols.Set(r.indexName, key)
		}

		if err == nil && r.valueName != "" && r.valueName != defs.DiscardedVariable {
			var value interface{}

			ok := false
			if value, ok, err = actual.Get(key); ok && err == nil {
				err = c.symbols.Set(r.valueName, value)
			} else {
				// If the key was deleted inside the loop, we set the value to nil
				err = c.symbols.Set(r.valueName, nil)
			}
		}

		r.index++
	}

	return err
}

// Range to next rune in a string. The list of runes is actually contains in the range key set.
func rangeNextString(c *Context, r *rangeDefinition, destination int, stackSize int) error {
	var err error

	if r.index >= len(r.keySet) {
		c.programCounter = destination
		c.rangeStack = c.rangeStack[:stackSize-1]
	} else {
		key := r.keySet[r.index]
		value := r.runes[r.index]

		if r.indexName != "" && r.indexName != defs.DiscardedVariable {
			err = c.symbols.Set(r.indexName, key)
		}

		if err == nil && r.valueName != "" && r.valueName != defs.DiscardedVariable {
			err = c.symbols.Set(r.valueName, string(value))
		}

		r.index++
	}

	return err
}
