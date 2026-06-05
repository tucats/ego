package bytecode

import (
	"reflect"
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

// equalByteCode implements the Equal opcode
//
// Inputs:
//
//	stack+0    - The item to be compared
//	stack+1    - The item to compare to
//
// The top two values are popped from the stack and a type-specific equality
// test is performed.  The boolean result is pushed back onto the stack.
//
// Nil handling is resolved by two guards before the type switch runs:
//   - both nil  → push true
//   - one nil   → push false
//
// These two guards also mean that by the time the type switch executes, v1
// is guaranteed to be non-nil.  There is therefore no `case nil:` branch in
// the switch — such a branch would be unreachable dead code (EQUAL-3).
func equalByteCode(c *Context, i any) error {
	// Get the two terms to compare. These are found either in the operand as an
	// array of values or on the stack.
	v1, v2, err := getComparisonTerms(c, i)
	if err != nil {
		return err
	}

	// If both are nil, then they match.
	if data.IsNil(v1) && data.IsNil(v2) {
		return c.push(true)
	}

	// If exactly one side is nil there is no match.  After this guard, v1 is
	// guaranteed to be non-nil for the rest of the function.
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.push(false)
	}

	var result bool

	switch actual := v1.(type) {
	case time.Duration:
		if d, ok := v2.(time.Duration); ok {
			result = (actual == d)
		} else {
			return c.runtimeError(errors.ErrInvalidTypeForOperation)
		}

	case time.Time:
		if d, ok := v2.(time.Time); ok {
			result = (actual.Equal(d))
		} else {
			return c.runtimeError(errors.ErrInvalidTypeForOperation)
		}

	case *data.Type:
		return equalTypes(v2, c, actual)

	case *errors.Error:
		result = actual.Equal(v2)

	case *data.Struct:
		str, ok := v2.(*data.Struct)
		if ok {
			result = reflect.DeepEqual(actual, str)
		} else {
			result = false
		}

	case *data.Map:
		result = reflect.DeepEqual(v1, v2)

	case *data.Array:
		if array, ok := v2.(*data.Array); ok {
			result = actual.DeepEqual(array)
		} else {
			result = false
		}

	default:
		return genericEqualCompare(c, v1, v2)
	}

	return c.push(result)
}

func genericEqualCompare(c *Context, v1 any, v2 any) error {
	var (
		err    error
		result bool
	)

	// If type checking is set to strict, the types must match exactly.
	if c.typeStrictness == defs.StrictTypeEnforcement {
		if !data.TypeOf(v1).IsType(data.TypeOf(v2)) {
			return c.runtimeError(errors.ErrTypeMismatch).
				Context(data.TypeOf(v2).String() + ", " + data.TypeOf(v1).String())
		}
	} else {
		// Otherwise, normalize the types to the same type.
		v1, v2, err = data.Normalize(v1, v2)
		if err != nil {
			return err
		}
	}

	if v1 == nil && v2 == nil {
		result = true
	} else {
		// Based on the now-normalized types, do the comparison.
		switch v1.(type) {
		case nil:
			result = false

		case byte, int8, int16, int32, int, int64:
			x1, err := data.Int64(v1)
			if err != nil {
				return err
			}

			x2, err := data.Int64(v2)
			if err != nil {
				return err
			}

			result = x1 == x2

		case uint16, uint32, uint, uint64:
			x1, err := data.UInt64(v1)
			if err != nil {
				return err
			}

			x2, err := data.UInt64(v2)
			if err != nil {
				return err
			}

			result = x1 == x2

		case float64:
			result = v1.(float64) == v2.(float64)

		case float32:
			result = v1.(float32) == v2.(float32)

		case string:
			result = v1.(string) == v2.(string)

		case bool:
			result = v1.(bool) == v2.(bool)
		}
	}

	return c.push(result)
}

// equalTypes compares a *data.Type (v1) against either another *data.Type or a
// string that names a type, pushing a boolean result onto the stack.
//
// Deep-equal cannot be used on type objects because their internal pointers
// differ even when the types are semantically identical.  String comparison of
// the canonical type name is used instead.
//
// v2 may be:
//   - a string      → compare against actual.String()
//   - a *data.Type  → compare actual.String() against v.String()
//   - anything else → return ErrNotAType via c.runtimeError (EQUAL-1 fix)
//
// EQUAL-1: The original code returned errors.ErrNotAType.Context(v2) directly,
// bypassing c.runtimeError.  That left the error without the module name or
// source-line annotation that every other runtime error in this package carries.
// Fixed: use c.runtimeError so the error is consistently decorated.
func equalTypes(v2 any, c *Context, actual *data.Type) error {
	if v, ok := v2.(string); ok {
		return c.push(actual.String() == v)
	} else if v, ok := v2.(*data.Type); ok {
		return c.push(actual.String() == v.String())
	}

	// v2 is not a type descriptor or a type-name string.  Return an error
	// decorated with the current module/line so callers can locate the fault.
	return c.runtimeError(errors.ErrNotAType, v2)
}

// getComparisonTerms reads the two operands for any comparison instruction.
//
// # Operand sources
//
// The right-hand operand (v2) can come from two places:
//   - Stack mode (i == nil or i is not a []any{oneValue}): v2 is popped from
//     the execution stack, then v1 is popped.
//   - Operand mode (i is []any{oneValue}): v2 is read from the single element
//     of the instruction operand slice; v1 is still popped from the stack.
//     The compiler uses this mode when the right-hand side is a compile-time
//     constant that has been folded into the instruction itself.
//
// # Immutable (constant) unwrapping
//
// Values pushed by the Constant opcode are wrapped in data.Immutable{Value: v}
// to distinguish compile-time constants from ordinary stack values.  This
// function strips that wrapper before returning, setting v1Constant or
// v2Constant so the coercion step below knows which side was a constant.
//
// # Constant coercion
//
// When at least one operand was a compile-time constant and both operands are
// numeric, the lower-rank type is coerced to match the higher-rank type.  This
// allows comparisons like `myInt32 == 5` to work in strict mode: the literal 5
// is an int, but it gets coerced to int32 to match the variable.
//
// EQUAL-2: The original code returned data.Coerce's error directly without
// wrapping in c.runtimeError, leaving the error without module/line info.
// Fixed: any coerce failure is now passed through c.runtimeError so it carries
// the same location annotation as all other errors in this package.
// In practice data.Coerce never fails for two valid numeric values; the
// c.runtimeError wrap is purely defensive.
func getComparisonTerms(c *Context, i any) (any, any, error) {
	var (
		v1         any
		v2         any
		v1Constant bool
		v2Constant bool
	)

	// Determine where v2 comes from: the instruction operand or the stack.
	if array, ok := i.([]any); ok && len(array) == 1 {
		// Operand mode: the compiler folded v2 directly into the instruction.
		v2 = array[0]
		if constant, ok := v2.(data.Immutable); ok {
			v2 = constant.Value
			v2Constant = true
		}
	} else {
		// Stack mode: pop v2 from the top of the stack.
		var err error

		v2, err = c.PopWithoutUnwrapping()
		if err != nil {
			return nil, nil, err
		}

		if constant, ok := v2.(data.Immutable); ok {
			v2Constant = true
			v2 = constant.Value
		}
	}

	// v1 is always popped from the stack regardless of where v2 came from.
	v1, err := c.PopWithoutUnwrapping()
	if err != nil {
		return nil, nil, err
	}

	if constant, ok := v1.(data.Immutable); ok {
		v1Constant = true
		v1 = constant.Value
	}

	// A StackMarker in either operand position means a sub-expression returned
	// void (no value).  That is a runtime error, not a comparison.
	if isStackMarker(v1) || isStackMarker(v2) {
		return nil, nil, c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	// If either argument was a compile-time constant and both sides are numeric,
	// coerce the lower-rank type to the higher-rank type so that comparisons
	// such as `myFloat64 == 3` (where 3 is an int constant) work correctly in
	// all type-strictness modes.
	if (v2Constant || v1Constant) && data.IsNumeric(v1) && data.IsNumeric(v2) {
		k1 := data.KindOf(v1)
		k2 := data.KindOf(v2)

		var coerceErr error

		if k1 > k2 {
			v2, coerceErr = data.Coerce(v2, v1)
		} else {
			v1, coerceErr = data.Coerce(v1, v2)
		}

		// EQUAL-2 fix: decorate any coerce error with module/line info.
		// data.Coerce never fails for two valid numeric values in practice,
		// but wrapping here is consistent with all other error returns in
		// this package.
		if coerceErr != nil {
			return nil, nil, c.runtimeError(coerceErr)
		}
	}

	return v1, v2, nil
}
