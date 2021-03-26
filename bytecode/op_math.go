package bytecode

import (
	"math"
	"reflect"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

/******************************************\
*                                         *
*     M A T H   P R I M I T I V E S       *
*                                         *
\******************************************/

// NegateImpl instruction processor pops the top stack
// item and pushes it's negative.
//
// *  booleans, a logical NOT
// *  numerics, arithmetic negation
// *  strings and arrays, reverse of element order
//
// If the argument is a boolen true, then this is a boolean
// NOT operations instead of a negation, which has narrower
// rules for how it must be processed.
func NegateImpl(c *Context, i interface{}) *errors.EgoError {
	kind := util.GetBool(i)
	if kind {
		return NotImpl(c, i)
	}

	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Cannot do math on a nil value
	if datatypes.IsNil(v) {
		return c.newError(errors.InvalidTypeError)
	}

	switch value := v.(type) {
	case bool:
		_ = c.stackPush(!value)

	case int:
		_ = c.stackPush(-value)

	case float64:
		_ = c.stackPush(0.0 - value)

	case string:
		length := 0
		for range value {
			length++
		}

		runes := make([]rune, length)
		for idx, rune := range value {
			runes[length-idx-1] = rune
		}

		result := string(runes)
		_ = c.stackPush(result)

	case *datatypes.EgoArray:
		// Create an array in inverse order.
		r := datatypes.NewArray(value.ValueType(), value.Len())

		for n := 0; n < value.Len(); n = n + 1 {
			d, _ := value.Get(n)
			_ = r.Set(value.Len()-n-1, d)
		}

		_ = c.stackPush(r)

	case []interface{}:
		// Create an array in inverse order.
		r := make([]interface{}, len(value))

		for n, d := range value {
			r[len(value)-n-1] = d
		}

		_ = c.stackPush(r)

	default:
		return c.newError(errors.InvalidTypeError)
	}

	return nil
}

// NotImpl instruction processor pops the top stack
// item and pushes it's boolean NOT value.
func NotImpl(c *Context, i interface{}) *errors.EgoError {
	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Cannot do math on a nil value
	if datatypes.IsNil(v) {
		return c.newError(errors.InvalidTypeError)
	}

	switch value := v.(type) {
	case bool:
		_ = c.stackPush(!value)

	case int:
		if value != 0 {
			_ = c.stackPush(false)
		} else {
			_ = c.stackPush(true)
		}

	case float64:
		if value != 0.0 {
			_ = c.stackPush(false)
		} else {
			_ = c.stackPush(true)
		}

	default:
		return c.newError(errors.InvalidTypeError)
	}

	return nil
}

// addByteCode bytecode instruction processor. This removes the top two
// items and adds them together. For boolean values, this is an OR
// operation. For numeric values, it is arithmetic addition. For
// strings or arrays, it concatenates the two items. For a struct,
// it merges the addend into the first struct.
func addByteCode(c *Context, i interface{}) *errors.EgoError {
	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Cannot do math on a nil value
	if datatypes.IsNil(v1) || datatypes.IsNil(v2) {
		return c.newError(errors.InvalidTypeError)
	}

	switch vx := v1.(type) {
	case error:
		return c.stackPush(vx.Error() + util.GetString(v2))

	// Is it a native array we are concatenating to?
	case []interface{}:
		switch vy := v2.(type) {
		// Array requires a deep concatenation.
		case []interface{}:
			// If we're in static type mode, each member of the
			// array being added must match the type of the target
			// array.
			if c.Static {
				arrayType := reflect.TypeOf(vx[0])

				for _, vv := range vy {
					if arrayType != reflect.TypeOf(vv) {
						return c.newError(errors.InvalidTypeError)
					}
				}
			}

			newArray := append(vx, vy...)

			return c.stackPush(newArray)

		// Everything else is a simple append.
		default:
			newArray := append(vx, v2)

			return c.stackPush(newArray)
		}

		// You can add a map to another map.
	case map[string]interface{}:
		switch vy := v2.(type) {
		case map[string]interface{}:
			t := datatypes.Struct("")

			for k, v := range vy {
				if !strings.HasPrefix(k, "__") {
					vx[k] = v
				}
			}

			for k, v := range vx {
				if !strings.HasPrefix(k, "__") {
					_ = t.AddField(k, datatypes.TypeOf(v))
				}
			}

			// Write updated type info
			datatypes.SetMetadata(vx, datatypes.TypeMDKey, t)

			return c.stackPush(vx)

		default:
			return c.newError(errors.InvalidTypeError)
		}

		// All other types are scalar math.
	default:
		v1, v2 = util.Normalize(v1, v2)

		switch v1.(type) {
		case int:
			return c.stackPush(v1.(int) + v2.(int))

		case float64:
			return c.stackPush(v1.(float64) + v2.(float64))

		case string:
			return c.stackPush(v1.(string) + v2.(string))

		case bool:
			return c.stackPush(v1.(bool) && v2.(bool))

		default:
			return c.newError(errors.InvalidTypeError)
		}
	}
}

// AndImpl bytecode instruction processor.
func AndImpl(c *Context, i interface{}) *errors.EgoError {
	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Cannot do math on a nil value
	if datatypes.IsNil(v1) || datatypes.IsNil(v2) {
		return c.newError(errors.InvalidTypeError)
	}

	return c.stackPush(util.GetBool(v1) && util.GetBool(v2))
}

// OrImpl bytecode instruction processor.
func OrImpl(c *Context, i interface{}) *errors.EgoError {
	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Cannot do math on a nil value
	if datatypes.IsNil(v1) || datatypes.IsNil(v2) {
		return c.newError(errors.InvalidTypeError)
	}

	return c.stackPush(util.GetBool(v1) || util.GetBool(v2))
}

// SubtractImpl instruction processor removes two items from the
// stack and subtracts them. For numeric values, this is arithmetic
// subtraction. For an array, the item to be subtracted is removed
// from the array (in any array location it is found).
func SubtractImpl(c *Context, i interface{}) *errors.EgoError {
	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Cannot do math on a nil value
	if datatypes.IsNil(v1) || datatypes.IsNil(v2) {
		return c.newError(errors.InvalidTypeError)
	}

	switch vx := v1.(type) {
	// For an array, make a copy removing the item to be subtracted.
	case []interface{}:
		newArray := make([]interface{}, 0)

		for _, v := range vx {
			if !reflect.DeepEqual(v2, v) {
				newArray = append(newArray, v)
			}
		}

		return c.stackPush(newArray)

	// Everything else is a scalar subtraction.
	default:
		v1, v2 = util.Normalize(v1, v2)

		switch v1.(type) {
		case int:
			return c.stackPush(v1.(int) - v2.(int))

		case float64:
			return c.stackPush(v1.(float64) - v2.(float64))

		case string:
			s := strings.ReplaceAll(v1.(string), v2.(string), "")

			return c.stackPush(s)

		default:
			return c.newError(errors.InvalidTypeError)
		}
	}
}

// MultiplyImpl bytecode instruction processor.
func MultiplyImpl(c *Context, i interface{}) *errors.EgoError {
	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Cannot do math on a nil value
	if datatypes.IsNil(v1) || datatypes.IsNil(v2) {
		return c.newError(errors.InvalidTypeError)
	}

	v1, v2 = util.Normalize(v1, v2)

	switch v1.(type) {
	case int:
		return c.stackPush(v1.(int) * v2.(int))

	case float64:
		return c.stackPush(v1.(float64) * v2.(float64))

	case bool:
		return c.stackPush(v1.(bool) || v2.(bool))

	default:
		return c.newError(errors.InvalidTypeError)
	}
}

// ExponentImpl bytecode instruction processor.
func ExponentImpl(c *Context, i interface{}) *errors.EgoError {
	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, v2 = util.Normalize(v1, v2)

	// Cannot do math on a nil value
	if datatypes.IsNil(v1) || datatypes.IsNil(v2) {
		return c.newError(errors.InvalidTypeError)
	}

	switch v1.(type) {
	case int:
		if v2.(int) == 0 {
			return c.stackPush(0)
		}

		if v2.(int) == 1 {
			return c.stackPush(v1)
		}

		prod := v1.(int)

		for n := 2; n <= v2.(int); n = n + 1 {
			prod = prod * v1.(int)
		}

		return c.stackPush(prod)

	case float64:
		return c.stackPush(math.Pow(v1.(float64), v2.(float64)))

	default:
		return c.newError(errors.InvalidTypeError)
	}
}

// DivideImpl bytecode instruction processor.
func DivideImpl(c *Context, i interface{}) *errors.EgoError {
	if c.sp < 1 {
		return c.newError(errors.StackUnderflowError)
	}

	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Cannot do math on a nil value
	if datatypes.IsNil(v1) || datatypes.IsNil(v2) {
		return c.newError(errors.InvalidTypeError)
	}

	v1, v2 = util.Normalize(v1, v2)

	switch v1.(type) {
	case int:
		if v2.(int) == 0 {
			return c.newError(errors.DivisionByZeroError)
		}

		return c.stackPush(v1.(int) / v2.(int))

	case float64:
		if v2.(float64) == 0 {
			return c.newError(errors.DivisionByZeroError)
		}

		return c.stackPush(v1.(float64) / v2.(float64))

	default:
		return c.newError(errors.InvalidTypeError)
	}
}
