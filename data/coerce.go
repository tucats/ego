package data

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
)

// Coerce converts value to the same concrete type as model and returns the
// result.  The model argument is an _instance_ of the desired target type —
// not a type descriptor.  For example, to convert something to int32, pass
// int32(0) as the model (or any int32 value).
//
// If model is a *Type descriptor rather than a concrete value, Coerce first
// materializes a zero-value instance of that type and uses it as the model.
//
// Returns nil and an error if the conversion is not possible (e.g. converting
// a struct to an integer).
func Coerce(value any, model any) (any, error) {
	if e, ok := value.(error); ok {
		value = errors.New(e)
	}

	// If the model is a type specification, create an instance of that type to
	// use as the model.
	if t, ok := model.(*Type); ok {
		model = InstanceOfType(t)
	}

	switch mt := model.(type) {
	case *Array:
		if va, ok := value.(*Array); ok {
			if va.valueType.kind == mt.valueType.kind {
				return value, nil
			}
		}

		// Can't do other kinds of coercions.
		return nil, errors.ErrInvalidValue.Context(value)

	case *errors.Error:
		return errors.Message(fmt.Sprintf("%v", value)), nil

	case *Type:
		if _, ok := value.(*Type); ok {
			return value, nil
		}

	// This is a bit of a hack, but we cannot convert maps generally. However, we allow
	// the case of a map with the same key type but value type of interface as the model.
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

	case int8:
		return coerceToInt8(value)

	case int16:
		return coerceToInt16(value)

	case uint16:
		return coerceToUInt16(value)

	case int32:
		return coerceInt32(value)

	case uint32:
		return coerceUInt32(value)

	case int64:
		return coerceToInt64(value)

	case uint64:
		return coerceUInt64(value)

	case uint:
		return coerceUInt64(value)

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

func coerceBool(value any) (any, error) {
	switch actual := value.(type) {
	case nil:
		return false, nil

	case bool:
		return actual, nil

	case byte, int8, uint16, int16, int32, uint32, int, uint, int64, uint64:
		v, err := Int64(value)
		if err != nil {
			return false, err
		}

		return (v != 0), nil

	case float32, float64:
		v, err := Float64(value)
		if err != nil {
			return false, err
		}

		return v != 0.0, nil

	case string:
		test := strings.TrimSpace(strings.ToLower(actual))
		switch test {
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

func coerceString(v any) (any, error) {
	switch value := v.(type) {
	case nil:
		return "", nil

	case bool:
		if value {
			return True, nil
		}

		return False, nil

	case byte:
		return strconv.Itoa(int(value)), nil

	case int8:
		return strconv.Itoa(int(value)), nil

	case uint16:
		return strconv.Itoa(int(value)), nil

	case int16:
		return strconv.Itoa(int(value)), nil

	case int:
		return strconv.Itoa(value), nil

	case int32:
		return strconv.Itoa(int(value)), nil

	case int64:
		return strconv.FormatInt(value, 10), nil

	case uint32:
		return strconv.FormatUint(uint64(value), 10), nil

	case uint:
		return strconv.FormatUint(uint64(value), 10), nil

	case uint64:
		return strconv.FormatUint(value, 10), nil

	case float32:
		return strconv.FormatFloat(float64(value), 'g', 8, 32), nil

	case float64:
		return strconv.FormatFloat(value, 'g', 10, 64), nil

	case string:
		return value, nil
	}

	return Format(v), nil
}

func coerceFloat64(v any) (any, error) {
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

	case int8:
		return float64(value), nil

	case int16:
		return float64(value), nil

	case uint16:
		return float64(value), nil

	case uint:
		return float64(value), nil

	case uint32:
		return float64(value), nil

	case uint64:
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
			return nil, errors.ErrInvalidFloatValue.Context(value)
		}

		return st, nil
	}

	return nil, errors.ErrInvalidFloatValue.Context(v)
}

func coerceFloat32(v any) (any, error) {
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

	case int8:
		return float32(value), nil

	case int16:
		return float32(value), nil

	case uint16:
		return float32(value), nil

	case int32:
		if math.Abs(float64(value)) > math.MaxFloat32 {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return float32(value), nil

	case uint32:
		if math.Abs(float64(value)) > math.MaxFloat32 {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return float32(value), nil

	case int:
		return float32(value), nil

	case uint64:
		return float32(value), nil

	case int64:
		return float32(value), nil

	case float32:
		return value, nil

	case float64:
		if math.Abs(value) > math.MaxFloat32 {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return float32(value), nil

	case string:
		st, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return nil, errors.ErrInvalidFloatValue.Context(value)
		}

		return float32(st), nil
	}

	return nil, errors.ErrInvalidFloatValue.Context(v)
}

func coerceToInt8(v any) (any, error) {
	switch value := v.(type) {
	case nil:
		return int8(0), nil

	case bool:
		if value {
			return int8(1), nil
		}

		return int8(0), nil

	case byte:
		if math.Abs(float64(value)) > 255 {
			if precisionError() {
				return 0, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return int8(value), nil

	case int8:
		if math.Abs(float64(value)) > 255 {
			if precisionError() {
				return 0, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return value, nil

	case int16:
		if int64(value) > math.MaxInt8 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int8(value), nil

	case uint16:
		if int64(value) > math.MaxInt8 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int8(value), nil

	case uint32:
		if int64(value) > math.MaxInt8 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int8(value), nil

	case uint:
		if int64(value) > math.MaxInt8 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int8(value), nil

	case uint64:
		if math.Abs(float64(value)) > math.MaxInt8 {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return int8(value), nil

	case int32:
		if int64(value) > math.MaxInt8 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int8(value), nil

	case int64:
		if int64(value) > math.MaxInt8 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int8(value), nil

	case int:
		if int64(value) > math.MaxInt8 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int8(value), nil

	case float32:
		if math.Abs(float64(value)) > math.MaxInt8 {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return int8(value), nil

	case float64:
		if math.Abs(value) > math.MaxInt8 {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return int8(value), nil

	case string:
		if value == "" {
			return int8(0), nil
		}

		st, err := egostrings.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		if int64(st) > math.MaxInt8 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(st)
		}

		return int8(st), nil
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

func coerceToInt16(v any) (any, error) {
	switch value := v.(type) {
	case nil:
		return int16(0), nil

	case bool:
		if value {
			return int16(1), nil
		}

		return int16(0), nil

	case byte:
		return int16(value), nil

	case int8:
		return int16(value), nil

	case int16:
		return value, nil

	case uint16:
		if int64(value) > math.MaxInt16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int16(value), nil

	case uint32:
		if int64(value) > math.MaxInt16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int16(value), nil

	case uint:
		if int64(value) > math.MaxInt16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int16(value), nil

	case uint64:
		if int64(value) > math.MaxInt16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int16(value), nil

	case int32:
		if int64(value) > math.MaxInt16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int16(value), nil

	case int64:
		if int64(value) > math.MaxInt16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int16(value), nil

	case int:
		if int64(value) > math.MaxInt16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int16(value), nil

	case float32:
		if int64(value) > math.MaxInt16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int16(value), nil

	case float64:
		if int64(value) > math.MaxInt16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int16(value), nil

	case string:
		if value == "" {
			return 0, nil
		}

		st, err := egostrings.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		if int64(st) > math.MaxInt16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(st)
		}

		return int16(st), nil
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

func coerceToUInt16(v any) (any, error) {
	switch value := v.(type) {
	case nil:
		return uint16(0), nil

	case bool:
		if value {
			return uint16(1), nil
		}

		return uint16(0), nil

	case byte:
		return uint16(value), nil

	case int8:
		return uint16(value), nil

	case int16:
		return uint16(value), nil

	case uint16:
		return value, nil

	case uint32:
		if int64(value) > math.MaxUint16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return uint16(value), nil

	case uint:
		if int64(value) > math.MaxUint16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return uint16(value), nil

	case uint64:
		if int64(value) > math.MaxUint16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return uint16(value), nil

	case int32:
		if int64(value) > math.MaxUint16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return uint16(value), nil

	case int64:
		if int64(value) > math.MaxUint16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return uint16(value), nil

	case int:
		if int64(value) > math.MaxUint16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return uint16(value), nil

	case float32:
		if int64(value) > math.MaxUint16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return uint16(value), nil

	case float64:
		if int64(value) > math.MaxUint16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return uint16(value), nil

	case string:
		if value == "" {
			return 0, nil
		}

		st, err := egostrings.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		if int64(st) > math.MaxUint16 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(st)
		}

		return uint16(st), nil
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

func coerceToInt(v any) (any, error) {
	switch value := v.(type) {
	case nil:
		return int(0), nil

	case bool:
		if value {
			return 1, nil
		}

		return 0, nil

	case byte:
		return int(value), nil

	case int8:
		return int(value), nil

	case int16:
		return int(value), nil

	case uint16:
		return int(value), nil

	case uint32:
		return int(value), nil

	case uint:
		if math.Abs(float64(value)) > math.MaxInt {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return int(value), nil

	case uint64:
		if math.Abs(float64(value)) > math.MaxInt64 {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return int(value), nil

	case int32:
		return int(value), nil

	case int64:
		return int(value), nil

	case int:
		return value, nil

	case float32:
		if math.Abs(float64(value)) > math.MaxInt {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return int(value), nil

	case float64:
		if math.Abs(value) > math.MaxInt {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return int(value), nil

	case string:
		if value == "" {
			return 0, nil
		}

		st, err := egostrings.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		return st, nil
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

func coerceToInt64(v any) (any, error) {
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

	case int8:
		return int64(value), nil

	case int16:
		return int64(value), nil

	case uint16:
		return int64(value), nil

	case int:
		return int64(value), nil

	case int32:
		return int64(value), nil

	case int64:
		return value, nil

	case uint32:
		return int64(value), nil

	case uint:
		return int64(value), nil

	case uint64:
		if math.Abs(float64(value)) > math.MaxInt64 && precisionError() {
			return nil, errors.ErrLossOfPrecision.Context(value)
		}

		return int64(value), nil

	case float32:
		r := int64(value)
		if float64(r) != math.Floor(float64(value)) {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return r, nil

	case float64:
		r := int64(value)
		if float64(r) != math.Floor(value) {
			if precisionError() {
				return nil, errors.ErrLossOfPrecision.Context(value)
			}
		}

		return r, nil

	case string:
		if value == "" {
			return 0, nil
		}

		st, err := egostrings.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		return int64(st), nil
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

func coerceInt32(v any) (any, error) {
	switch value := v.(type) {
	case nil:
		return int32(0), nil

	case bool:
		if value {
			return int32(1), nil
		}

		return int32(0), nil

	case int8:
		return int32(value), nil

	case int16:
		return int32(value), nil

	case uint16:
		return int32(value), nil

	case int:
		return coerceInt64ToInt32(int64(value))

	case int64:
		return coerceInt64ToInt32(value)

	case int32:
		return value, nil

	case uint:
		return coerceUInt64ToInt32(uint64(value))

	case uint32:
		return coerceUInt64ToInt32(uint64(value))

	case uint64:
		return coerceUInt64ToInt32(uint64(value))

	case byte:
		return int32(value), nil

	case float32:
		return coerceFloat64ToInt32(float64(value))

	case float64:
		return coerceFloat64ToInt32(value)

	case string:
		if value == "" {
			return 0, nil
		}

		intValue, err := egostrings.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		return coerceInt32(intValue)
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

func coerceUInt32(v any) (any, error) {
	switch value := v.(type) {
	case nil:
		return uint32(0), nil

	case bool:
		if value {
			return uint32(1), nil
		}

		return uint32(0), nil

	case int8:
		return uint32(value), nil

	case int16:
		return uint32(value), nil

	case uint16:
		return uint32(value), nil

	case int:
		return coerceUInt64ToUInt32(uint64(value))

	case int64:
		return coerceUInt64ToUInt32(uint64(value))

	case int32:
		return uint32(value), nil

	case uint:
		return coerceUInt64ToUInt32(uint64(value))

	case uint32:
		return coerceUInt64ToUInt32(uint64(value))

	case uint64:
		return coerceUInt64ToUInt32(uint64(value))

	case byte:
		return uint32(value), nil

	case float32:
		return coerceFloat64ToUInt32(float64(value))

	case float64:
		return coerceFloat64ToUInt32(value)

	case string:
		if value == "" {
			return 0, nil
		}

		intValue, err := egostrings.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		return coerceUInt32(intValue)
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

func coerceUInt64(v any) (any, error) {
	switch value := v.(type) {
	case nil:
		return uint64(0), nil

	case bool:
		if value {
			return uint64(1), nil
		}

		return uint64(0), nil

	case int8:
		return uint64(value), nil

	case int16:
		return uint64(value), nil

	case uint16:
		return uint64(value), nil

	case int:
		return uint64(value), nil

	case int64:
		return uint64(value), nil

	case int32:
		return uint64(value), nil

	case uint:
		return uint64(value), nil

	case uint32:
		return uint64(value), nil

	case uint64:
		return value, nil

	case byte:
		return uint64(value), nil

	case float32:
		return coerceFloat64ToUInt64(float64(value))

	case float64:
		return coerceFloat64ToUInt64(value)

	case string:
		if value == "" {
			return 0, nil
		}

		intValue, err := egostrings.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		return coerceUInt64(intValue)
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

func coerceToByte(v any) (any, error) {
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

	case int8:
		return coerceInt64ToByte(int64(value))

	case int16:
		return coerceInt64ToByte(int64(value))

	case uint16:
		return coerceInt64ToByte(int64(value))

	case int:
		return coerceInt64ToByte(int64(value))

	case int32:
		return coerceInt64ToByte(int64(value))

	case int64:
		return coerceInt64ToByte(value)

	case uint:
		return coerceUInt64ToByte(uint64(value))

	case uint32:
		return coerceUInt64ToByte(uint64(value))

	case uint64:
		return coerceUInt64ToByte(value)

	case float32:
		return coerceFloat64ToByte(float64(value))

	case float64:
		return coerceFloat64ToByte(value)

	case string:
		if value == "" {
			return byte(0), nil
		}

		st, err := egostrings.Atoi(value)
		if err != nil {
			return nil, errors.ErrInvalidInteger.Context(value)
		}

		return coerceToByte(st)
	}

	return nil, errors.ErrInvalidInteger.Context(v)
}

// Normalize promotes v1 and v2 to a common type before performing arithmetic
// or comparison on them.  Go does not allow mixed-type arithmetic (you cannot
// add int32 + float64 without an explicit cast), so the Ego runtime calls
// Normalize whenever it evaluates a binary expression where the two operands
// have different types.
//
// The promotion follows the kind ordering defined at the top of types.go —
// lower-numbered kinds are less precise, so the less-precise value is coerced
// up to match the more-precise one.  For example:
//   - int32 (kind 6) + float64 (kind 13) → both become float64
//   - bool (kind 1) + int (kind 8)       → both become int
//
// If v1 and v2 already have the same kind, they are returned unchanged.
func Normalize(v1 any, v2 any) (any, any, error) {
	var err error

	kind1 := KindOf(v1)
	kind2 := KindOf(v2)

	if kind1 == kind2 {
		return v1, v2, nil
	}

	// Is either an array? If so, we just see if the value types work.
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

// Coerce is a method on Type that converts v to the type's own kind.
// It is a convenience wrapper around the package-level accessor functions
// (Int, Float64, String, Bool, etc.) that lets callers write
// myType.Coerce(value) without a type-switch.
// Only builtin scalar kinds are handled; composite types (struct, map, array)
// are returned unchanged.
func (t Type) Coerce(v any) (any, error) {
	switch t.kind {
	case ByteKind:
		return Byte(v)

	case Int8Kind:
		return Int8(v)

	case Int16Kind:
		return Int16(v)

	case UInt16Kind:
		return UInt16(v)

	case Int32Kind:
		return Int32(v)

	case UInt32Kind:
		return UInt32(v)

	case IntKind:
		return Int(v)

	case UIntKind:
		return UInt(v)

	case Int64Kind:
		return Int64(v)

	case UInt64Kind:
		return UInt64(v)

	case Float64Kind:
		return Float64(v)

	case Float32Kind:
		return Float32(v)

	case StringKind:
		return String(v), nil

	case BoolKind:
		return Bool(v)
	}

	return v, nil
}

// precisionError returns true when the server is configured to treat
// loss-of-precision conversions (e.g. truncating a float64 to int32) as
// errors rather than silently rounding.  The setting is read from the
// persistent configuration store on every call so runtime changes take
// effect immediately.
func precisionError() bool {
	return settings.GetBool(defs.PrecisionErrorSetting)
}

func coerceInt64ToInt32(value int64) (int32, error) {
	n := value
	if n < 0 {
		n = -n
	}

	if n > math.MaxInt32 {
		if precisionError() {
			return 0, errors.ErrLossOfPrecision.Context(value)
		}
	}

	return int32(value), nil
}

func coerceUInt64ToInt32(value uint64) (int32, error) {
	if value > math.MaxInt32 {
		if precisionError() {
			return 0, errors.ErrLossOfPrecision.Context(value)
		}
	}

	return int32(value), nil
}

func coerceUInt64ToUInt32(value uint64) (uint32, error) {
	if value > math.MaxInt32+1 {
		if precisionError() {
			return 0, errors.ErrLossOfPrecision.Context(value)
		}
	}

	return uint32(value), nil
}

func coerceFloat64ToInt32(value float64) (int32, error) {
	if math.Abs(float64(value)) > math.MaxInt32 {
		if precisionError() {
			return 0, errors.ErrLossOfPrecision.Context(value)
		}
	}

	return int32(value), nil
}

func coerceFloat64ToUInt32(value float64) (uint32, error) {
	if math.Abs(float64(value)) > math.MaxInt32+1 {
		if precisionError() {
			return 0, errors.ErrLossOfPrecision.Context(value)
		}
	}

	return uint32(value), nil
}

func coerceFloat64ToUInt64(value float64) (uint32, error) {
	if math.Abs(float64(value)) > math.MaxInt64+1 {
		if precisionError() {
			return 0, errors.ErrLossOfPrecision.Context(value)
		}
	}

	return uint32(value), nil
}

func coerceInt64ToByte(value int64) (byte, error) {
	if value < 0 || value > 255 {
		if precisionError() {
			return 0, errors.ErrLossOfPrecision.Context(value)
		}
	}

	return byte(value), nil
}

func coerceUInt64ToByte(value uint64) (byte, error) {
	if value > 255 {
		if precisionError() {
			return 0, errors.ErrLossOfPrecision.Context(value)
		}
	}

	return byte(value), nil
}

func coerceFloat64ToByte(value float64) (byte, error) {
	if value < 0.0 || value > 255.0 {
		if precisionError() {
			return 0, errors.ErrLossOfPrecision.Context(value)
		}
	}

	return byte(value), nil
}
