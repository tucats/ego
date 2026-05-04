package sort

import (
	"fmt"
	"sort"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// sortSearch implements sort.Search.  It wraps Go's sort.Search, invoking the
// supplied Ego function f(i int) bool as the predicate.  f is expected to be
// monotone: f(i) == false for i < threshold, true for i >= threshold.
// Returns the smallest index in [0, n) where f returns true, or n if none.
func sortSearch(s *symbols.SymbolTable, args data.List) (any, error) {
	n, err := data.Int(args.Get(0))
	if err != nil {
		return nil, errors.New(err).In("Search")
	}

	fn, ok := args.Get(1).(*bytecode.ByteCode)
	if !ok {
		return nil, errors.ErrArgumentType.Context(fmt.Sprintf("argument 2: %s", data.TypeOf(args.Get(1)).String()))
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

	return result, funcError
}
