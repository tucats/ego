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
func Coerce(value interface{}, model interface{}) (interface{}, error) {
	if e, ok := value.(error); ok {
		return e, nil
	}

	switch model.(type) {
	case *errors.Error:
		return errors.Message(fmt.Sprintf("%v", value)), nil

	// This is a bit of a hack, but we cannot convert maps generally. However, we allow
	// the case of a map with the same key type but value type of inteface as the model.
	case *Map:
		if sourceMap, ok := value.(*Map); ok {
			modelMap := model.(*Map)

			if sourceMap.KeyType() == modelMap.KeyType() && modelMap.elementType.kind == InterfaceKind {
				return value, nil
			}
		}

		return nil, nil

	case byte:
		return coerceToByte(value)

	case int32:
		return coerceInt32(value)

	case int64:
		return coerceToInt64(value)

	case int:
		return coerceToInt(value)

	case float32:
		return coerceFloat32(value)

	case float64:
		return coerceFloat64(value)

	case string:
		return coerceString(value)

	case bool:
		return coerceBool(value)
	}

	return nil, errors.ErrInvalidValue.Context(value)
}

func coerceBool(value interface{}) (interface{}, error) {
	switch actual := value.(type) {
	case nil:
		return false, nil

	case bool:
		return actual, nil

	case byte, int32, int, int64:
		return (Int64(value) != 0), nil

	case float32, float64:
		return Float64(value) != 0.0, nil

	case string:
		switch strings.TrimSpace(strings.ToLower(actual)) {
		case True:
			return true, nil
		case False:
			return false, nil
		case "":
			return false, nil
		default:
			return nil, errors.ErrInvalidBooleanValue.Context(actual)
		}
	}

	return nil, errors.ErrInvalidBooleanValue.Context(value)
}

func coerceString(v interface{}) (interface{}, error) {
	switch value := v.(type) {
	case bool:
		if value {
			return True, nil
		}

		return False, nil

	case byte:
		return strconv.Itoa(int(value)), nil

	case int:
		return strconv.Itoa(value), nil

	case int32:
		return strconv.Itoa(int(value)), nil

	case int64:
		return strconv.FormatInt(value, 10), nil

	case float32:
		return strconv.FormatFloat(float64(value), 'g', 8, 32), nil

	case float64:
		return strconv.FormatFloat(value, 'g', 10, 64), nil

	case string:
		return value, nil

	case nil:
		return "", nil
	}

	return Format(v), nil
}

func coerceFloat64(v interface{}) (interface{}, error) {
	switch value := v.(type) {
	case nil:
		return float64(0.0), nil

	case bool:
		if value {
			return float64(1.0), nil
		}

		return float64(0.0), nil

	case byte:
		return float64(value), nil

	case int32:
		return float64(value), nil

	case int:
		return float64(value), nil

	case int64:
		return float64(value), nil

	case float32:
		return float64(value), nil

	case float64:
		return value, nil

	case string:
		st, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}

		return st, nil
	}

	return nil, errors.ErrInvalidFloatValue.Context(v)
}

func coerceFloat32(v interface{}) (interface{}, error) {
	switch value := v.(type) {
	case nil:
		return float32(0.0), nil

	case bool:
		if value {
			return float32(1.0), nil
		}

		return float32(0.0), nil

	case byte:
		return float32(value), nil

	case int32:
		return float32(value), nil

	case int:
		return float32(value), nil

	case int64:
		return float32(value), nil

	case float32:
		return value, nil

	case float64:
		return float32(value), nil

	case string:
		st, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return nil, err
		}

		return float32(st), nil
	}

	return nil, errors.ErrInvalidFloatValue.Context(v)
}

func coerceToInt(v interface{}) (interface{}, error) {
	switch value := v.(type) {
	case nil:
		return 0, nil

	case bool:
		if value {
			return 1, nil
		}

		return 0, nil

	case byte:
		return int(value), nil

	case int32:
		return int(value), nil

	case int64:
		return int(value), nil

	case int:
		return value, nil

	case float32:
		r := int(value)
		if float64(r) != math.Floor(float64(value)) {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return r, nil

	case float64:
		r := int(value)
		if float64(r) != math.Floor(value) {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return r, nil

	case string:
		if value == "" {
			return 0, nil
		}

		st, err := strconv.Atoi(value)
		if err != nil {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return st, nil
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

func coerceToInt64(v interface{}) (interface{}, error) {
	switch value := v.(type) {
	case nil:
		return int64(0), nil

	case bool:
		if value {
			return int64(1), nil
		}

		return int64(0), nil

	case byte:
		return int64(value), nil

	case int:
		return int64(value), nil

	case int32:
		return int64(value), nil

	case int64:
		return value, nil

	case float32:
		r := int64(value)
		if float64(r) != math.Floor(float64(value)) {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return r, nil

	case float64:
		r := int64(value)
		if float64(r) != math.Floor(value) {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return r, nil

	case string:
		if value == "" {
			return 0, nil
		}

		st, err := strconv.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		return int64(st), nil
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

func coerceInt32(v interface{}) (interface{}, error) {
	switch value := v.(type) {
	case nil:
		return int32(0), nil

	case bool:
		if value {
			return int32(1), nil
		}

		return int32(0), nil

	case int:
		result := int32(value & math.MaxInt32)
		if int(result) != value {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return result, nil

	case int64:
		result := int32(value & math.MaxInt32)
		if (value & math.MaxInt32) != int64(result) {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return result, nil

	case int32:
		return value, nil

	case byte:
		return int32(value), nil

	case float32:
		return int32(value), nil

	case float64:
		result := int32(value)
		if float64(result) != math.Floor(value) {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return result, nil

	case string:
		if value == "" {
			return 0, nil
		}

		intValue, err := strconv.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		return coerceInt32(intValue)
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

func coerceToByte(v interface{}) (interface{}, error) {
	switch value := v.(type) {
	case nil:
		return byte(0), nil

	case bool:
		if value {
			return byte(1), nil
		}

		return byte(0), nil

	case byte:
		return value, nil

	case int:
		result := byte(value & math.MaxInt8)
		if int(result) != value {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return result, nil

	case int32:
		result := byte(value & math.MaxInt8)
		if (value & math.MaxInt8) != int32(result) {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return result, nil

	case int64:
		result := byte(value & math.MaxInt8)
		if (value & math.MaxInt8) != int64(result) {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return result, nil

	case float32:
		result := byte(value)
		if float64(result) != math.Floor(float64(value)) {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return result, nil

	case float64:
		result := byte(value)
		if float64(result) != math.Floor(value) {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return result, nil

	case string:
		if value == "" {
			return 0, nil
		}

		st, err := strconv.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		return coerceToByte(st)
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

// Normalize accepts two different values and promotes them to
// the most highest precision type of the values.  If they are
// both the same type already, no work is done.
//
// For example, passing in an int32 and a float64 returns the
// values both converted to float64.
func Normalize(v1 interface{}, v2 interface{}) (interface{}, interface{}, error) {
	var err error

	kind1 := KindOf(v1)
	kind2 := KindOf(v2)

	if kind1 == kind2 {
		return v1, v2, nil
	}

	// Is either an array? If so, we just see if the valeu types work.
	if kind1 == ArrayKind || kind1 == InterfaceKind {
		if array, ok := v1.(*Array); ok {
			k := array.valueType.Kind()
			if k == kind2 || k == InterfaceKind {
				return v1, v2, nil
			}
		}
	}

	if kind2 == ArrayKind || kind2 == InterfaceKind {
		if array, ok := v2.(*Array); ok {
			k := array.valueType.Kind()
			if k == kind1 || k == InterfaceKind {
				return v1, v2, nil
			}
		}
	}

	if kind1 < kind2 {
		v1, err = Coerce(v1, v2)
		if err != nil {
			return nil, nil, err
		}
	} else {
		v2, err = Coerce(v2, v1)
		if err != nil {
			return nil, nil, err
		}
	}

	return v1, v2, nil
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
