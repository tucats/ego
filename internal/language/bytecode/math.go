package bytecode

import (
	"math"
	"strings"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
)

/******************************************\
*                                         *
*     M A T H   P R I M I T I V E S       *
*                                         *
\******************************************/

// incrementByteCode instruction processor accepts two arguments
// that are the name of a variable and a numeric value. The named
// variable is incremented by the value.
func incrementByteCode(c *Context, i any) error {
	var (
		err       error
		symbol    string
		increment any
	)

	if operands, ok := i.([]any); ok && len(operands) == 2 {
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

	case int16:
		return c.set(symbol, value+increment.(int16))

	case uint16:
		return c.set(symbol, value+increment.(uint16))

	case uint32:
		return c.set(symbol, value+increment.(uint32))

	case uint:
		return c.set(symbol, value+increment.(uint))

	case uint64:
		return c.set(symbol, value+increment.(uint64))

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
// If the argument is a boolean true, then this is a boolean
// NOT operations instead of a negation, which has narrower
// rules for how it must be processed.
func negateByteCode(c *Context, i any) error {
	b, err := data.Bool(i)
	if err != nil {
		return c.runtimeError(err)
	}

	if b {
		return notByteCode(c, nil)
	}

	v, err := c.PopWithoutUnwrapping()
	if err != nil {
		// MATH-11 fix: wrap with c.runtimeError so the error carries the
		// current module name and source-line position, consistent with every
		// other error return in the bytecode package.
		return c.runtimeError(err)
	}

	isConstant := false
	if c, ok := v.(data.Immutable); ok {
		isConstant = true
		v = c.Value
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
		value = -value
		if isConstant {
			return c.push(data.Constant(value))
		}

		return c.push(value)

	case int16:
		value = -value
		if isConstant {
			return c.push(data.Constant(value))
		}

		return c.push(value)

	case uint16:
		value = -value
		if isConstant {
			return c.push(data.Constant(value))
		}

		return c.push(value)

	case uint32:
		value = -value
		if isConstant {
			return c.push(data.Constant(value))
		}

		return c.push(value)

	case uint:
		value = -value
		if isConstant {
			return c.push(data.Constant(value))
		}

		return c.push(value)

	case uint64:
		value = -value
		if isConstant {
			return c.push(data.Constant(value))
		}

		return c.push(value)

	case int32:
		value = -value
		if isConstant {
			return c.push(data.Constant(value))
		}

		return c.push(value)

	case int:
		value = -value
		if isConstant {
			return c.push(data.Constant(value))
		}

		return c.push(value)

	case int64:
		value = -value
		if isConstant {
			return c.push(data.Constant(value))
		}

		return c.push(value)

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
func notByteCode(c *Context, i any) error {
	var (
		v   any
		err error
	)

	if i != nil {
		v = i
	} else {
		v, err = c.Pop()
		if err != nil {
			// MATH-11 fix: decorate the error with module/line info so error
			// messages identify where in the Ego program the underflow occurred.
			return c.runtimeError(err)
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

	// MATH-9 fix: the original single multi-type case gave the switch variable
	// 'value' type any.  Comparing any(int64(0)) == 0 is false because the
	// untyped constant 0 defaults to int in interface comparisons, so the
	// dynamic types int64 and int differ.  Splitting into individual cases
	// makes 'value' the concrete type for each case, so value == 0 uses the
	// correctly-typed zero literal and the comparison is exact.

	case byte:
		return c.push(value == 0)

	case int8:
		return c.push(value == 0)

	case int16:
		return c.push(value == 0)

	case uint16:
		return c.push(value == 0)

	case int32:
		return c.push(value == 0)

	case uint32:
		return c.push(value == 0)

	case int:
		return c.push(value == 0)

	case uint:
		return c.push(value == 0)

	case int64:
		return c.push(value == 0)

	case uint64:
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
// items and adds them together. For boolean values, this is an AND (&&)
// operation — note that multiplyByteCode performs OR (||) for booleans.
// MATH-10 fix: the previous comment incorrectly said "OR"; the implementation
// uses && (AND), which is what this comment now reflects.
// For numeric values, it is arithmetic addition. For strings or arrays, it
// concatenates the two items. For a struct, it merges the addend into the
// first struct.
func addByteCode(c *Context, i any) error {
	// Get the two values we will operate on. This includes a flag
	// indicating that one or both of the items are constants, so
	// type coercion is permitted even in strict mode.
	v1, v2, coerceOk, err := getDiadicValues(c)
	if err != nil {
		return c.runtimeError(err)
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

		case int8:
			return c.push(v1.(int8) + v2.(int8))

		case int16:
			return c.push(v1.(int16) + v2.(int16))

		case uint16:
			return c.push(v1.(uint16) + v2.(uint16))

		case uint32:
			return c.push(v1.(uint32) + v2.(uint32))

		case uint:
			return c.push(uint(v1.(uint32)) + uint(v2.(uint32)))

		case uint64:
			return c.push(v1.(uint64) + v2.(uint64))

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
func andByteCode(c *Context, i any) error {
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
func orByteCode(c *Context, i any) error {
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
func subtractByteCode(c *Context, i any) error {
	// Get the two values we will operate on. This includes a flag
	// indicating that one or both of the items are constants, so
	// type coercion is permitted even in strict mode.
	v1, v2, coerceOk, err := getDiadicValues(c)
	if err != nil {
		return c.runtimeError(err)
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

	case uint32:
		return c.push(v1.(uint32) - v2.(uint32))

	case uint:
		return c.push(uint(v1.(uint)) - uint(v2.(uint)))

	case uint64:
		return c.push(v1.(uint64) - v2.(uint64))

	case int16:
		return c.push(v1.(int16) - v2.(int16))

	case int8:
		// MATH-4 fix: original cast v1.(int16) panicked because after
		// data.Normalize two equal int8 values remain int8 (same kind is
		// unchanged).  The type assertion int16 on an int8 value fails.
		return c.push(v1.(int8) - v2.(int8))

	case uint16:
		return c.push(v1.(uint16) - v2.(uint16))

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
func multiplyByteCode(c *Context, i any) error {
	// Get the two values we will operate on. This includes a flag
	// indicating that one or both of the items are constants, so
	// type coercion is permitted even in strict mode.
	v1, v2, coerceOk, err := getDiadicValues(c)
	if err != nil {
		return c.runtimeError(err)
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

	case int8:
		return c.push(int8(v1.(int8)) * int8(v2.(int8)))

	case int16:
		// MATH-2 fix: original cast v1.(int8) panicked because v1 is int16
		// after data.Normalize leaves two equal-kind values unchanged.
		return c.push(v1.(int16) * v2.(int16))

	case uint16:
		// MATH-3 fix: original cast v1.(int8) panicked; v1 is uint16
		// after data.Normalize leaves two equal-kind uint16 values unchanged.
		return c.push(v1.(uint16) * v2.(uint16))

	case uint32:
		return c.push(v1.(uint32) * v2.(uint32))

	case uint:
		return c.push(uint(v1.(uint)) * uint(v2.(uint)))

	case uint64:
		return c.push(v1.(uint64) * v2.(uint64))

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
func exponentByteCode(c *Context, i any) error {
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
	case byte, int8, int16, int32, int, int64:
		vv1, err := data.Int64(v1)
		if err != nil {
			return c.runtimeError(err)
		}

		vv2, err := data.Int64(v2)
		if err != nil {
			return c.runtimeError(err)
		}

		if vv2 == 0 {
			// MATH-1 fix: x^0 == 1 for all non-zero bases.  The previous code
			// pushed the untyped literal 0 (which becomes int(0)), giving the
			// mathematically wrong result.  int64(1) matches the type that the
			// success path pushes after the multiplication loop.
			return c.push(int64(1))
		}

		if vv2 == 1 {
			return c.push(v1)
		}

		prod := vv1

		for n := int64(2); n <= vv2; n = n + 1 {
			prod = prod * vv1
		}

		return c.push(prod)

	case uint16, uint32, uint, uint64:
		vv1, err := data.UInt64(v1)
		if err != nil {
			return c.runtimeError(err)
		}

		vv2, err := data.UInt64(v2)
		if err != nil {
			return c.runtimeError(err)
		}

		if vv2 == 0 {
			return c.push(uint64(1))
		}

		if vv2 == 1 {
			return c.push(v1)
		}

		prod := vv1

		for n := uint64(2); n <= vv2; n = n + 1 {
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
func divideByteCode(c *Context, i any) error {
	// Get the two values we will operate on. This includes a flag
	// indicating that one or both of the items are constants, so
	// type coercion is permitted even in strict mode.
	v1, v2, coerceOk, err := getDiadicValues(c)
	if err != nil {
		return c.runtimeError(err)
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

	case uint16:
		if v2.(uint16) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		// MATH-6 fix: original cast v1.(int8) panicked; v1 is uint16.
		return c.push(v1.(uint16) / v2.(uint16))

	case int16:
		if v2.(int16) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		// MATH-5 fix: original cast v1.(int8) panicked; v1 is int16.
		return c.push(v1.(int16) / v2.(int16))

	case uint32:
		if v2.(uint32) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(uint32) / v2.(uint32))

	case uint:
		if v2.(uint) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(uint(v1.(uint)) / uint(v2.(uint)))

	case uint64:
		if v2.(uint64) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(uint64) / v2.(uint64))

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
func moduloByteCode(c *Context, i any) error {
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

	case int16:
		if v2.(int16) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		// MATH-7 fix: original cast v1.(int8) panicked; v1 is int16.
		return c.push(v1.(int16) % v2.(int16))

	case uint16:
		if v2.(uint16) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		// MATH-8 fix: original cast v1.(int8) panicked; v1 is uint16.
		return c.push(v1.(uint16) % v2.(uint16))

	case int8:
		if v2.(int8) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(int8(v1.(int8)) % int8(v2.(int8)))

	case uint32:
		if v2.(uint32) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(uint32) % v2.(uint32))

	case uint:
		if v2.(uint) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(uint(v1.(uint)) % uint(v2.(uint)))

	case uint64:
		if v2.(uint64) == 0 {
			return c.runtimeError(errors.ErrDivisionByZero)
		}

		return c.push(v1.(uint64) % v2.(uint64))

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

func bitAndByteCode(c *Context, i any) error {
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

func bitOrByteCode(c *Context, i any) error {
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

// bitShiftByteCode implements the << and >> operators. The shift direction is
// supplied by the compiler as the instruction operand `i`: a boolean `true`
// means shift left (<<) and `false` means shift right (>>). A nil operand is
// treated as a right shift, which keeps older unit tests (and any caller that
// passes nil) behaving as an ordinary >> operation.
//
// The two operands are popped from the stack:
//
//	v1 (top of stack) = the shift amount (how many bits to shift)
//	v2               = the value being shifted
//
// Because the direction now travels in the opcode rather than being encoded in
// the sign of the shift amount, `shift` here is always the user's literal
// value. That lets us reject a negative shift the way Go does, instead of
// silently reinterpreting it as a shift in the opposite direction. (BUG-47).
func bitShiftByteCode(c *Context, i any) error {
	// Decode the shift direction from the instruction operand. data.BoolOrFalse
	// returns false for a nil operand, so an absent operand defaults to a
	// right shift.
	shiftLeft := data.BoolOrFalse(i)

	// v1 is the top of the stack: the number of bits to shift by.
	v1, err := c.Pop()
	if err != nil {
		return err
	}

	// v2 is the value that will be shifted.
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

	value, err := data.Int64(v2)
	if err != nil {
		return c.runtimeError(err)
	}

	// Go treats a negative shift amount as a runtime error ("negative shift
	// amount"). Reproduce that here rather than silently flipping the shift
	// direction, which is exactly the divergence BUG-47 describes.
	if shift < 0 {
		return c.runtimeError(errors.ErrNegativeShift).Context(shift)
	}

	// Apply the shift in the direction requested by the compiler.
	//
	// The upper bound on the shift amount differs by direction, matching the
	// range Ego accepted before this fix. A left shift may move a bit all the
	// way out of a 64-bit value (shift of 64 yields 0), so left shifts allow up
	// to 64. A right shift only has 63 meaningful positions for a signed value,
	// so right shifts are capped at 63. Amounts past these bounds have no useful
	// result and return the existing ErrInvalidBitShift.
	if shiftLeft {
		if shift > 64 {
			return c.runtimeError(errors.ErrInvalidBitShift).Context(shift)
		}

		value = value << shift
	} else {
		if shift > 63 {
			return c.runtimeError(errors.ErrInvalidBitShift).Context(shift)
		}

		value = value >> shift
	}

	return c.push(value)
}

// Pop two values from the stack to be used for standard diadic operators
// (addition, subtraction, multiplication, etc). Also returns a flag if
// either is a constant, which would mean the caller is free to coerce the
// values even in strict type-checking mode. The error indicates either a
// stack underflow, or value like a stack marker or a nil value that cannot
// be used with a math operation.
func getDiadicValues(c *Context) (any, any, bool, error) {
	var coerceOk bool

	v2, err := c.PopWithoutUnwrapping()
	if err != nil {
		return nil, nil, false, err
	}

	v1, err := c.PopWithoutUnwrapping()
	if err != nil {
		return nil, nil, false, err
	}

	if isStackMarker(v1) || isStackMarker(v2) {
		return nil, nil, false, c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// Cannot do math on a nil value
	if data.IsNil(v1) || data.IsNil(v2) {
		return nil, nil, false, c.runtimeError(errors.ErrInvalidType).Context("nil")
	}

	if c, ok := v1.(data.Immutable); ok {
		v1 = c.Value
		coerceOk = true
	}

	if c, ok := v2.(data.Immutable); ok {
		v2 = c.Value
		coerceOk = true
	}

	return v1, v2, coerceOk, nil
}
