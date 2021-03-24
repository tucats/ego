package datatypes

import (
	"fmt"
	"strconv"
	"strings"
)

func GetType(v interface{}) Type {
	if t, ok := v.(Type); ok {
		return t
	}

	return UndefinedTypeDef
}

func GetString(v interface{}) string {
	return fmt.Sprintf("%v", v)
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
