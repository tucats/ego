package sort

import (
	"sort"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// sortStrings implements the sort.sortStrings function.
func sortStrings(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*data.Array); ok {
		if array.ValueType().IsString() {
			err := array.Sort()

			return array, err
		}

		return nil, errors.ErrWrongArrayValueType.In("Strings")
	}

	return nil, errors.ErrArgumentType.In("Strings").Context(args[0])
}

// sortBytes implements the sort.sortBytes function.
func sortBytes(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*data.Array); ok {
		if array.ValueType().IsKind(data.ByteKind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.ErrWrongArrayValueType.Context("Bytes").In("Bytes")
	}

	return nil, errors.ErrArgumentType.In("Bytes")
}

// sortInts implements the sort.sortInts function.
func sortInts(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*data.Array); ok {
		if array.ValueType().IsKind(data.IntKind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.ErrWrongArrayValueType.In("Ints")
	}

	return nil, errors.ErrArgumentType.In("Ints")
}

// sortInt32s implements the sort.sortInt32s function.
func sortInt32s(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*data.Array); ok {
		if array.ValueType().IsKind(data.Int32Kind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.ErrWrongArrayValueType.In("Int32s")
	}

	return nil, errors.ErrArgumentType.In("Int32s")
}

// sortInt64s implements the sort.sortInt64s function.
func sortInt64s(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*data.Array); ok {
		if array.ValueType().IsKind(data.Int64Kind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.ErrWrongArrayValueType.In("Int64s")
	}

	return nil, errors.ErrArgumentType.In("Int64s")
}

// Floats implements the sort.Floats function.
func Floats(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*data.Array); ok {
		if array.ValueType().IsKind(data.Float64Kind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.ErrWrongArrayValueType.In("Floats")
	}

	return nil, errors.ErrArgumentType.In("Floats")
}

// sortFloat32s implements the sort.sortFloat32s function.
func sortFloat32s(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*data.Array); ok {
		if array.ValueType().IsKind(data.Float32Kind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.ErrWrongArrayValueType.In("Float32s")
	}

	return nil, errors.ErrArgumentType.In("Float32s")
}

// sortFloat64s implements the sort.sortFloat64s function.
func sortFloat64s(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if array, ok := args[0].(*data.Array); ok {
		if array.ValueType().IsKind(data.Float64Kind) {
			err := array.Sort()

			return array, err
		}

		return nil, errors.ErrWrongArrayValueType.In("Float64s")
	}

	return nil, errors.ErrArgumentType.In("Float64s")
}

// genericSort implements the sort.genericSort() function, whichi sorts an array regardless of it's type.
func genericSort(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	// Make a master array of the values presented
	var array []interface{}

	// Special case. If there is a single argument, and it is already an Ego array,
	// use the native sort function
	if len(args) == 1 {
		if array, ok := args[0].(*data.Array); ok {
			err := array.Sort()
			if err != nil {
				err = errors.NewError(err).In("Sort")
			}

			return array, err
		}
	}

	for _, a := range args {
		switch v := a.(type) {
		case *data.Array:
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
			intArray = append(intArray, data.Byte(i))
		}

		sort.Slice(intArray, func(i, j int) bool { return intArray[i] < intArray[j] })

		resultArray := data.NewArray(data.ByteType, len(array))

		for n, i := range intArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case int:
		intArray := make([]int, 0)

		for _, i := range array {
			intArray = append(intArray, data.Int(i))
		}

		sort.Ints(intArray)

		resultArray := data.NewArray(data.IntType, len(array))

		for n, i := range intArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case int32:
		intArray := make([]int32, 0)

		for _, i := range array {
			intArray = append(intArray, data.Int32(i))
		}

		sort.Slice(intArray, func(i, j int) bool { return intArray[i] < intArray[j] })

		resultArray := data.NewArray(data.Int32Type, len(array))

		for n, i := range intArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case int64:
		intArray := make([]int64, 0)

		for _, i := range array {
			intArray = append(intArray, data.Int64(i))
		}

		sort.Slice(intArray, func(i, j int) bool { return intArray[i] < intArray[j] })

		resultArray := data.NewArray(data.Int64Type, len(array))

		for n, i := range intArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case float32:
		floatArray := make([]float32, 0)

		for _, i := range array {
			floatArray = append(floatArray, data.Float32(i))
		}

		sort.Slice(floatArray, func(i, j int) bool { return floatArray[i] < floatArray[j] })

		resultArray := data.NewArray(data.Float64Type, len(array))

		for n, i := range floatArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case float64:
		floatArray := make([]float64, 0)

		for _, i := range array {
			floatArray = append(floatArray, data.Float64(i))
		}

		sort.Float64s(floatArray)

		resultArray := data.NewArray(data.Float64Type, len(array))

		for n, i := range floatArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	case string:
		stringArray := make([]string, 0)

		for _, i := range array {
			stringArray = append(stringArray, data.String(i))
		}

		sort.Strings(stringArray)

		resultArray := data.NewArray(data.StringType, len(array))

		for n, i := range stringArray {
			_ = resultArray.Set(n, i)
		}

		return resultArray, nil

	default:
		return nil, errors.ErrInvalidType.In("Sort").Context(data.TypeOf(rv).String())
	}
}
