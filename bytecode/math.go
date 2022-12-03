package bytecode

import (
	"math"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

/******************************************\
*                                         *
*     M A T H   P R I M I T I V E S       *
*                                         *
\******************************************/

// negateByteCode instruction processor pops the top stack
// item and pushes it's negative.
//
// *  booleans, a logical NOT
// *  numerics, arithmetic negation
// *  strings and arrays, reverse of element order
//
// If the argument is a boolen true, then this is a boolean
// NOT operations instead of a negation, which has narrower
// rules for how it must be processed.
func negateByteCode(c *Context, i interface{}) *errors.EgoError {
	if datatypes.GetBool(i) {
		return NotImpl(c, i)
	}

	v, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	// Cannot do math on a nil value
	if datatypes.IsNil(v) {
		return c.newError(errors.ErrInvalidType)
	}

	switch value := v.(type) {
	case bool:
		_ = c.stackPush(!value)

	case byte:
		_ = c.stackPush(-value)

	case int32:
		_ = c.stackPush(-value)

	case int:
		_ = c.stackPush(-value)

	case int64:
		_ = c.stackPush(-value)

	case float32:
		_ = c.stackPush(float32(0.0) - value)

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

	default:
		return c.newError(errors.ErrInvalidType)
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
		return c.newError(errors.ErrInvalidType)
	}

	switch value := v.(type) {
	case bool:
		_ = c.stackPush(!value)

	case byte, int32, int, int64:
		_ = c.stackPush(value == 0)

	case float32:
		_ = c.stackPush(value == float32(0))

	case float64:
		_ = c.stackPush(value == float64(0))

	default:
		return c.newError(errors.ErrInvalidType)
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
		return c.newError(errors.ErrInvalidType)
	}

	switch vx := v1.(type) {
	case error:
		return c.stackPush(vx.Error() + datatypes.GetString(v2))

		// All other types are scalar math.
	default:
		v1, v2 = datatypes.Normalize(v1, v2)

		switch v1.(type) {
		case byte:
			return c.stackPush(v1.(byte) + v2.(byte))

		case int32:
			return c.stackPush(v1.(int32) + v2.(int32))

		case int:
			return c.stackPush(v1.(int) + v2.(int))

		case int64:
			return c.stackPush(v1.(int64) + v2.(int64))

		case float32:
			return c.stackPush(v1.(float32) + v2.(float32))

		case float64:
			return c.stackPush(v1.(float64) + v2.(float64))

		case string:
			return c.stackPush(v1.(string) + v2.(string))

		case bool:
			return c.stackPush(v1.(bool) && v2.(bool))

		default:
			return c.newError(errors.ErrInvalidType)
		}
	}
}

// andByteCode bytecode instruction processor.
func andByteCode(c *Context, i interface{}) *errors.EgoError {
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
		return c.newError(errors.ErrInvalidType)
	}

	return c.stackPush(datatypes.GetBool(v1) && datatypes.GetBool(v2))
}

// orByteCode bytecode instruction processor.
func orByteCode(c *Context, i interface{}) *errors.EgoError {
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
		return c.newError(errors.ErrInvalidType)
	}

	return c.stackPush(datatypes.GetBool(v1) || datatypes.GetBool(v2))
}

// subtractByteCode instruction processor removes two items from the
// stack and subtracts them. For numeric values, this is arithmetic
// subtraction. For an array, the item to be subtracted is removed
// from the array (in any array location it is found).
func subtractByteCode(c *Context, i interface{}) *errors.EgoError {
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
		return c.newError(errors.ErrInvalidType)
	}

	v1, v2 = datatypes.Normalize(v1, v2)

	switch v1.(type) {
	case byte:
		return c.stackPush(v1.(byte) - v2.(byte))

	case int32:
		return c.stackPush(v1.(int32) - v2.(int32))

	case int:
		return c.stackPush(v1.(int) - v2.(int))

	case int64:
		return c.stackPush(v1.(int64) - v2.(int64))

	case float32:
		return c.stackPush(v1.(float32) - v2.(float32))

	case float64:
		return c.stackPush(v1.(float64) - v2.(float64))

	case string:
		s := strings.ReplaceAll(v1.(string), v2.(string), "")

		return c.stackPush(s)

	default:
		return c.newError(errors.ErrInvalidType)
	}
}

// multiplyByteCode bytecode instruction processor.
func multiplyByteCode(c *Context, i interface{}) *errors.EgoError {
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
		return c.newError(errors.ErrInvalidType)
	}

	// Special case of multiply of string by integer to repeat string
	if (datatypes.KindOf(v1) == datatypes.StringKind) &&
		datatypes.IsNumeric(v2) {
		str := datatypes.GetString(v1)
		count := datatypes.GetInt(v2)
		r := strings.Repeat(str, count)

		return c.stackPush(r)
	}

	// Nope, plain old math multiply, so normalize the values.
	v1, v2 = datatypes.Normalize(v1, v2)

	switch v1.(type) {
	case bool:
		return c.stackPush(v1.(bool) || v2.(bool))

	case byte:
		return c.stackPush(v1.(byte) * v2.(byte))

	case int32:
		return c.stackPush(v1.(int32) * v2.(int32))

	case int:
		return c.stackPush(v1.(int) * v2.(int))

	case int64:
		return c.stackPush(v1.(int64) * v2.(int64))

	case float32:
		return c.stackPush(v1.(float32) * v2.(float32))

	case float64:
		return c.stackPush(v1.(float64) * v2.(float64))

	default:
		return c.newError(errors.ErrInvalidType)
	}
}

// exponentByteCode bytecode instruction processor.
func exponentByteCode(c *Context, i interface{}) *errors.EgoError {
	v2, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, err := c.Pop()
	if !errors.Nil(err) {
		return err
	}

	v1, v2 = datatypes.Normalize(v1, v2)

	// Cannot do math on a nil value
	if datatypes.IsNil(v1) || datatypes.IsNil(v2) {
		return c.newError(errors.ErrInvalidType)
	}

	switch v1.(type) {
	case byte, int32, int, int64:
		vv1 := datatypes.GetInt64(v1)
		vv2 := datatypes.GetInt64(v2)

		if vv2 == 0 {
			return c.stackPush(0)
		}

		if vv2 == 1 {
			return c.stackPush(v1)
		}

		prod := vv1

		for n := int64(2); n <= vv2; n = n + 1 {
			prod = prod * vv1
		}

		return c.stackPush(prod)

	case float32:
		return c.stackPush(float32(math.Pow(float64(v1.(float32)), float64(v2.(float32)))))

	case float64:
		return c.stackPush(math.Pow(v1.(float64), v2.(float64)))

	default:
		return c.newError(errors.ErrInvalidType)
	}
}

// divideByteCode bytecode instruction processor.
func divideByteCode(c *Context, i interface{}) *errors.EgoError {
	if c.stackPointer < 1 {
		return c.newError(errors.ErrStackUnderflow)
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
		return c.newError(errors.ErrInvalidType)
	}

	v1, v2 = datatypes.Normalize(v1, v2)

	switch v1.(type) {
	case byte:
		if v2.(byte) == 0 {
			return c.newError(errors.ErrDivisionByZero)
		}

		return c.stackPush(v1.(byte) / v2.(byte))

	case int32:
		if v2.(int32) == 0 {
			return c.newError(errors.ErrDivisionByZero)
		}

		return c.stackPush(v1.(int32) / v2.(int32))

	case int:
		if v2.(int) == 0 {
			return c.newError(errors.ErrDivisionByZero)
		}

		return c.stackPush(v1.(int) / v2.(int))

	case int64:
		if v2.(int64) == 0 {
			return c.newError(errors.ErrDivisionByZero)
		}

		return c.stackPush(v1.(int64) / v2.(int64))

	case float32:
		if v2.(float32) == 0 {
			return c.newError(errors.ErrDivisionByZero)
		}

		return c.stackPush(v1.(float32) / v2.(float32))

	case float64:
		if v2.(float64) == 0 {
			return c.newError(errors.ErrDivisionByZero)
		}

		return c.stackPush(v1.(float64) / v2.(float64))

	default:
		return c.newError(errors.ErrInvalidType)
	}
}

// moduloByteCode bytecode instruction processor.
func moduloByteCode(c *Context, i interface{}) *errors.EgoError {
	if c.stackPointer < 1 {
		return c.newError(errors.ErrStackUnderflow)
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
		return c.newError(errors.ErrInvalidType)
	}

	v1, v2 = datatypes.Normalize(v1, v2)

	switch v1.(type) {
	case byte:
		if v2.(byte) == 0 {
			return c.newError(errors.ErrDivisionByZero)
		}

		return c.stackPush(v1.(byte) % v2.(byte))

	case int32:
		if v2.(int32) == 0 {
			return c.newError(errors.ErrDivisionByZero)
		}

		return c.stackPush(v1.(int32) % v2.(int32))

	case int:
		if v2.(int) == 0 {
			return c.newError(errors.ErrDivisionByZero)
		}

		return c.stackPush(v1.(int) % v2.(int))

	case int64:
		if v2.(int64) == 0 {
			return c.newError(errors.ErrDivisionByZero)
		}

		return c.stackPush(v1.(int64) % v2.(int64))

	default:
		return c.newError(errors.ErrInvalidType)
	}
}

func bitAndByteCode(c *Context, i interface{}) *errors.EgoError {
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
		return c.newError(errors.ErrInvalidType)
	}

	result := datatypes.GetInt(v1) & datatypes.GetInt(v2)
	_ = c.stackPush(result)

	return nil
}

func bitOrByteCode(c *Context, i interface{}) *errors.EgoError {
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
		return c.newError(errors.ErrInvalidType)
	}

	result := datatypes.GetInt(v1) | datatypes.GetInt(v2)
	_ = c.stackPush(result)

	return nil
}

func bitShiftByteCode(c *Context, i interface{}) *errors.EgoError {
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
		return c.newError(errors.ErrInvalidType)
	}

	shift := datatypes.GetInt(v1)
	value := datatypes.GetInt(v2)

	if shift < -31 || shift > 31 {
		return c.newError(errors.ErrInvalidBitShift).Context(shift)
	}

	if shift < 0 {
		value = value << -shift
	} else {
		value = value >> shift
	}

	_ = c.stackPush(value)

	return nil
}
