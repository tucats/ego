package bytecode

import (
	"reflect"
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
)

// notEqualByteCode implements the NotEqual opcode
//
// Inputs:
//
//	stack+0    - The item to be compared
//	stack+1    - The item to compare to
//
// The top two values are popped from the stack,
// and a type-specific test for equality is done.
// If the values are not equal, then true is pushed
// back on the stack, else false.
//
// COMPARE-4 (BUG-13 fix): *data.Type values must be handled explicitly here
// to match the equalByteCode behaviour. Without dedicated cases, a type value
// on either side would fall through to the default branch → Normalize →
// Coerce, producing "invalid integer value: int" for string operands.  The
// same EQUAL-4 logic is mirrored: type != type compares canonical names;
// type != non-type is always true (they can never be equal).
func notEqualByteCode(c *Context, i any) error {
	var err error
	// Get the two terms to compare. These are found either in the operand as an
	// array of values or on the stack.
	v1, v2, err := getComparisonTerms(c, i)
	if err != nil {
		return c.runtimeError(err)
	}

	// A named scalar type decays to its underlying value for comparison, the
	// same way it does for arithmetic (data.Coerce). Without this, a *Scalar
	// is a Go pointer and would incorrectly fall into the native-pointer
	// identity comparison in the default case below (isPointerValue).
	v1 = unwrapScalar(v1)
	v2 = unwrapScalar(v2)

	// If only one side is nil, they are not equal by definition.
	if !data.IsNil(v1) && data.IsNil(v2) ||
		data.IsNil(v1) && !data.IsNil(v2) {
		return c.push(true)
	}

	// COMPARE-4: Handle the case where v2 is a *data.Type and v1 is not (the
	// switch-case direction where the compiler emits the case value first).  A
	// non-type value is never equal to a type value, so != is always true.
	if t2, ok := v2.(*data.Type); ok {
		if t1, v1IsType := v1.(*data.Type); !v1IsType {
			// v1 is not a type; types and non-types are always unequal.
			return c.push(true)
		} else {
			// Both are types: compare canonical names.
			return c.push(t1.String() != t2.String())
		}
	}

	var result bool

	switch actual := v1.(type) {
	case time.Duration:
		if d, ok := v2.(time.Duration); ok {
			result = (actual != d)
		} else {
			return c.runtimeError(errors.ErrInvalidTypeForOperation)
		}

	case time.Time:
		if d, ok := v2.(time.Time); ok {
			result = !(actual.Equal(d))
		} else {
			return c.runtimeError(errors.ErrInvalidTypeForOperation)
		}

	case nil:
		result = (v2 != nil)

	// COMPARE-4: A type value is only equal to another type with the same
	// canonical name; it is unequal to all non-type values.
	case *data.Type:
		if other, ok := v2.(*data.Type); ok {
			result = actual.String() != other.String()
		} else {
			result = true
		}

	case *errors.Error:
		result = !actual.Equal(v2)

	case error:
		result = !reflect.DeepEqual(v1, v2)

	// COMPARE-2 fix: the original cases used value types (data.Map, data.Array,
	// data.Struct) which can never match because Ego always stores these as
	// pointers (*data.Map, *data.Array, *data.Struct).  The value-type cases
	// caused every composite != comparison to silently return false (equal).
	// The pointer-type cases below mirror the pattern used in equalByteCode.
	case *data.Map:
		result = !reflect.DeepEqual(v1, v2)

	case *data.Array:
		if array, ok := v2.(*data.Array); ok {
			result = !actual.DeepEqual(array)
		} else {
			result = true // different types are always not equal
		}

	case *data.Struct:
		str, ok := v2.(*data.Struct)
		if !ok {
			result = true // different types are always not equal
		} else {
			result = !reflect.DeepEqual(actual, str)
		}

	default:
		// fix BUG-34: mirror the pointer-identity handling added to
		// equalByteCode's default case -- see the comment there for the
		// full rationale. A native Go pointer value (e.g. *int, *string --
		// from data.AddressOf, or **Map/**Array/**Channel) has no case of
		// its own above, so without this branch execution falls straight
		// into the strict/normalize logic below, which has no notion of a
		// pointer and left `result` at its zero value (false) -- meaning
		// "pa != pb" was always false, even for two pointers to two
		// different variables.
		if isPointerValue(v1) || isPointerValue(v2) {
			if isPointerValue(v1) && isPointerValue(v2) {
				// See equalByteCode's default case for why "v1 != v2" here
				// can never panic.
				result = (v1 != v2)
			} else {
				// A pointer is never equal to a non-pointer value (e.g.
				// pa != 5), so they are always "not equal".
				result = true
			}

			break
		}

		// If type checking is set to strict, the types must match exactly.
		if c.typeStrictness == defs.StrictTypeEnforcement {
			if !data.TypeOf(v1).IsType(data.TypeOf(v2)) {
				return c.runtimeError(errors.ErrTypeMismatch).
					Context(data.TypeOf(v2).String() + ", " + data.TypeOf(v1).String())
			}
		} else {
			// Otherwise, normalize the types to the same type. The comparison
			// result is always a bool, so the promotion direction never
			// affects correctness here; constant-ness is irrelevant.
			v1, v2, err = data.Normalize(v1, false, v2, false, false)
			if err != nil {
				return c.runtimeError(err)
			}
		}

		// Based on the now-normalized types, do the comparison.
		switch v1.(type) {
		case nil:
			result = false

		case uint16, uint32, uint, uint64:
			x1, err := data.UInt64(v1)
			if err != nil {
				return c.runtimeError(err)
			}

			x2, err := data.UInt64(v2)
			if err != nil {
				return c.runtimeError(err)
			}

			result = (x1 != x2)

		case byte, int32, int8, int16, int, int64:
			x1, err := data.Int64(v1)
			if err != nil {
				return c.runtimeError(err)
			}

			x2, err := data.Int64(v2)
			if err != nil {
				return c.runtimeError(err)
			}

			result = (x1 != x2)

		case float32:
			result = v1.(float32) != v2.(float32)

		case float64:
			result = v1.(float64) != v2.(float64)

		case complex64:
			result = v1.(complex64) != v2.(complex64)

		case complex128:
			result = v1.(complex128) != v2.(complex128)

		case string:
			result = v1.(string) != v2.(string)

		case bool:
			result = v1.(bool) != v2.(bool)
		}
	}

	_ = c.push(result)

	return nil
}
