package sort

import (
	"sort"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// sortStable implements sort.Stable.  It accepts any typed array and sorts it
// in non-decreasing order using a stable algorithm, so equal elements keep
// their original relative positions.
func sortStable(symTable *symbols.SymbolTable, args data.List) (any, error) {
	array, ok := args.Get(0).(*data.Array)
	if !ok {
		return nil, errors.ErrArgumentType.In("Stable").Context(data.TypeOf(args.Get(0)).String())
	}

	if err := stableSortArray(array); err != nil {
		return nil, err
	}

	return array, nil
}

// stableSortArray sorts a *data.Array in-place using a stable algorithm.
func stableSortArray(array *data.Array) error {
	switch array.Type().Kind() {
	case data.StringKind:
		base := array.BaseArray()
		sort.SliceStable(base, func(i, j int) bool {
			return data.String(base[i]) < data.String(base[j])
		})

	case data.ByteKind:
		// GetBytes returns the internal slice directly; sorting it updates the array.
		b := array.GetBytes()
		sort.SliceStable(b, func(i, j int) bool { return b[i] < b[j] })

	case data.IntKind:
		base := array.BaseArray()
		sort.SliceStable(base, func(i, j int) bool {
			vi, _ := data.Int(base[i])
			vj, _ := data.Int(base[j])
			return vi < vj
		})

	case data.Int32Kind:
		base := array.BaseArray()
		sort.SliceStable(base, func(i, j int) bool {
			vi, _ := data.Int32(base[i])
			vj, _ := data.Int32(base[j])
			return vi < vj
		})

	case data.Int64Kind:
		base := array.BaseArray()
		sort.SliceStable(base, func(i, j int) bool {
			vi, _ := data.Int64(base[i])
			vj, _ := data.Int64(base[j])
			return vi < vj
		})

	case data.Float32Kind:
		base := array.BaseArray()
		sort.SliceStable(base, func(i, j int) bool {
			vi, _ := data.Float32(base[i])
			vj, _ := data.Float32(base[j])
			return vi < vj
		})

	case data.Float64Kind:
		base := array.BaseArray()
		sort.SliceStable(base, func(i, j int) bool {
			vi, _ := data.Float64(base[i])
			vj, _ := data.Float64(base[j])
			return vi < vj
		})

	default:
		return errors.ErrInvalidType.In("Stable").Context(array.Type().String())
	}

	return nil
}
