package sort

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// isArraySorted returns true when every adjacent pair in the array is in
// non-decreasing order.  The comparison is done in the native element type.
func isArraySorted(array *data.Array) (bool, error) {
	n := array.Len()

	for i := 1; i < n; i++ {
		prev, err := array.Get(i - 1)
		if err != nil {
			return false, err
		}

		curr, err := array.Get(i)
		if err != nil {
			return false, err
		}

		var less bool

		switch array.Type().Kind() {
		case data.StringKind:
			less = data.String(curr) < data.String(prev)
		case data.ByteKind:
			p, _ := data.Byte(prev)
			c, _ := data.Byte(curr)
			less = c < p
		case data.IntKind:
			p, _ := data.Int(prev)
			c, _ := data.Int(curr)
			less = c < p
		case data.Int32Kind:
			p, _ := data.Int32(prev)
			c, _ := data.Int32(curr)
			less = c < p
		case data.Int64Kind:
			p, _ := data.Int64(prev)
			c, _ := data.Int64(curr)
			less = c < p
		case data.Float32Kind:
			p, _ := data.Float32(prev)
			c, _ := data.Float32(curr)
			less = c < p
		case data.Float64Kind:
			p, _ := data.Float64(prev)
			c, _ := data.Float64(curr)
			less = c < p
		default:
			return false, errors.ErrInvalidType.In("IsSorted").Context(array.Type().String())
		}

		if less {
			return false, nil
		}
	}

	return true, nil
}

// sortIsSorted implements sort.IsSorted.  It accepts any typed array and
// returns true if the elements are already in non-decreasing order.
func sortIsSorted(s *symbols.SymbolTable, args data.List) (any, error) {
	array, ok := args.Get(0).(*data.Array)
	if !ok {
		return nil, errors.ErrArgumentType.In("IsSorted").Context(data.TypeOf(args.Get(0)).String())
	}

	return isArraySorted(array)
}

// sortIntsAreSorted implements sort.IntsAreSorted.
func sortIntsAreSorted(s *symbols.SymbolTable, args data.List) (any, error) {
	array, ok := args.Get(0).(*data.Array)
	if !ok || array.Type().Kind() != data.IntKind {
		return nil, errors.ErrArgumentType.In("IntsAreSorted").Context(data.TypeOf(args.Get(0)).String())
	}

	return isArraySorted(array)
}

// sortFloat64sAreSorted implements sort.Float64sAreSorted.
func sortFloat64sAreSorted(s *symbols.SymbolTable, args data.List) (any, error) {
	array, ok := args.Get(0).(*data.Array)
	if !ok || array.Type().Kind() != data.Float64Kind {
		return nil, errors.ErrArgumentType.In("Float64sAreSorted").Context(data.TypeOf(args.Get(0)).String())
	}

	return isArraySorted(array)
}

// sortStringsAreSorted implements sort.StringsAreSorted.
func sortStringsAreSorted(s *symbols.SymbolTable, args data.List) (any, error) {
	array, ok := args.Get(0).(*data.Array)
	if !ok || array.Type().Kind() != data.StringKind {
		return nil, errors.ErrArgumentType.In("StringsAreSorted").Context(data.TypeOf(args.Get(0)).String())
	}

	return isArraySorted(array)
}
