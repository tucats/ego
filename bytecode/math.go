package bytecode

import (
	"math"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

/******************************************\
*                                         *
*     M A T H   P R I M I T I V E S       *
*                                         *
\******************************************/

// incrementByteCode instruction processor accepts two arguments
// that are the name of a variable and a numeric value. The named
// variable is incremented by the value.
func incrementByteCode(c *Context, i interface{}) error {
	var (
		err       error
		symbol    string
		increment interface{}
	)

	if operands, ok := i.([]interface{}); ok && len(operands) == 2 {
		symbol = data.String(operands[0])

		increment = operands[1]
		if c, ok := increment.(data.Immutable); ok {
			increment = c.Value
		}
	} else {
		return c.runtimeError(errors.ErrInvalidOperand)
	}

	// Get the symbol value
	v, found := c.get(symbol)
	if !found {
		return c.runtimeError(errors.ErrUnknownSymbol).Context(symbol)
	}

	v = c.unwrapConstant(v)

	// Cannot do math on a nil value
	if data.IsNil(v) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	// Some special cases. If v is an array, then we are being asked
	// to append an element to the array. This is only done when
	// language extensions are enabled.
	if a, ok := v.(*data.Array); ok && c.extensions {
		// If the value being added isn't an array, coerce it to the
		// type of the base array. We don't do this for interface arrays.
		if _, ok := increment.(*data.Array); !ok && c.typeStrictness != defs.StrictTypeEnforcement {
			if a.Type().BaseType().Kind() != data.InterfaceType.Kind() {
				increment, err = data.Coerce(increment, data.InstanceOfType(a.Type().BaseType()))
				if err != nil {
					return c.runtimeError(err)
				}
			}
		}

		a.Append(increment)

		return c.set(symbol, a)
	}

	// Normalize the values and add them.
	if c.typeStrictness != defs.StrictTypeEnforcement {
		v, increment, err = data.Normalize(v, increment)
		if err != nil {
			return c.runtimeError(err)
		}
	} else {
		if !data.TypeOf(v).IsType(data.TypeOf(increment)) {
			return c.runtimeError(errors.ErrTypeMismatch)
		}
	}

	switch value := v.(type) {
	case byte:
		return c.set(symbol, value+increment.(byte))

	case int32:
		return c.set(symbol, value+increment.(int32))

	case int:
		return c.set(symbol, value+increment.(int))

	case int64:
		return c.set(symbol, value+increment.(int64))

	case float32:
		return c.set(symbol, value+increment.(float32))

	case float64:
		return c.set(symbol, value+increment.(float64))

	case string:
		return c.set(symbol, value+increment.(string))

	default:
		return c.runtimeError(errors.ErrInvalidType)
	}
}

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
	b, err := data.Bool(i)
	if err != nil {
		return c.runtimeError(err)
	}

	if b {
		return notByteCode(c, nil)
	}

	v, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
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
		return c.runtimeError(errors.ErrInvalidType)
	}
}

// notByteCode instruction processor pops the top stack
// item and pushes it's boolean NOT value. If the operand
// is non-nill, that is used as the value to negate instead
// of the top of the stack.
func notByteCode(c *Context, i interface{}) error {
	var (
		v   interface{}
		err error
	)

	if i != nil {
		v = i
	} else {
		v, err = c.Pop()
		if err != nil {
			return err
		}
	}

	if isStackMarker(v) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
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
		return c.runtimeError(errors.ErrInvalidType)
	}
}

// addByteCode bytecode instruction processor. This removes the top two
// items and adds them together. For boolean values, this is an OR
// operation. For numeric values, it is arithmetic addition. For
// strings or arrays, it concatenates the two items. For a struct,
// it merges the addend into the first struct.
func addByteCode(c *Context, i interface{}) error {
	var coerceOk bool

	v2, err := c.PopWithoutUnwrapping()
	if err != nil {
		return err
	}

	v1, err := c.PopWithoutUnwrapping()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	if c, ok := v1.(data.Immutable); ok {
		v1 = c.Value
		coerceOk = true
	}

	if c, ok := v2.(data.Immutable); ok {
		v2 = c.Value
		coerceOk = true
	}

	// Some special cases. If v1 is an array, then we are being
	// asked to append an element to the array. This is only
	// done when language extensions are enabled.
	if a, ok := v1.(*data.Array); ok && c.extensions {
		// If the value being added isn't an array, coerce it to the
		// type of the base array.
		if _, ok := v2.(*data.Array); !ok && c.typeStrictness != defs.StrictTypeEnforcement {
			v2, err = data.Coerce(v2, data.InstanceOfType(a.Type().BaseType()))
			if err != nil {
				return c.runtimeError(err)
			}
		}

		a.Append(v2)

		return c.push(a)
	}

	// Same for when v2 is the array.
	if a, ok := v2.(*data.Array); ok && c.extensions {
		// If the value being added isn't an array, coerce it to the
		// type of the base array.
		if _, ok := v1.(*data.Array); !ok && c.typeStrictness != defs.StrictTypeEnforcement {
			v1, err = data.Coerce(v1, data.InstanceOfType(a.Type().BaseType()))
			if err != nil {
				return c.runtimeError(err)
			}
		}

		a.Append(v1)

		return c.push(a)
	}

	if !coerceOk && c.typeStrictness == defs.StrictTypeEnforcement {
		if data.KindOf(v1) != data.KindOf(v2) {
			return c.runtimeError(errors.ErrTypeMismatch).Context(data.TypeOf(v1).String() + ", " + data.TypeOf(v2).String())
		}
	}

	v1, v2, err = data.Normalize(v1, v2)
	if err != nil {
		return c.runtimeError(err)
	}

	switch vx := v1.(type) {
	case error:
		return c.push(vx.Error() + data.String(v2))

		// All other types are scalar math.
	default:
		v1, v2, err = data.Normalize(v1, v2)
		if err != nil {
			return c.runtimeError(err)
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
			return c.runtimeError(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
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
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	x1, err := data.Bool(v1)
	if err != nil {
		return c.runtimeError(err)
	}

	x2, err := data.Bool(v2)
	if err != nil {
		return c.runtimeError(err)
	}

	return c.push(x1 && x2)
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
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	x1, err := data.Bool(v1)
	if err != nil {
		return c.runtimeError(err)
	}

	x2, err := data.Bool(v2)
	if err != nil {
		return c.runtimeError(err)
	}

	return c.push(x1 || x2)
}

// subtractByteCode instruction processor removes two items from the
// stack and subtracts them. For numeric values, this is arithmetic
// subtraction. For an array, the item to be subtracted is removed
// from the array (in any array location it is found).
func subtractByteCode(c *Context, i interface{}) error {
	var coerceOk bool

	v2, err := c.PopWithoutUnwrapping()
	if err != nil {
		return err
	}

	v1, err := c.PopWithoutUnwrapping()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	if c, ok := v1.(data.Immutable); ok {
		v1 = c.Value
		coerceOk = true
	}

	if c, ok := v2.(data.Immutable); ok {
		v2 = c.Value
		coerceOk = true
	}

	// Some special cases. If v1 is an array, then we are being
	// asked to delete an element from the array. This is only
	// done when language extensions are enabled.
	if a, ok := v1.(*data.Array); ok && c.extensions {
		for n := 0; n < a.Len(); n = n + 1 {
			v, _ := a.Get(n)
			x1 := v
			x2 := v2

			// If we don't require strict types, see if we can coerce
			// the types together.
			if c.typeStrictness != defs.StrictTypeEnforcement {
				x1, x2, err = data.Normalize(v, v2)
				if err != nil {
					return c.runtimeError(err)
				}
			}

			if data.Equals(x1, x2) {
				_ = a.Delete(n)
			}
		}

		return c.push(a)
	}

	if !coerceOk && c.typeStrictness == defs.StrictTypeEnforcement {
		if data.KindOf(v1) != data.KindOf(v2) {
			return c.runtimeError(errors.ErrTypeMismatch).Context(data.TypeOf(v1).String() + ", " + data.TypeOf(v2).String())
		}
	}

	v1, v2, err = data.Normalize(v1, v2)
	if err != nil {
		return c.runtimeError(err)
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
		return c.runtimeError(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
	}
}

// multiplyByteCode bytecode instruction processor.
func multiplyByteCode(c *Context, i interface{}) error {
	var coerceOk bool

	v2, err := c.PopWithoutUnwrapping()
	if err != nil {
		return err
	}

	v1, err := c.PopWithoutUnwrapping()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	if c, ok := v1.(data.Immutable); ok {
		v1 = c.Value
		coerceOk = true
	}

	if c, ok := v2.(data.Immutable); ok {
		v2 = c.Value
		coerceOk = true
	}

	// Special case of multiply of string by integer to repeat string
	if (data.KindOf(v1) == data.StringKind) &&
		data.IsNumeric(v2) {
		str := data.String(v1)

		count, err := data.Int(v2)
		if err != nil {
			return c.runtimeError(err)
		}

		r := strings.Repeat(str, count)

		return c.push(r)
	}

	// Nope, plain old math multiply, so normalize the values.
	if !coerceOk && c.typeStrictness == defs.StrictTypeEnforcement {
		if data.KindOf(v1) != data.KindOf(v2) {
			return c.runtimeError(errors.ErrTypeMismatch).Context(data.TypeOf(v1).String() + ", " + data.TypeOf(v2).String())
		}
	}

	v1, v2, err = data.Normalize(v1, v2)
	if err != nil {
		return c.runtimeError(err)
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
		return c.runtimeError(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
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

	if c, ok := v1.(data.Immutable); ok {
		v1 = c.Value
	}

	if c, ok := v2.(data.Immutable); ok {
		v2 = c.Value
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	switch v1.(type) {
	case byte, int32, int, int64:
		vv1, err := data.Int64(v1)
		if err != nil {
			return c.runtimeError(err)
		}

		vv2, err := data.Int64(v2)
		if err != nil {
			return c.runtimeError(err)
		}

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
		return c.runtimeError(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
	}
}

// divideByteCode bytecode instruction processor.
func divideByteCode(c *Context, i interface{}) error {
	var coerceOk bool

	if c.stackPointer < 1 {
		return c.runtimeError(errors.ErrStackUnderflow)
	}

	v2, err := c.PopWithoutUnwrapping()
	if err != nil {
		return err
	}

	v1, err := c.PopWithoutUnwrapping()
	if err != nil {
		return err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	if c, ok := v1.(data.Immutable); ok {
		v1 = c.Value
		coerceOk = true
	}

	if c, ok := v2.(data.Immutable); ok {
		v2 = c.Value
		coerceOk = true
	}

	if !coerceOk && c.typeStrictness == defs.StrictTypeEnforcement {
		if data.KindOf(v1) != data.KindOf(v2) {
			return c.runtimeError(errors.ErrTypeMismatch).Context(data.TypeOf(v1).String() + ", " + data.TypeOf(v2).String())
		}
	}

	v1, v2, err = data.Normalize(v1, v2)
	if err != nil {
		return c.runtimeError(err)
	}

	switch v1.(type) {
	case byte:
		if v2.(byte) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(byte) / v2.(byte))

	case int32:
		if v2.(int32) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int32) / v2.(int32))

	case int:
		if v2.(int) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int) / v2.(int))

	case int64:
		if v2.(int64) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int64) / v2.(int64))

	case float32:
		if v2.(float32) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(float32) / v2.(float32))

	case float64:
		if v2.(float64) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(float64) / v2.(float64))

	default:
		return c.runtimeError(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
	}
}

// moduloByteCode bytecode instruction processor.
func moduloByteCode(c *Context, i interface{}) error {
	var coerceOk bool

	if c.stackPointer < 1 {
		return c.runtimeError(errors.ErrStackUnderflow)
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
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	if c, ok := v1.(data.Immutable); ok {
		v1 = c.Value
		coerceOk = true
	}

	if c, ok := v2.(data.Immutable); ok {
		v2 = c.Value
		coerceOk = true
	}

	if !coerceOk && c.typeStrictness == defs.StrictTypeEnforcement {
		if data.KindOf(v1) != data.KindOf(v2) {
			return c.runtimeError(errors.ErrTypeMismatch).Context(data.TypeOf(v1).String() + ", " + data.TypeOf(v2).String())
		}
	}

	v1, v2, err = data.Normalize(v1, v2)
	if err != nil {
		return c.runtimeError(err)
	}

	switch v1.(type) {
	case byte:
		if v2.(byte) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(byte) % v2.(byte))

	case int32:
		if v2.(int32) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int32) % v2.(int32))

	case int:
		if v2.(int) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int) % v2.(int))

	case int64:
		if v2.(int64) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(int64) % v2.(int64))

	default:
		return c.runtimeError(errors.ErrInvalidType).Context(data.TypeOf(v1).String())
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
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	x1, err := data.Int(v1)
	if err != nil {
		return c.runtimeError(err)
	}

	x2, err := data.Int(v2)
	if err != nil {
		return c.runtimeError(err)
	}

	return c.push(x1 & x2)
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
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	x1, err := data.Int(v1)
	if err != nil {
		return c.runtimeError(err)
	}

	x2, err := data.Int(v2)
	if err != nil {
		return c.runtimeError(err)
	}

	return c.push(x1 | x2)
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
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	shift, err := data.Int(v1)
	if err != nil {
		return c.runtimeError(err)
	}

	value, err := data.Int(v2)
	if err != nil {
		return c.runtimeError(err)
	}

	if shift < -31 || shift > 31 {
		return c.runtimeError(errors.ErrInvalidBitShift).Context(shift)
	}

	if shift < 0 {
		value = value << -shift
	} else {
		value = value >> shift
	}

	return c.push(value)
}
