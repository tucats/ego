package util

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

// GetMap extracts a struct from an abstract interface. Returns nil
// if the interface did not contain a struct/map.
func GetMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}

	return nil
}

// GetArray extracts a struct from an abstract interface. Returns nil
// if the interface did not contain a struct/map.
func GetArray(v interface{}) []interface{} {
	if m, ok := v.([]interface{}); ok {
		return m
	}

	return nil
}

// GetInt64 takes a generic interface and returns the integer value, using
// type coercion if needed.
func GetInt64(v interface{}) int64 {
	switch v.(type) {
	case map[string]interface{}, []interface{}, nil:
		return int64(0)

	case error:
		return 0
	}

	return Coerce(v, int64(1)).(int64)
}

// GetInt takes a generic interface and returns the integer value, using
// type coercion if needed.
func GetInt(v interface{}) int {
	if v == nil {
		return 0
	}

	switch v.(type) {
	case error:
		return 0

	case map[string]interface{}, []interface{}, nil:
		return 0
	}

	return Coerce(v, 1).(int)
}

// GetBool takes a generic interface and returns the boolean value, using
// type coercion if needed.
func GetBool(v interface{}) bool {
	switch v.(type) {
	case error:
		return false

	case map[string]interface{}, []interface{}, nil:
		return false
	}

	return Coerce(v, true).(bool)
}

// GetString takes a generic interface and returns the string value, using
// type coercion if needed.
func GetString(v interface{}) string {
	switch actual := v.(type) {
	case error:
		return ""

	case *datatypes.EgoArray:
		return actual.String()

	case *datatypes.EgoMap:
		return actual.String()

	case map[string]interface{}:
		return Format(v)

	case []interface{}, nil:
		return ""
	}

	return Coerce(v, "").(string)
}

// GetFloat takes a generic interface and returns the float64 value, using
// type coercion if needed.
func GetFloat(v interface{}) float64 {
	switch v.(type) {
	case error:
		return 0.0

	case map[string]interface{}, []interface{}, nil:
		return 0.0
	}

	return Coerce(v, float64(0)).(float64)
}

// Coerce returns the value after it has been converted to the type of the
// model value.
func Coerce(v interface{}, model interface{}) interface{} {
	if e, ok := v.(error); ok {
		return e
	}

	switch model.(type) {
	case int64:
		switch value := v.(type) {
		case nil:
			return int64(0)

		case bool:
			if value {
				return int64(1)
			}

			return int64(0)

		case int:
			return int64(value)

		case int64:
			return value

		case float64:
			return int64(value)

		case string:
			st, err := strconv.Atoi(value)
			if !errors.Nil(err) {
				return nil
			}

			return int64(st)
		}

	case int:
		switch value := v.(type) {
		case nil:
			return 0

		case bool:
			if value {
				return 1
			}

			return 0

		case int64:
			return int(value)

		case int:
			return value

		case float64:
			return int(value)

		case string:
			st, err := strconv.Atoi(value)
			if !errors.Nil(err) {
				return nil
			}

			return st
		}

	case float64:
		switch value := v.(type) {
		case nil:
			return float64(0.0)

		case bool:
			if value {
				return float64(1.0)
			}

			return float64(0.0)

		case int:
			return float64(value)

		case int64:
			return float64(value)

		case float64:
			return value

		case string:
			st, _ := strconv.ParseFloat(value, 64)

			return st
		}

	case string:
		switch value := v.(type) {
		case bool:
			if value {
				return "true"
			}

			return "false"

		case int:
			return strconv.Itoa(value)

		case int64:
			return fmt.Sprintf("%v", value)

		case float64:
			return fmt.Sprintf("%v", value)

		case string:
			return value

		case nil:
			return ""
		}

	case bool:
		switch vv := v.(type) {
		case nil:
			return false

		case bool:
			return vv

		case int:
			return (vv != 0)

		case int64:
			return vv != int64(0)

		case float64:
			return vv != 0.0

		case string:
			switch vv {
			case "true":
				return true
			case "false":
				return false
			default:
				return false
			}
		}
	}

	return nil
}

// Normalize accepts two different values and promotes them to
// the most compatable format.
func Normalize(v1 interface{}, v2 interface{}) (interface{}, interface{}) {
	switch v1.(type) {
	case nil:
		switch v2.(type) {
		case string:
			return "", v2

		case bool:
			return false, v2

		case int:
			return 0, v2

		case int64:
			return int64(0), v2

		case float32:
			return float32(0), v2

		case float64:
			return float64(0), v2

		case []interface{}:
			return []interface{}{}, v2

		case map[string]interface{}:
			return map[string]interface{}{}, v2
		}

	case string:
		switch vv := v2.(type) {
		case string:
			return v1, v2

		case int:
			return v1, strconv.Itoa(vv)

		case float64:
			return v1, fmt.Sprintf("%v", vv)

		case bool:
			return v1, vv
		}

	case float64:
		switch vv := v2.(type) {
		case string:
			return fmt.Sprintf("%v", v1.(float64)), v2

		case int:
			return v1, float64(vv)

		case float64:
			return v1, v2

		case bool:
			if vv {
				return v1, 1.0
			}

			return v1, 0.0
		}

	case int:
		switch vv := v2.(type) {
		case string:
			return strconv.Itoa(v1.(int)), v2

		case int:
			return v1, v2

		case float64:
			return float64(v1.(int)), v2

		case bool:
			if vv {
				return v1, 1
			}

			return v1, 0
		}

	case int64:
		switch vv := v2.(type) {
		case string:
			return fmt.Sprintf("%v", v1.(int64)), v2

		case int:
			return v1.(int64), int64(vv)

		case int64:
			return v1, v2

		case float64:
			return float64(v1.(int64)), v2

		case bool:
			if vv {
				return v1, 1
			}

			return v1, 0
		}

	case bool:
		switch v2.(type) {
		case string:
			if v1.(bool) {
				return "true", v2.(string)
			}

			return "false", v2.(string)

		case int:
			if v1.(bool) {
				return 1, v2.(int)
			}

			return 0, v2.(int)

		case float64:
			if v1.(bool) {
				return 1.0, v2.(float64)
			}

			return 0.0, v2.(float64)

		case bool:
			return v1, v2
		}
	}

	return v1, v2
}

// CoerceType will coerce an interface to a given type by name.
func CoerceType(v interface{}, typeName string) interface{} {
	switch typeName {
	case "int":
		return Coerce(v, int(0))

	case "int64":
		return Coerce(v, int64(0))

	case "float32":
		return Coerce(v, float32(0))

	case "float64":
		return Coerce(v, float64(0))

	case "string":
		return Coerce(v, "")

	case "bool":
		return Coerce(v, true)

	default:
		return nil
	}
}

// InList is a support function that checks to see if a string matches
// any of a list of other strings.
func InList(s string, test ...string) bool {
	for _, t := range test {
		if s == t {
			return true
		}
	}

	return false
}

// Given a list of strings, convert them to a sorted list in
// Ego array format.
func MakeSortedArray(array []string) []interface{} {
	sort.Strings(array)
	result := make([]interface{}, len(array))

	for i, v := range array {
		result[i] = v
	}

	return result
}
