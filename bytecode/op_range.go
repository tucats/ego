package bytecode

import (
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/util"
)

// Range describes what we know about the (current) for..range loop. This
// is created by the RangeInit instruction and pushed on a stack in the
// context. The RangeNext instruction uses this information to advance
// through the range, and determine when the range is exhausted.
type Range struct {
	indexName string
	valueName string
	value     interface{}
	keySet    []interface{}
	runes     []rune
	index     int
}

// RangeInitImpl implements the RangeInit opcode
//
// Inputs:
//    operand    - an array of two strings containing
//                 the names of the index and value
//                 variables.
//    stack+0    - The item to be "ranged" is stored
//                 on the stack. This can be a map,
//                 an array, a structure, or a channel
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
func RangeInitImpl(c *Context, i interface{}) error {
	var v interface{}

	var err error

	r := Range{}

	if list, ok := i.([]interface{}); ok && len(list) == 2 {
		r.indexName = util.GetString(list[0])
		r.valueName = util.GetString(list[1])

		if r.indexName != "" && r.indexName != "_" {
			err = c.symbols.Create(r.indexName)
		}

		if err == nil && r.valueName != "" && r.valueName != "_" {
			err = c.symbols.Create(r.valueName)
		}
	}

	if err == nil {
		if v, err = c.Pop(); err == nil {
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

			case map[string]interface{}:
				r.keySet = []interface{}{}
				i := 0

				for k := range actual {
					if !strings.HasPrefix(k, "__") {
						r.keySet = append(r.keySet, k)
						i++
					}
				}

			case *datatypes.EgoMap:
				r.keySet = actual.Keys()
				actual.ImmutableKeys(true)

			case *datatypes.Channel:
				// No further init required

			case []interface{}:
				// No further init required

			default:
				err = c.NewError(InvalidTypeError)
			}

			r.index = 0
			c.rangeStack = append(c.rangeStack, &r)
		}
	}

	return err
}

// RangeNextImpl implements the RangeNext opcode
//
// Inputs:
//    operand    - The bytecode address to branch to
//                 when the range is exhausted.
//
// The RangeNext opcode fetches the top of the range
// stack from the runtime context, and evaluates the
// type of the item being ranged. For each type, the
// operations are similar:
//
// 1. Determine if the index is already outside the
//    range, in which case the branch is taken. The
//    topmost item on the range stack is discarded.
//
// 2. The range is incremented and the associated
//    value (map member, array index, channel value)
//    is read and stored in the value variable. The
//    index number is also stored in the index variable.
func RangeNextImpl(c *Context, i interface{}) error {
	var err error

	destination := util.GetInt(i)

	stackSize := len(c.rangeStack)
	if stackSize == 0 {
		c.pc = destination
	} else {
		r := c.rangeStack[stackSize-1]

		switch actual := r.value.(type) {
		case string:
			if r.index >= len(r.keySet) {
				c.pc = destination
				c.rangeStack = c.rangeStack[:stackSize-1]
			} else {
				key := r.keySet[r.index]
				value := r.runes[r.index]

				if r.indexName != "" && r.indexName != "_" {
					err = c.symbols.Set(r.indexName, key)
				}

				if err == nil && r.valueName != "" && r.valueName != "_" {
					err = c.symbols.Set(r.valueName, string(value))
				}

				r.index++
			}

		case map[string]interface{}:
			if r.index >= len(r.keySet) {
				c.pc = destination
				c.rangeStack = c.rangeStack[:stackSize-1]
			} else {
				key := r.keySet[r.index]

				if r.indexName != "" && r.indexName != "_" {
					err = c.symbols.Set(r.indexName, key)
				}

				if err == nil && r.valueName != "" && r.valueName != "_" {
					err = c.symbols.Set(r.valueName, actual[util.GetString(key)])
				}

				r.index++
			}

		case *datatypes.EgoMap:
			if r.index >= len(r.keySet) {
				c.pc = destination
				c.rangeStack = c.rangeStack[:stackSize-1]

				actual.ImmutableKeys(false)
			} else {
				key := r.keySet[r.index]

				if r.indexName != "" && r.indexName != "_" {
					err = c.symbols.Set(r.indexName, key)
				}

				if err == nil && r.valueName != "" && r.valueName != "_" {
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

		case *datatypes.Channel:
			var datum interface{}

			if actual.IsEmpty() {
				c.pc = destination
				c.rangeStack = c.rangeStack[:stackSize-1]
			} else {
				datum, err = actual.Receive()
				if err == nil {
					if r.indexName != "" && r.indexName != "_" {
						err = c.symbols.Set(r.indexName, r.index)
					}
					if err == nil && r.valueName != "" && r.valueName != "_" {
						err = c.symbols.Set(r.valueName, datum)
					}

					r.index++
				} else {
					c.pc = destination
					c.rangeStack = c.rangeStack[:stackSize-1]
				}
			}

		case []interface{}:
			if r.index >= len(actual) {
				c.pc = destination
				c.rangeStack = c.rangeStack[:stackSize-1]
			} else {
				if r.indexName != "" && r.indexName != "_" {
					err = c.symbols.Set(r.indexName, r.index)
				}
				if err == nil && r.valueName != "" && r.valueName != "_" {
					err = c.symbols.Set(r.valueName, actual[r.index])
				}
				r.index++
			}

		default:
			c.pc = destination
		}
	}

	return err
}
