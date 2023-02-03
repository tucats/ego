package bytecode

import (
	"math"
	"strings"

	"github.com/tucats/ego/data"
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
func negateByteCode(c *Context, i interface{}) error {
	if data.Bool(i) {
		return notByteCode(c, i)
	}

	v, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	switch value := v.(type) {
	case bool:
		return c.push(!value)

	case byte:
		return c.push(-value)

	case int32:
		return c.push(-value)

	case int:
		return c.push(-value)

	case int64:
		return c.push(-value)

	case float32:
		return c.push(float32(0.0) - value)

	case float64:
		return c.push(0.0 - value)

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

		return c.push(result)

	case *data.Array:
		// Create an array in inverse order.
		r := data.NewArray(value.Type(), value.Len())

		for n := 0; n < value.Len(); n = n + 1 {
			d, _ := value.Get(n)
			_ = r.Set(value.Len()-n-1, d)
		}

		return c.push(r)

	default:
		return c.error(errors.ErrInvalidType)
	}
}

// notByteCode instruction processor pops the top stack
// item and pushes it's boolean NOT value.
func notByteCode(c *Context, i interface{}) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// A nil value is treated as false, so !nil is true
	if data.IsNil(v) {
		return c.push(true)
	}

	switch value := v.(type) {
	case bool:
		return c.push(!value)

	case byte, int32, int, int64:
		return c.push(value == 0)

	case float32:
		return c.push(value == float32(0))

	case float64:
		return c.push(value == float64(0))

	default:
		return c.error(errors.ErrInvalidType)
	}
}

// addByteCode bytecode instruction processor. This removes the top two
// items and adds them together. For boolean values, this is an OR
// operation. For numeric values, it is arithmetic addition. For
// strings or arrays, it concatenates the two items. For a struct,
// it merges the addend into the first struct.
func addByteCode(c *Context, i interface{}) error {
	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	switch vx := v1.(type) {
	case error:
		return c.push(vx.Error() + data.String(v2))

		// All other types are scalar math.
	default:
		if c.typeStrictness > 0 {
			v1, v2 = data.Normalize(v1, v2)
		} else {
			if !data.TypeOf(v1).IsType(data.TypeOf(v2)) {
				return c.error(errors.ErrTypeMismatch).
					Context(data.TypeOf(v2).String() + ", " + data.TypeOf(v1).String())
			}
		}

		switch v1.(type) {
		case byte:
			return c.push(v1.(byte) + v2.(byte))

		case int32:
			return c.push(v1.(int32) + v2.(int32))

		case int:
			return c.push(v1.(int) + v2.(int))

		case int64:
			return c.push(v1.(int64) + v2.(int64))

		case float32:
			return c.push(v1.(float32) + v2.(float32))

		case float64:
			return c.push(v1.(float64) + v2.(float64))

		case string:
			return c.push(v1.(string) + v2.(string))

		case bool:
			return c.push(v1.(bool) && v2.(bool))

		default:
			return c.error(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
		}
	}
}

// andByteCode bytecode instruction processor.
func andByteCode(c *Context, i interface{}) error {
	v1, err := c.Pop()
	if err != nil {
		return err
	}

	v2, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	return c.push(data.Bool(v1) && data.Bool(v2))
}

// orByteCode bytecode instruction processor.
func orByteCode(c *Context, i interface{}) error {
	v1, err := c.Pop()
	if err != nil {
		return err
	}

	v2, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	return c.push(data.Bool(v1) || data.Bool(v2))
}

// subtractByteCode instruction processor removes two items from the
// stack and subtracts them. For numeric values, this is arithmetic
// subtraction. For an array, the item to be subtracted is removed
// from the array (in any array location it is found).
func subtractByteCode(c *Context, i interface{}) error {
	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	if c.typeStrictness > 0 {
		v1, v2 = data.Normalize(v1, v2)
	} else {
		if !data.TypeOf(v1).IsType(data.TypeOf(v2)) {
			return c.error(errors.ErrTypeMismatch).
				Context(data.TypeOf(v2).String() + ", " + data.TypeOf(v1).String())
		}
	}

	switch v1.(type) {
	case byte:
		return c.push(v1.(byte) - v2.(byte))

	case int32:
		return c.push(v1.(int32) - v2.(int32))

	case int:
		return c.push(v1.(int) - v2.(int))

	case int64:
		return c.push(v1.(int64) - v2.(int64))

	case float32:
		return c.push(v1.(float32) - v2.(float32))

	case float64:
		return c.push(v1.(float64) - v2.(float64))

	case string:
		s := strings.ReplaceAll(v1.(string), v2.(string), "")

		return c.push(s)

	default:
		return c.error(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
	}
}

// multiplyByteCode bytecode instruction processor.
func multiplyByteCode(c *Context, i interface{}) error {
	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	// Special case of multiply of string by integer to repeat string
	if (data.KindOf(v1) == data.StringKind) &&
		data.IsNumeric(v2) {
		str := data.String(v1)
		count := data.Int(v2)
		r := strings.Repeat(str, count)

		return c.push(r)
	}

	// Nope, plain old math multiply, so normalize the values.
	if c.typeStrictness > 0 {
		v1, v2 = data.Normalize(v1, v2)
	} else {
		if !data.TypeOf(v1).IsType(data.TypeOf(v2)) {
			return c.error(errors.ErrTypeMismatch).
				Context(data.TypeOf(v2).String() + ", " + data.TypeOf(v1).String())
		}
	}

	switch v1.(type) {
	case bool:
		return c.push(v1.(bool) || v2.(bool))

	case byte:
		return c.push(v1.(byte) * v2.(byte))

	case int32:
		return c.push(v1.(int32) * v2.(int32))

	case int:
		return c.push(v1.(int) * v2.(int))

	case int64:
		return c.push(v1.(int64) * v2.(int64))

	case float32:
		return c.push(v1.(float32) * v2.(float32))

	case float64:
		return c.push(v1.(float64) * v2.(float64))

	default:
		return c.error(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
	}
}

// exponentByteCode bytecode instruction processor.
func exponentByteCode(c *Context, i interface{}) error {
	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	if c.typeStrictness > 0 {
		v1, v2 = data.Normalize(v1, v2)
	} else {
		if !data.TypeOf(v1).IsType(data.TypeOf(v2)) {
			return c.error(errors.ErrTypeMismatch).
				Context(data.TypeOf(v2).String() + ", " + data.TypeOf(v1).String())
		}
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	switch v1.(type) {
	case byte, int32, int, int64:
		vv1 := data.Int64(v1)
		vv2 := data.Int64(v2)

		if vv2 == 0 {
			return c.push(0)
		}

		if vv2 == 1 {
			return c.push(v1)
		}

		prod := vv1

		for n := int64(2); n <= vv2; n = n + 1 {
			prod = prod * vv1
		}

		return c.push(prod)

	case float32:
		return c.push(float32(math.Pow(float64(v1.(float32)), float64(v2.(float32)))))

	case float64:
		return c.push(math.Pow(v1.(float64), v2.(float64)))

	default:
		return c.error(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
	}
}

// divideByteCode bytecode instruction processor.
func divideByteCode(c *Context, i interface{}) error {
	if c.stackPointer < 1 {
		return c.error(errors.ErrStackUnderflow)
	}

	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	if c.typeStrictness > 0 {
		v1, v2 = data.Normalize(v1, v2)
	} else {
		if !data.TypeOf(v1).IsType(data.TypeOf(v2)) {
			return c.error(errors.ErrTypeMismatch).
				Context(data.TypeOf(v2).String() + ", " + data.TypeOf(v1).String())
		}
	}

	switch v1.(type) {
	case byte:
		if v2.(byte) == 0 {
			return c.error(errors.ErrDivisionByZero)
		}

		return c.push(v1.(byte) / v2.(byte))

	case int32:
		if v2.(int32) == 0 {
			return c.error(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int32) / v2.(int32))

	case int:
		if v2.(int) == 0 {
			return c.error(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int) / v2.(int))

	case int64:
		if v2.(int64) == 0 {
			return c.error(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int64) / v2.(int64))

	case float32:
		if v2.(float32) == 0 {
			return c.error(errors.ErrDivisionByZero)
		}

		return c.push(v1.(float32) / v2.(float32))

	case float64:
		if v2.(float64) == 0 {
			return c.error(errors.ErrDivisionByZero)
		}

		return c.push(v1.(float64) / v2.(float64))

	default:
		return c.error(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
	}
}

// moduloByteCode bytecode instruction processor.
func moduloByteCode(c *Context, i interface{}) error {
	if c.stackPointer < 1 {
		return c.error(errors.ErrStackUnderflow)
	}

	v2, err := c.Pop()
	if err != nil {
		return err
	}

	v1, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	if c.typeStrictness > 0 {
		v1, v2 = data.Normalize(v1, v2)
	} else {
		if !data.TypeOf(v1).IsType(data.TypeOf(v2)) {
			return c.error(errors.ErrTypeMismatch).
				Context(data.TypeOf(v2).String() + ", " + data.TypeOf(v1).String())
		}
	}

	switch v1.(type) {
	case byte:
		if v2.(byte) == 0 {
			return c.error(errors.ErrDivisionByZero)
		}

		return c.push(v1.(byte) % v2.(byte))

	case int32:
		if v2.(int32) == 0 {
			return c.error(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int32) % v2.(int32))

	case int:
		if v2.(int) == 0 {
			return c.error(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int) % v2.(int))

	case int64:
		if v2.(int64) == 0 {
			return c.error(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int64) % v2.(int64))

	default:
		return c.error(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
	}
}

func bitAndByteCode(c *Context, i interface{}) error {
	v1, err := c.Pop()
	if err != nil {
		return err
	}

	v2, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	result := data.Int(v1) & data.Int(v2)

	return c.push(result)
}

func bitOrByteCode(c *Context, i interface{}) error {
	v1, err := c.Pop()
	if err != nil {
		return err
	}

	v2, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	result := data.Int(v1) | data.Int(v2)

	return c.push(result)
}

func bitShiftByteCode(c *Context, i interface{}) error {
	v1, err := c.Pop()
	if err != nil {
		return err
	}

	v2, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.error(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.error(errors.ErrInvalidType).Context("nil")
	}

	shift := data.Int(v1)
	value := data.Int(v2)

	if shift < -31 || shift > 31 {
		return c.error(errors.ErrInvalidBitShift).Context(shift)
	}

	if shift < 0 {
		value = value << -shift
	} else {
		value = value >> shift
	}

	return c.push(value)
}
