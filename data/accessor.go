package data

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tucats/ego/defs"
)

// String retrieves the string value of the argument, converting the
// underlying value if needed.
func String(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

// Byte retrieves the byte value of the argument, converting the
// underlying value if needed.
func Byte(v interface{}) byte {
	i := Int(v)

	return byte(i & math.MaxInt8)
}

// Int32 retrieves the int32 value of the argument, converting the
// underlying value if needed.
func Int32(v interface{}) int32 {
	i := Int(v)

	return int32(i & math.MaxInt32)
}

// Int retrieves the int value of the argument, converting the
// underlying value if needed.
func Int(v interface{}) int {
	result := 0

	switch actual := v.(type) {
	case bool:
		if actual {
			result = 1
		}

	case byte:
		result = int(actual)

	case int32:
		result = int(actual)

	case int:
		result = actual

	case int64:
		result = int(actual)

	case float32:
		result = int(actual)

	case float64:
		result = int(actual)

	case string:
		result, _ = strconv.Atoi(actual)
	}

	return result
}

// Int64 retrieves the int64 value of the argument, converting the
// underlying value if needed.
func Int64(v interface{}) int64 {
	var result int64

	switch actual := v.(type) {
	case bool:
		if actual {
			result = int64(1)
		}

	case byte:
		result = int64(actual)

	case int32:
		result = int64(actual)

	case int:
		result = int64(actual)

	case int64:
		result = actual

	case float32:
		result = int64(actual)

	case float64:
		result = int64(actual)

	case string:
		fmt.Sscanf(actual, "%d", &result)
	}

	return result
}

// Float64 retrieves the float64 value of the argument, converting the
// underlying value if needed.
func Float64(v interface{}) float64 {
	var result float64

	switch actual := v.(type) {
	case bool:
		if actual {
			result = 1.0
		}

	case int32:
		result = float64(actual)

	case int:
		result = float64(actual)

	case int64:
		result = float64(actual)

	case float32:
		result = float64(actual)

	case float64:
		result = actual

	case string:
		result, _ = strconv.ParseFloat(actual, 64)
	}

	return result
}

// Float32 retrieves the float32 value of the argument, converting the
// underlying value if needed.
func Float32(v interface{}) float32 {
	f := Float64(v)

	return float32(f)
}

// GetString retrieves the boolean value of the argument, converting the
// underlying value if needed.
func Bool(v interface{}) bool {
	switch actual := v.(type) {
	case byte, int32, int, int64:
		return Int64(v) != 0

	case float64, float32:
		return Float64(v) != 0.0

	case bool:
		return actual

	case string:
		for _, str := range []string{defs.True, "yes", "1", "y", "t"} {
			if strings.EqualFold(actual, str) {
				return true
			}
		}
	}

	return false
}

// DeepCopy creates a new copy of the interface. This includes recursively copying
// any member elements of arrays, maps, or structures. This cannot be used on a
// pointer value.
func DeepCopy(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch actual := v.(type) {
	case bool:
		return actual

	case byte:
		return actual

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
		result.fields = map[string]interface{}{}

		for k, v := range actual.fields {
			result.fields[k] = DeepCopy(v)
		}

		return result

	default:
		return nil // Unsupported type, (for example, pointers)
	}
}
