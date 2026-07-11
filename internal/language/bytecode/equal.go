package bytecode

import (
	"reflect"
	"time"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
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
//
// EQUAL-4 (BUG-13 fix): *data.Type values are only equal to other *data.Type
// values.  Before this fix, equalTypes() also accepted a string on the right-
// hand side and compared it against the canonical type name, so typeof(n)=="int"
// returned true.  This "cheat" predates the Ego type system and is now removed.
// The type switch below now also handles the reverse ordering (type on v2, non-
// type on v1) which previously fell through to genericEqualCompare and produced
// a confusing "invalid integer value" error (seen in switch-case bodies where
// the compiler pushes the case value before loading the switch expression).
func equalByteCode(c *Context, i any) error {
	// Get the two terms to compare. These are found either in the operand as an
	// array of values or on the stack.
	v1, v2, err := getComparisonTerms(c, i)
	if err != nil {
		return err
	}

	// A named scalar type decays to its underlying value for comparison, the
	// same way it does for arithmetic (data.Coerce). Without this, a *Scalar
	// is a Go pointer and would incorrectly fall into the native-pointer
	// identity comparison in the default case below (isPointerValue).
	v1 = unwrapScalar(v1)
	v2 = unwrapScalar(v2)

	// If both are nil, then they match.
	if data.IsNil(v1) && data.IsNil(v2) {
		return c.push(true)
	}

	// If exactly one side is nil there is no match.  After this guard, v1 is
	// guaranteed to be non-nil for the rest of the function.
	if data.IsNil(v1) || data.IsNil(v2) {
		return c.push(false)
	}

	// EQUAL-4: If v2 is a *data.Type and v1 is not, treat it symmetrically with
	// the case *data.Type branch below.  This covers the switch-case direction
	// where the compiler pushes the case expression (e.g. the bare type name
	// "int" or the type identifier int) before loading the switch expression
	// (typeof(n)), meaning v2 carries the type and v1 carries the case value.
	// Without this guard, control would fall to genericEqualCompare → Normalize,
	// which tries to Coerce the case value to the type's instance kind and can
	// produce "invalid integer value: int" for string case values.
	if t2, ok := v2.(*data.Type); ok {
		if _, v1IsType := v1.(*data.Type); !v1IsType {
			// v1 is not a type; a type is never equal to a non-type value.
			return c.push(false)
		}

		// Both are types: delegate to equalTypes with the roles canonicalized
		// so actual (v1) is always the left-hand type.
		return equalTypes(v1, c, t2)
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
		// fix BUG-34: a native Go pointer value -- e.g. *int, *string,
		// *float64 (produced by data.AddressOf when Ego code takes the
		// address of a scalar variable with "&x"), *any (AddressOf's
		// fallback for interface{} values), or **Map/**Array/**Channel
		// (the double-pointer form AddressOf returns when "&" is applied to
		// a variable that already holds one of Ego's own reference types) --
		// has no case of its own above, so it used to fall straight through
		// to genericEqualCompare. That function's inner switch has no branch
		// for a pointer's Go type either, so `result` was left at its zero
		// value (false) no matter what -- meaning "pa == pb" was always
		// false, even when pa and pb pointed at the exact same variable.
		//
		// isPointerValue reports whether a value is one of these native Go
		// pointers (see its doc comment for why Ego's own *data.Struct,
		// *data.Map, *data.Array, and *data.Type are excluded -- they are
		// already handled, correctly, by the cases above).
		if isPointerValue(v1) || isPointerValue(v2) {
			if isPointerValue(v1) && isPointerValue(v2) {
				// Two pointers are equal only when they share the exact same
				// concrete Go type (e.g. both *int) AND the same address.
				// "v1 == v2" is safe here and can never panic: both operands
				// are guaranteed to be Go pointer values, which are always
				// comparable, and Go's interface-equality rules already
				// define comparisons between two differently-typed pointers
				// as simply "not equal" (no panic) rather than an error.
				return c.push(v1 == v2)
			}

			// A pointer is never equal to a non-pointer value (e.g. pa == 5).
			return c.push(false)
		}

		return genericEqualCompare(c, v1, v2)
	}

	return c.push(result)
}

// isPointerValue reports whether v is a native Go pointer -- e.g. *int,
// *string, *float64 (as produced by data.AddressOf when Ego code evaluates
// "&x" for a scalar variable x), *any (AddressOf's fallback representation
// for interface{} values), or **Map / **Array / **Channel (the
// pointer-to-pointer form AddressOf returns when "&" is applied to a
// variable that already holds one of Ego's own reference types).
//
// It is used by equalByteCode and notEqualByteCode (fix for BUG-34) to
// detect a pointer comparison so it can be resolved by address identity
// instead of silently falling through to comparison logic that has no
// notion of a pointer at all and always reported "not equal to anything".
//
// Ego's own composite value types -- *data.Struct, *data.Map, *data.Array,
// *data.Type -- are technically Go pointers too, but callers never see them
// reach this function in practice: the type switches in equalByteCode and
// notEqualByteCode each have a dedicated, earlier case for those types that
// compares structural (deep) equality instead of address identity, which is
// the correct semantics for them (see BUG-26).
// unwrapScalar decays a named scalar type value (e.g. "type buzz int32") to
// its underlying value for comparison purposes, mirroring data.Coerce's
// decay-on-operation semantics for arithmetic. Comparison opcodes need their
// own explicit unwrap because several of them (equalByteCode, notEqualByteCode,
// and the strict-mode branch of the ordered comparisons) inspect the operand's
// concrete Go type directly instead of always routing through data.Coerce/
// data.Normalize, and a *data.Scalar is itself a Go pointer that would
// otherwise be misidentified as a native pointer value or fail to match any
// concrete-type case at all.
func unwrapScalar(v any) any {
	if sv, ok := v.(*data.Scalar); ok {
		return sv.Value()
	}

	return v
}

func isPointerValue(v any) bool {
	if v == nil {
		return false
	}

	t := reflect.TypeOf(v)

	return t != nil && t.Kind() == reflect.Ptr
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
		// Otherwise, normalize the types to the same type. The comparison
		// result is always a bool, so the promotion direction never affects
		// correctness here; constant-ness is irrelevant to this call.
		v1, v2, err = data.Normalize(v1, false, v2, false, false)
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

// equalTypes compares two *data.Type values, pushing a boolean result onto the
// stack.
//
// Deep-equal cannot be used on type objects because their internal pointers
// differ even when the types are semantically identical.  String comparison of
// the canonical type name is used instead.
//
// v2 must be a *data.Type.  Any other value causes the function to push false,
// because a type value is never equal to a non-type value.
//
// EQUAL-1: The original code returned errors.ErrNotAType.Context(v2) directly
// for non-type v2, bypassing c.runtimeError.  The error decoration is now moot
// because non-type v2 is handled by pushing false (see EQUAL-4 below).
//
// EQUAL-4 (BUG-13 fix): The original code also accepted a string v2 and
// compared it to actual.String(), so typeof(n)=="int" returned true.  This
// "cheat" predated the Ego type system.  It is now removed: a type value is
// only equal to another type value with the same canonical name.  Use the type
// identifier directly — typeof(n) == int — rather than a string literal.
func equalTypes(v2 any, c *Context, actual *data.Type) error {
	if v, ok := v2.(*data.Type); ok {
		return c.push(actual.String() == v.String())
	}

	// v2 is not a *data.Type.  A type is never equal to a non-type value,
	// including strings that happen to spell out the type name.
	return c.push(false)
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
