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

// sortSlice implements the sort.sortSlice() function.
func sortSlice(s *symbols.SymbolTable, args data.List) (any, error) {
	var funcError error

	array, ok := args.Get(0).(*data.Array)
	if !ok {
		return nil, errors.ErrArgumentType.Context(fmt.Sprintf("argument %d: %s", 1, data.TypeOf(args.Get(0)).String()))
	}

	fn, ok := args.Get(1).(*bytecode.ByteCode)
	if !ok {
		return nil, errors.ErrArgumentType.Context(fmt.Sprintf("argument %d: %s", 2, data.TypeOf(args.Get(1)).String()))
	}

	// Create a symbol table to use for the slice comparator callback function.
	sliceSymbols := symbols.NewChildSymbolTable("sort slice", s)

	// Coerce the name of the bytecode to represent that it is the
	// anonymous compare function value. We only do this if it is
	// actually anonymous.
	if fn.Name() == "" {
		fn.SetName(defs.Anon)
	}

	// Reusable context that will handle each callback.
	ctx := bytecode.NewContext(sliceSymbols, fn)

	// Use the native sort.Slice function, and provide a comparison function
	// whose job is to run the supplied bytecode instructions, passing in
	// the two native arguments
	sort.Slice(array.BaseArray(), func(i, j int) bool {
		// Set the i,j variables as the current function arguments
		sliceSymbols.SetAlways(defs.ArgumentListVariable,
			data.NewArrayFromInterfaces(data.IntType, i, j))
		// Run the comparator function
		if err := ctx.Run(); err != nil {
			if funcError == nil {
				funcError = err
			}

			return false
		}

		// Return the result as this function's value.
		return data.BoolOrFalse(ctx.Result())
	})

	return array, funcError
}
