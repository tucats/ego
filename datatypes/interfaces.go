package datatypes

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tucats/ego/defs"
)

// GetType returns the Type of a given interface object. If the value is a type
// or a pointer to a type, return that Type. If the interface isn't a type, but
// is a scalar value, return the Type of the value.
func GetType(v interface{}) *Type {
	if t, ok := v.(Type); ok {
		if t.valueType != nil && t.valueType.kind == InterfaceKind {
			t = *t.valueType
		}

		return &t
	}

	if t, ok := v.(*Type); ok {
		return t
	}

	// If it's a value object, return it's type.
	switch v.(type) {
	case bool:
		return BoolType

	case int32:
		return &Int32Type

	case int:
		return &IntType

	case int64:
		return &Int64Type

	case float32:
		return &Float32Type

	case float64:
		return &Float64Type

	case string:
		return &StringType
	}

	// Who knows what the heck this is...
	return &UndefinedType
}

// GetString retrieves the string value of the argument, converting if needed.
func GetString(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

// GetByte retrieves the byte value of the argument, converting if needed.
func GetByte(v interface{}) byte {
	i := GetInt(v)

	return byte(i & math.MaxInt8)
}

// GetInt32 retrieves the int32 value of the argument, converting if needed.
func GetInt32(v interface{}) int32 {
	i := GetInt(v)

	return int32(i)
}

// GetInt retrieves the int value of the argument, converting if needed.
func GetInt(v interface{}) int {
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

// GetInt64 retrieves the int64 value of the argument, converting if needed.
func GetInt64(v interface{}) int64 {
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
		fmt.Scanf("%d", &result)
	}

	return result
}

// GetFloat64 retrieves the float64 value of the argument, converting if needed.
func GetFloat64(v interface{}) float64 {
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

// GetFloat32 retrieves the float32 value of the argument, converting if needed.
func GetFloat32(v interface{}) float32 {
	f := GetFloat64(v)

	return float32(f)
}

// GetString retrieves the boolean value of the argument, converting if needed.
func GetBool(v interface{}) bool {
	switch actual := v.(type) {
	case byte, int32, int, int64:
		return GetInt64(v) != 0

	case float64, float32:
		return GetFloat64(v) != 0.0

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

	case *EgoArray:
		size := actual.Len()
		result := NewArray(actual.valueType, size)

		for i := 0; i < size; i++ {
			v, _ := actual.Get(i)
			_ = result.Set(i, DeepCopy(v))
		}

		return result

	case *EgoMap:
		result := NewMap(actual.keyType, actual.valueType)
		keys := actual.Keys()

		for _, k := range keys {
			v, _, _ := actual.Get(k)
			_, _ = result.Set(k, DeepCopy(v))
		}

		return result

	case *EgoStruct:
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

// GetNativeMap extracts a map from an abstract interface. Returns nil
// if the interface did not contain a map. Note this is NOT an
// Ego map, but rather is used by the gremlin package for actual maps.
func GetNativeMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}

	return nil
}

// GetNativeArray extracts a struct from an abstract interface. Returns nil
// if the interface did not contain a struct/map.
func GetNativeArray(v interface{}) []interface{} {
	if m, ok := v.([]interface{}); ok {
		return m
	}

	return nil
}
