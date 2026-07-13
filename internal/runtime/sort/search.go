package sort

import (
	"fmt"
	"sort"

	"github.com/tucats/ego/internal/language/bytecode"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// sortSearch implements sort.Search.  It wraps Go's sort.Search, invoking the
// supplied Ego function f(i int) bool as the predicate.  f is expected to be
// monotone: f(i) == false for i < threshold, true for i >= threshold.
// Returns the smallest index in [0, n) where f returns true, or n if none.
func sortSearch(s *symbols.SymbolTable, args data.List) (any, error) {
	n, err := data.Int(args.Get(0))
	if err != nil {
		err = errors.New(err).In("Search")

		return data.NewList(nil, err), err
	}

	fn, ok := args.Get(1).(*bytecode.ByteCode)
	if !ok {
		err := errors.ErrArgumentType.Context(fmt.Sprintf("argument 2: %s", data.TypeOf(args.Get(1)).String()))

		return data.NewList(nil, err), err
	}

	searchSymbols := symbols.NewChildSymbolTable("sort search", s)

	if fn.Name() == "" {
		fn.SetName(defs.Anon)
	}

	ctx := bytecode.NewContext(searchSymbols, fn)

	var funcError error

	result := sort.Search(n, func(i int) bool {
		searchSymbols.SetAlways(defs.ArgumentListVariable,
			data.NewArrayFromInterfaces(data.IntType, i))

		if err := ctx.Run(); err != nil {
			if funcError == nil {
				funcError = err
			}

			return false
		}

		return data.BoolOrFalse(ctx.Result())
	})

	return data.NewList(result, funcError), funcError
}

// sortSearchInts implements sort.SearchInts.  It searches an already-sorted
// []int for x, returning the smallest index at which x could be inserted to
// keep the array sorted (which is also the position of x itself if it is
// already present).
func sortSearchInts(s *symbols.SymbolTable, args data.List) (any, error) {
	array, ok := args.Get(0).(*data.Array)
	if !ok || array.Type().Kind() != data.IntKind {
		err := errors.ErrArgumentType.In("SearchInts").Context(data.TypeOf(args.Get(0)).String())

		return data.NewList(nil, err), err
	}

	x, err := data.Int(args.Get(1))
	if err != nil {
		err = errors.New(err).In("SearchInts")

		return data.NewList(nil, err), err
	}

	result := sort.Search(array.Len(), func(i int) bool {
		v, _ := array.Get(i)
		vi, _ := data.Int(v)

		return vi >= x
	})

	return data.NewList(result, nil), nil
}

// sortSearchFloat64s implements sort.SearchFloat64s.  It searches an
// already-sorted []float64 for x, returning the smallest index at which x
// could be inserted to keep the array sorted.
func sortSearchFloat64s(s *symbols.SymbolTable, args data.List) (any, error) {
	array, ok := args.Get(0).(*data.Array)
	if !ok || array.Type().Kind() != data.Float64Kind {
		err := errors.ErrArgumentType.In("SearchFloat64s").Context(data.TypeOf(args.Get(0)).String())

		return data.NewList(nil, err), err
	}

	x, err := data.Float64(args.Get(1))
	if err != nil {
		err = errors.New(err).In("SearchFloat64s")

		return data.NewList(nil, err), err
	}

	result := sort.Search(array.Len(), func(i int) bool {
		v, _ := array.Get(i)
		vi, _ := data.Float64(v)

		return vi >= x
	})

	return data.NewList(result, nil), nil
}

// sortSearchStrings implements sort.SearchStrings.  It searches an
// already-sorted []string for x, returning the smallest index at which x
// could be inserted to keep the array sorted.
func sortSearchStrings(s *symbols.SymbolTable, args data.List) (any, error) {
	array, ok := args.Get(0).(*data.Array)
	if !ok || array.Type().Kind() != data.StringKind {
		err := errors.ErrArgumentType.In("SearchStrings").Context(data.TypeOf(args.Get(0)).String())

		return data.NewList(nil, err), err
	}

	x := data.String(args.Get(1))

	result := sort.Search(array.Len(), func(i int) bool {
		v, _ := array.Get(i)

		return data.String(v) >= x
	})

	return data.NewList(result, nil), nil
}
