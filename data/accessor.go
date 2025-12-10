package data

import (
	"fmt"
	"time"

	"github.com/tucats/ego/app-cli/ui"
)

// Rune converts an arbitrary value to a rune. For numeric values, it is
// converted to a comparable integer value expressed as a rune. For a string
// the rune value is the first (possible escaped) character in the string.
func Rune(v any) rune {
	v = UnwrapConstant(v)

	switch actual := v.(type) {
	case byte:
		return rune(actual)
	case int32:
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
			if ch == '\'' && len(actual) == 3 {
				ch = actual[1]
			}

			return rune(ch)
		}
	}

	return 0
}

// String retrieves the string value of the argument, converting the
// underlying value if needed.
func String(v any) string {
	v = UnwrapConstant(v)

	if v == nil {
		return ""
	}

	// If it's a time, we convert using time.RFC822Z format.
	if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC822Z)
	}

	return fmt.Sprintf("%v", v)
}

// Byte retrieves the byte value of the argument, converting the
// underlying value if needed.
func Byte(v any) (byte, error) {
	v = UnwrapConstant(v)

	b, err := Coerce(v, ByteType)
	if err != nil {
		return 0, err
	}

	return b.(byte), nil
}

// Int32 retrieves the int32 value of the argument, converting the
// underlying value if needed.
func Int32(v any) (int32, error) {
	v = UnwrapConstant(v)

	b, err := Coerce(v, Int32Type)
	if err != nil {
		return 0, err
	}

	return b.(int32), nil
}

// UInt32 retrieves the uint32 value of the argument, converting the
// underlying value if needed.
func UInt32(v any) (uint32, error) {
	v = UnwrapConstant(v)

	b, err := Coerce(v, UInt32Type)
	if err != nil {
		return 0, err
	}

	return b.(uint32), nil
}

// Int retrieves the int value of the argument, converting the
// underlying value if needed.
func Int(v any) (int, error) {
	v = UnwrapConstant(v)

	b, err := Coerce(v, IntType)
	if err != nil {
		return 0, err
	}

	return b.(int), nil
}

// UInt32 retrieves the uint32 value of the argument, converting the
// underlying value if needed.
func UInt(v any) (uint, error) {
	v = UnwrapConstant(v)

	b, err := Coerce(v, UIntType)
	if err != nil {
		return 0, err
	}

	return b.(uint), nil
}

// Int64 retrieves the int64 value of the argument, converting the
// underlying value if needed.
func Int64(v any) (int64, error) {
	v = UnwrapConstant(v)

	b, err := Coerce(v, Int64Type)
	if err != nil {
		return 0, err
	}

	return b.(int64), nil
}

// UInt32 retrieves the uint32 value of the argument, converting the
// underlying value if needed.
func UInt64(v any) (uint64, error) {
	v = UnwrapConstant(v)

	b, err := Coerce(v, UInt64Type)
	if err != nil {
		return 0, err
	}

	return b.(uint64), nil
}

// Float64 retrieves the float64 value of the argument, converting the
// underlying value if needed.
func Float64(v any) (float64, error) {
	v = UnwrapConstant(v)

	b, err := Coerce(v, Float64Type)
	if err != nil {
		return 0, err
	}

	return b.(float64), nil
}

// Float32 retrieves the float32 value of the argument, converting the
// underlying value if needed.
func Float32(v any) (float32, error) {
	v = UnwrapConstant(v)

	b, err := Coerce(v, Float32Type)
	if err != nil {
		return 0, err
	}

	return b.(float32), nil
}

// Bool retrieves the boolean value of the argument, converting the
// underlying value if needed.
func Bool(v any) (bool, error) {
	v = UnwrapConstant(v)

	b, err := Coerce(v, BoolType)
	if err != nil {
		return false, err
	}

	return b.(bool), nil
}

func IntOrZero(v2 any) int {
	b, err := Int(v2)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.int", ui.A{
			"error": err})

		return 0
	}

	return b
}

func Int32OrZero(v2 any) int32 {
	b, err := Int32(v2)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.int32", ui.A{
			"error": err})

		return 0
	}

	return b
}

func Int64OrZero(v2 any) int64 {
	b, err := Int64(v2)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.int64", ui.A{
			"error": err})

		return 0
	}

	return b
}

func Float64OrZero(v2 any) float64 {
	b, err := Float64(v2)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.float64", ui.A{
			"error": err})

		return 0.0
	}

	return b
}

func Float32OrZero(v2 any) float32 {
	b, err := Float32(v2)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.int32", ui.A{
			"error": err})

		return 0.0
	}

	return b
}

func BoolOrFalse(v any) bool {
	b, err := Bool(v)
	if err != nil {
		ui.Log(ui.InternalLogger, "runtime.access.bool", ui.A{
			"error": err})
	}

	return b
}

// DeepCopy creates a new copy of the interface. This includes recursively copying
// any member elements of arrays, maps, or structures. This cannot be used on a
// pointer value.
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
		size := actual.Len()
		result := NewArray(actual.valueType, size)

		for i := 0; i < size; i++ {
			v, _ := actual.Get(i)
			_ = result.Set(i, DeepCopy(v))
		}

		return result

	case *Map:
		result := NewMap(actual.keyType, actual.elementType)
		keys := actual.Keys()

		for _, k := range keys {
			v, _, _ := actual.Get(k)
			_, _ = result.Set(k, DeepCopy(v))
		}

		return result

	case *Struct:
		result := actual.Copy()
		result.fields = map[string]any{}

		for k, v := range actual.fields {
			result.fields[k] = DeepCopy(v)
		}

		return result

	default:
		return v // Unsupported type, (for example, pointers). Hope for the best...
	}
}
