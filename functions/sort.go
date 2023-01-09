package functions

import (
	"sort"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// SortStrings implements the sort.Strings function.
func SortStrings(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsKind(datatypes.StringKind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.EgoError(errors.ErrWrongArrayValueType).Context("sort.Strings()")
	}

	return nil, errors.EgoError(errors.ErrArgumentType).Context("sort.Strings()")
}

// SortBytes implements the sort.Bytes function.
func SortBytes(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsKind(datatypes.ByteKind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.EgoError(errors.ErrWrongArrayValueType).Context("sort.Bytes()")
	}

	return nil, errors.EgoError(errors.ErrArgumentType).Context("sort.Bytes()")
}

// SortInts implements the sort.Ints function.
func SortInts(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsKind(datatypes.IntKind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.EgoError(errors.ErrWrongArrayValueType).Context("sort.Ints()")
	}

	return nil, errors.EgoError(errors.ErrArgumentType).Context("sort.Ints()")
}

// SortInt32s implements the sort.Int32s function.
func SortInt32s(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsKind(datatypes.Int32Kind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.EgoError(errors.ErrWrongArrayValueType).Context("sort.Int32s()")
	}

	return nil, errors.EgoError(errors.ErrArgumentType).Context("sort.Int32s()")
}

// SortInt64s implements the sort.Int64s function.
func SortInt64s(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsKind(datatypes.Int64Kind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.EgoError(errors.ErrWrongArrayValueType).Context("sort.Int64s()")
	}

	return nil, errors.EgoError(errors.ErrArgumentType).Context("sort.Int64s()")
}

// SortFloats implements the sort.Floats function.
func SortFloats(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsKind(datatypes.Float64Kind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.EgoError(errors.ErrWrongArrayValueType).Context("sort.Floats()")
	}

	return nil, errors.EgoError(errors.ErrArgumentType).Context("sort.Floats()")
}

// SortFloat32s implements the sort.Float32s function.
func SortFloat32s(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsKind(datatypes.Float32Kind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.EgoError(errors.ErrWrongArrayValueType).Context("sort.Float32s()")
	}

	return nil, errors.EgoError(errors.ErrArgumentType).Context("sort.Float32s()")
}

// SortFloat64s implements the sort.Float64s function.
func SortFloat64s(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*datatypes.EgoArray); ok {
		if array.ValueType().IsKind(datatypes.Float64Kind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.EgoError(errors.ErrWrongArrayValueType).Context("sort.Float64s()")
	}

	return nil, errors.EgoError(errors.ErrArgumentType).Context("sort.Float64s()")
}

// Sort implements the sort.Sort() function, whichi sorts an array regardless of it's type.
func Sort(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Make a master array of the values presented
	var array []interface{}

	// Special case. If there is a single argument, and it is already an Ego array,
	// use the native sort function
	if len(args) == 1 {
		if array, ok := args[0].(*datatypes.EgoArray); ok {
			err := array.Sort()

			return array, err
		}
	}

	for _, a := range args {
		switch v := a.(type) {
		case *datatypes.EgoArray:
			array = append(array, v.BaseArray()...)

		default:
			array = append(array, v)
		}
	}

	if len(array) == 0 {
		return array, nil
	}

	v1 := array[0]

	switch rv := v1.(type) {
	case byte:
		intArray := make([]byte, 0)

		for _, i := range array {
			intArray = append(intArray, datatypes.Byte(i))
		}

		sort.Slice(intArray, func(i, j int) bool { return intArray[i] < intArray[j] })

		resultArray := datatypes.NewArray(&datatypes.ByteType, len(array))

		for n, i := range intArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case int:
		intArray := make([]int, 0)

		for _, i := range array {
			intArray = append(intArray, datatypes.Int(i))
		}

		sort.Ints(intArray)

		resultArray := datatypes.NewArray(&datatypes.IntType, len(array))

		for n, i := range intArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case int32:
		intArray := make([]int32, 0)

		for _, i := range array {
			intArray = append(intArray, datatypes.Int32(i))
		}

		sort.Slice(intArray, func(i, j int) bool { return intArray[i] < intArray[j] })

		resultArray := datatypes.NewArray(&datatypes.Int32Type, len(array))

		for n, i := range intArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case int64:
		intArray := make([]int64, 0)

		for _, i := range array {
			intArray = append(intArray, datatypes.Int64(i))
		}

		sort.Slice(intArray, func(i, j int) bool { return intArray[i] < intArray[j] })

		resultArray := datatypes.NewArray(&datatypes.Int64Type, len(array))

		for n, i := range intArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case float32:
		floatArray := make([]float32, 0)

		for _, i := range array {
			floatArray = append(floatArray, datatypes.Float32(i))
		}

		sort.Slice(floatArray, func(i, j int) bool { return floatArray[i] < floatArray[j] })

		resultArray := datatypes.NewArray(&datatypes.Float64Type, len(array))

		for n, i := range floatArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case float64:
		floatArray := make([]float64, 0)

		for _, i := range array {
			floatArray = append(floatArray, datatypes.Float64(i))
		}

		sort.Float64s(floatArray)

		resultArray := datatypes.NewArray(&datatypes.Float64Type, len(array))

		for n, i := range floatArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case string:
		stringArray := make([]string, 0)

		for _, i := range array {
			stringArray = append(stringArray, datatypes.String(i))
		}

		sort.Strings(stringArray)

		resultArray := datatypes.NewArray(&datatypes.StringType, len(array))

		for n, i := range stringArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	default:
		return nil, errors.EgoError(errors.ErrInvalidType).In("sort()").Context(datatypes.TypeOf(rv).String())
	}
}
