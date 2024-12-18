package data

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tucats/ego/errors"
)

// Coerce returns the value after it has been converted to the type of the
// model value. If the value passed in is non-nil but cannot be converted
// to the type of the model object, the function returns nil. Note that the
// model is an _instance_ of the type to convert to, not a type iteself.
func Coerce(v interface{}, model interface{}) interface{} {
	if e, ok := v.(error); ok {
		return e
	}

	switch model.(type) {
	case *errors.Error:
		return errors.Message(fmt.Sprintf("%v", v))

	// This is a bit of a hack, but we cannot convert maps generally. However, we allow
	// the case of a map with the same key type but value type of inteface as the model.
	case *Map:
		if sourceMap, ok := v.(*Map); ok {
			modelMap := model.(*Map)

			if sourceMap.KeyType() == modelMap.KeyType() && modelMap.elementType.kind == InterfaceKind {
				return v
			}
		}

		return nil

	case byte:
		return coerceToByte(v)

	case int32:
		return coerceInt32(v)

	case int64:
		return coerceToInt64(v)

	case int:
		return coerceToInt(v)

	case float32:
		return coerceFloat32(v)

	case float64:
		return coerceFloat64(v)

	case string:
		return coerceString(v)

	case bool:
		return coerceBool(v)
	}

	return nil
}

func coerceBool(v interface{}) interface{} {
	switch vv := v.(type) {
	case nil:
		return false

	case bool:
		return vv

	case byte, int32, int, int64:
		return (Int64(v) != 0)

	case float32, float64:
		return Float64(v) != 0.0

	case string:
		switch strings.TrimSpace(strings.ToLower(vv)) {
		case True:
			return true
		case False:
			return false
		default:
			return false
		}
	}

	return nil
}

func coerceString(v interface{}) interface{} {
	switch value := v.(type) {
	case bool:
		if value {
			return True
		}

		return False

	case byte:
		return strconv.Itoa(int(value))

	case int:
		return strconv.Itoa(value)

	case int32:
		return strconv.Itoa(int(value))

	case int64:
		return strconv.FormatInt(value, 10)

	case float32:
		return strconv.FormatFloat(float64(value), 'g', 8, 32)

	case float64:
		return strconv.FormatFloat(value, 'g', 10, 64)

	case string:
		return value

	case nil:
		return ""
	}

	return nil
}

func coerceFloat64(v interface{}) interface{} {
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
		st, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil
		}

		return st
	}

	return nil
}

func coerceFloat32(v interface{}) interface{} {
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
		st, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return nil
		}

		return float32(st)
	}

	return nil
}

func coerceToInt(v interface{}) interface{} {
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
		if err != nil {
			return nil
		}

		return st
	}

	return nil
}

func coerceToInt64(v interface{}) interface{} {
	switch value := v.(type) {
	case nil:
		return int64(0)

	case bool:
		if value {
			return int64(1)
		}

		return int64(0)

	case byte:
		return int64(value)

	case int:
		return int64(value)

	case int32:
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
		if err != nil {
			return nil
		}

		return int64(st)
	}

	return nil
}

func coerceInt32(v interface{}) interface{} {
	switch value := v.(type) {
	case nil:
		return int32(0)

	case bool:
		if value {
			return int32(1)
		}

		return int32(0)

	case int:
		return int32(value & math.MaxInt32)

	case int64:
		return int32(value & math.MaxInt32)

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
		if err != nil {
			return nil
		}

		return int32(st & math.MaxInt32)
	}

	return nil
}

func coerceToByte(v interface{}) interface{} {
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
		return byte(value & math.MaxInt8)

	case int32:
		return byte(value & math.MaxInt8)

	case int64:
		return byte(value & math.MaxInt8)

	case float32:
		return byte(int64(value) & math.MaxInt8)

	case float64:
		return byte(int64(value) & math.MaxInt8)

	case string:
		if value == "" {
			return 0
		}

		st, err := strconv.Atoi(value)
		if err != nil {
			return nil
		}

		return byte(st & math.MaxInt8)
	}

	return nil
}

// Normalize accepts two different values and promotes them to
// the most highest precision type of the values.  If they are
// both the same type already, no work is done.
//
// For example, passing in an int32 and a float64 returns the
// values both converted to float64.
func Normalize(v1 interface{}, v2 interface{}) (interface{}, interface{}) {
	kind1 := KindOf(v1)
	kind2 := KindOf(v2)

	if kind1 == kind2 {
		return v1, v2
	}

	if kind1 < kind2 {
		v1 = Coerce(v1, v2)
	} else {
		v2 = Coerce(v2, v1)
	}

	return v1, v2
}

// For a given Type, coverce the given value to the same
// type. This only works for builtin scalar values like
// int or string.
func (t Type) Coerce(v interface{}) interface{} {
	switch t.kind {
	case ByteKind:
		return Byte(v)

	case Int32Kind:
		return Int32(v)

	case IntKind:
		return Int(v)

	case Int64Kind:
		return Int64(v)

	case Float64Kind:
		return Float64(v)

	case Float32Kind:
		return Float32(v)

	case StringKind:
		return String(v)

	case BoolKind:
		return Bool(v)
	}

	return v
}
