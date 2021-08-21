package datatypes

import (
	"fmt"
	"strconv"
	"strings"
)

// For a given interface, unwrap it.
func GetType(v interface{}) Type {
	if t, ok := v.(Type); ok {
		return t
	}

	if t, ok := v.(*Type); ok {
		return *t
	}

	return UndefinedType
}

func GetString(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

func GetByte(v interface{}) byte {
	i := GetInt(v)
	return byte(i)
}

func GetInt32(v interface{}) int32 {
	i := GetInt(v)
	return int32(i)
}

func GetInt(v interface{}) int {
	switch actual := v.(type) {
	case int32:
		return int(actual)

	case int:
		return actual

	case int64:
		return int(actual)

	case float32:
		return int(actual)

	case float64:
		return int(actual)

	case bool:
		if actual {
			return 1
		}

		return 0

	default:
		v, _ := strconv.Atoi(fmt.Sprintf("%v", actual))

		return v
	}
}

func GetFloat(v interface{}) float64 {
	switch actual := v.(type) {
	case int32:
		return float64(actual)

	case int:
		return float64(actual)

	case int64:
		return float64(actual)

	case float32:
		return float64(actual)

	case float64:
		return actual

	case bool:
		if actual {
			return 1.0
		}

		return 0.0

	default:
		f, _ := strconv.ParseFloat(fmt.Sprintf("%v", actual), 64)

		return f
	}
}

func GetFloat32(v interface{}) float32 {
	f := GetFloat(v)

	return float32(f)
}

func GetBool(v interface{}) bool {
	switch actual := v.(type) {
	case int:
		return actual != 0

	case float64:
		return actual != 0.0

	case bool:
		return actual

	case string:
		for _, str := range []string{"true", "yes", "1", "y", "t"} {
			if strings.EqualFold(actual, str) {
				return true
			}
		}
	}

	return false
}

func DeepCopy(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch actual := v.(type) {
	case byte:
		return actual
	case int32:
		return actual
	case int:
		return actual
	case string:
		return actual
	case bool:
		return actual
	case float32:
		return actual
	case float64:
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
		return nil // Unsupported type, like pointers
	}
}
