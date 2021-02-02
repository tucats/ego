package bytecode

import (
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/util"
)

func RangeInitImpl(c *Context, i interface{}) error {
	r := Range{}
	var v interface{}
	var err error

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

func RangeNextImpl(c *Context, i interface{}) error {
	var err error
	destination := util.GetInt(i)
	stackSize := len(c.rangeStack)
	if stackSize == 0 {
		c.pc = destination
	} else {
		r := c.rangeStack[stackSize-1]
		switch actual := r.value.(type) {
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
					if value, ok, err = actual.Get(key); ok {
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
