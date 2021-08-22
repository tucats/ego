package datatypes

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/errors"
)

// Coerce returns the value after it has been converted to the type of the
// model value.
func Coerce(v interface{}, model interface{}) interface{} {
	if e, ok := v.(error); ok {
		return e
	}

	switch model.(type) {
	case byte:
		switch value := v.(type) {
		case nil:
			return byte(0)

		case bool:
			if value {
				return byte(1)
			}

			return byte(0)

		case byte:
			return value

		case int:
			return byte(value)

		case int32:
			return byte(value)

		case int64:
			return byte(value)

		case float32:
			return byte(value)

		case float64:
			return byte(value)

		case string:
			if value == "" {
				return 0
			}

			st, err := strconv.Atoi(value)
			if !errors.Nil(err) {
				return nil
			}

			return byte(st)
		}

	case int32:
		switch value := v.(type) {
		case nil:
			return int32(0)

		case bool:
			if value {
				return int32(1)
			}

			return int32(0)

		case int:
			return int32(value)

		case int64:
			return int32(value)

		case int32:
			return value

		case byte:
			return int32(value)

		case float32:
			return int32(value)

		case float64:
			return int32(value)

		case string:
			if value == "" {
				return 0
			}

			st, err := strconv.Atoi(value)
			if !errors.Nil(err) {
				return nil
			}

			return int32(st)
		}

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

		case float32:
			return int64(value)

		case float64:
			return int64(value)

		case string:
			if value == "" {
				return 0
			}

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

		case byte:
			return int(value)

		case int32:
			return int(value)

		case int64:
			return int(value)

		case int:
			return value

		case float32:
			return int(value)

		case float64:
			return int(value)

		case string:
			if value == "" {
				return 0
			}

			st, err := strconv.Atoi(value)
			if !errors.Nil(err) {
				return nil
			}

			return st
		}

	case float32:
		switch value := v.(type) {
		case nil:
			return float32(0.0)

		case bool:
			if value {
				return float32(1.0)
			}

			return float32(0.0)

		case byte:
			return float32(value)

		case int32:
			return float32(value)

		case int:
			return float32(value)

		case int64:
			return float32(value)

		case float32:
			return value

		case float64:
			return float32(value)

		case string:
			st, _ := strconv.ParseFloat(value, 32)

			return float32(st)
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

		case byte:
			return float64(value)

		case int32:
			return float64(value)

		case int:
			return float64(value)

		case int64:
			return float64(value)

		case float32:
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

		case byte, int, int32, int64:
			return fmt.Sprintf("%v", GetInt(v))

		case float32:
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

		case byte, int32, int, int64:
			return (GetInt(v) != 0)

		case float32, float64:
			return GetFloat64(v) != 0.0

		case string:
			switch strings.TrimSpace(strings.ToLower(vv)) {
			case "true":
				return true
			case "false":
				return false
			default:
				return false
			}
		default:
			return false
		}
	}

	return nil
}

// Normalize accepts two different values and promotes them to
// the most compatable format.
func Normalize(v1 interface{}, v2 interface{}) (interface{}, interface{}) {
	t1 := TypeOf(v1).Kind()
	t2 := TypeOf(v2).Kind()

	if t1 == t2 {
		return v1, v2
	}

	if t1 < t2 {
		v1 = Coerce(v1, v2)
	} else {
		v2 = Coerce(v2, v1)
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

func (t Type) Coerce(v interface{}) interface{} {
	switch t.kind {
	case IntKind:
		return GetInt(v)

	case Float64Kind:
		return GetFloat64(v)

	case StringKind:
		return GetString(v)

	case BoolKind:
		return GetBool(v)
	}

	return v
}
