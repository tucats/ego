package data

import (
	"fmt"
	"time"

	"github.com/tucats/ego/app-cli/ui"
)

// Rune converts an arbitrary value to a rune (Go's name for a Unicode code
// point, which is just an alias for int32).
//
// For numeric types the conversion is a straight cast — the integer value
// becomes the Unicode code point.  For a string, the first byte is taken as
// the character code; if the string looks like a rune literal (e.g. "'A'") the
// character between the quotes is used instead.
func Rune(v any) rune {
	if v == nil {
		return rune(0)
	}

	// UnwrapConstant removes the Immutable wrapper if one is present so
	// the underlying value can be type-switched below.
	v = UnwrapConstant(v)

	switch actual := v.(type) {
	case byte:
		return rune(actual)
	case int16:
		return rune(actual)
	case uint16:
		return rune(actual)
	case int8:
		return rune(actual)
	case int32:
		// int32 and rune are the same underlying type in Go; no conversion needed.
		return actual
	case uint32:
		return rune(actual)
	case uint:
		return rune(actual)
	case uint64:
		return rune(actual)
	case int:
		return rune(actual)
	case int64:
		return rune(actual)
	case float32:
		return rune(actual)
	case float64:
		return rune(actual)
	case string:
		if len(actual) > 0 {
			ch := actual[0]
			// Handle the Ego rune-literal syntax: a three-character string
			// of the form "'x'" represents the single character x.
			if ch == '\'' && len(actual) == 3 {
				ch = actual[1]
			}

			return rune(ch)
		}
	}

	return 0
}

// String converts any value to its string representation.
//
// For nil, an empty string is returned.  For time.Time values, RFC 822 with
// numeric timezone is used because it is compact and unambiguous.  For
// everything else, fmt.Sprintf with the %v verb produces Go's default
// "value" representation — numbers as digits, booleans as "true"/"false",
// and so on.
func String(v any) string {
	if v == nil {
		return ""
	}

	v = UnwrapConstant(v)

	if v == nil {
		return ""
	}

	// time.RFC822Z produces a human-readable timestamp like "02 Jan 06 15:04 -0700".
	if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC822Z)
	}

	return fmt.Sprintf("%v", v)
}

// Byte converts any value to a byte (uint8) by delegating to Coerce.
// Returns 0 and a nil error for a nil input.
func Byte(v any) (byte, error) {
	if v == nil {
		return byte(0), nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, ByteType)
	if err != nil {
		return 0, err
	}

	return b.(byte), nil
}

// Int32 converts any value to int32 by delegating to Coerce.
func Int32(v any) (int32, error) {
	if v == nil {
		return int32(0), nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, Int32Type)
	if err != nil {
		return 0, err
	}

	return b.(int32), nil
}

// UInt32 converts any value to uint32 by delegating to Coerce.
func UInt32(v any) (uint32, error) {
	v = UnwrapConstant(v)

	b, err := Coerce(v, UInt32Type)
	if err != nil {
		return 0, err
	}

	return b.(uint32), nil
}

// Int8 converts any value to int8 by delegating to Coerce.
func Int8(v any) (int8, error) {
	if v == nil {
		return int8(0), nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, Int8Type)
	if err != nil {
		return 0, err
	}

	return b.(int8), nil
}

// Int converts any value to int (the platform-native integer size) by
// delegating to Coerce.
func Int(v any) (int, error) {
	if v == nil {
		return 0, nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, IntType)
	if err != nil {
		return 0, err
	}

	return b.(int), nil
}

// UInt converts any value to uint by delegating to Coerce.
func UInt(v any) (uint, error) {
	if v == nil {
		return 0, nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, UIntType)
	if err != nil {
		return 0, err
	}

	return b.(uint), nil
}

// Int16 converts any value to int16 by delegating to Coerce.
func Int16(v any) (int16, error) {
	if v == nil {
		return int16(0), nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, Int16Type)
	if err != nil {
		return 0, err
	}

	return b.(int16), nil
}

// UInt16 converts any value to uint16 by delegating to Coerce.
func UInt16(v any) (uint16, error) {
	if v == nil {
		return uint16(0), nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, UInt16Type)
	if err != nil {
		return 0, err
	}

	return b.(uint16), nil
}

// Int64 converts any value to int64 by delegating to Coerce.
func Int64(v any) (int64, error) {
	if v == nil {
		return int64(0), nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, Int64Type)
	if err != nil {
		return 0, err
	}

	return b.(int64), nil
}

// UInt64 converts any value to uint64 by delegating to Coerce.
func UInt64(v any) (uint64, error) {
	if v == nil {
		return uint64(0), nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, UInt64Type)
	if err != nil {
		return 0, err
	}

	return b.(uint64), nil
}

// Float64 converts any value to float64 by delegating to Coerce.
func Float64(v any) (float64, error) {
	if v == nil {
		return 0, nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, Float64Type)
	if err != nil {
		return 0, err
	}

	return b.(float64), nil
}

// Float32 converts any value to float32 by delegating to Coerce.
func Float32(v any) (float32, error) {
	if v == nil {
		return 0, nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, Float32Type)
	if err != nil {
		return 0, err
	}

	return b.(float32), nil
}

// Bool converts any value to bool by delegating to Coerce.
func Bool(v any) (bool, error) {
	if v == nil {
		return false, nil
	}

	v = UnwrapConstant(v)

	b, err := Coerce(v, BoolType)
	if err != nil {
		return false, err
	}

	return b.(bool), nil
}

// The "OrZero" functions below are convenience wrappers that suppress the
// error return from their corresponding typed accessor.  They are used in
// contexts where the caller knows the value should be convertible (e.g.
// reading a well-typed field from a struct) and an error is unexpected.
// When an error does occur it is logged to the internal diagnostic logger
// and the zero value for the target type is returned, preventing a panic.

// IntOrZero converts v to int, returning 0 on error.
func IntOrZero(v2 any) int {
	b, err := Int(v2)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.int", ui.A{
			"error": err})

		return 0
	}

	return b
}

// Int32OrZero converts v to int32, returning 0 on error.
func Int32OrZero(v2 any) int32 {
	b, err := Int32(v2)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.int32", ui.A{
			"error": err})

		return 0
	}

	return b
}

// Int64OrZero converts v to int64, returning 0 on error.
func Int64OrZero(v2 any) int64 {
	b, err := Int64(v2)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.int64", ui.A{
			"error": err})

		return 0
	}

	return b
}

// Float64OrZero converts v to float64, returning 0.0 on error.
func Float64OrZero(v2 any) float64 {
	b, err := Float64(v2)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.float64", ui.A{
			"error": err})

		return 0.0
	}

	return b
}

// Float32OrZero converts v to float32, returning 0.0 on error.
func Float32OrZero(v2 any) float32 {
	b, err := Float32(v2)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.int32", ui.A{
			"error": err})

		return 0.0
	}

	return b
}

// BoolOrFalse converts v to bool, returning false on error.
func BoolOrFalse(v any) bool {
	b, err := Bool(v)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.bool", ui.A{
			"error": err})
	}

	return b
}

// DeepCopy creates an entirely independent copy of the given value. Every
// nested array, map, and struct is recursively copied so that modifying the
// copy cannot affect the original, and vice versa.
//
// For scalar types (bool, int, float64, string, etc.) Go already copies
// values on assignment, so returning "actual" is sufficient.
//
// For Ego's own reference types (*Array, *Map, *Struct) we create new
// objects and copy every element/key/field recursively.
//
// The default case is a safety net for pointer types and other complex values
// that this function does not know how to deep-copy.  Returning v unchanged
// means the "copy" shares storage with the original — callers should avoid
// mutating pointer values obtained through DeepCopy.
func DeepCopy(v any) any {
	if v == nil {
		return nil
	}

	switch actual := v.(type) {
	case bool:
		return actual

	case byte:
		return actual

	case uint32:
		return actual

	case uint:
		return uint(actual)

	case uint64:
		return uint64(actual)

	case int32:
		return actual

	case int:
		return actual

	case int64:
		return actual

	case float32:
		return actual

	case float64:
		return actual

	case string:
		return actual

	case *Array:
		// Allocate a new array of the same type and size, then copy each
		// element, recursing into any nested arrays/maps/structs.
		size := actual.Len()
		result := NewArray(actual.valueType, size)

		for i := 0; i < size; i++ {
			v, _ := actual.Get(i)
			_ = result.Set(i, DeepCopy(v))
		}

		return result

	case *Map:
		// Allocate a new map with the same key/value types, then copy each
		// entry recursively.
		result := NewMap(actual.keyType, actual.elementType)
		keys := actual.Keys()

		for _, k := range keys {
			v, _, _ := actual.Get(k)
			_, _ = result.Set(k, DeepCopy(v))
		}

		return result

	case *Struct:
		// Copy() duplicates the struct metadata (type, flags, etc.); we then
		// replace the fields map with a fresh copy so no field is shared.
		result := actual.Copy()
		result.fields = map[string]any{}

		for k, v := range actual.fields {
			result.fields[k] = DeepCopy(v)
		}

		return result

	default:
		// Unsupported type (e.g. a pointer or a native Go struct).  Return
		// the original value as-is — the caller accepts shared storage.
		return v
	}
}
