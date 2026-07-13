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

// sortableSlice returns the native Go slice backing an Ego array, suitable
// for passing to sort.Slice/sort.SliceStable so the sort mutates the array's
// actual storage in place. BaseArray() returns a fresh []any copy for a byte
// array (see its doc comment), so sorting that copy would silently leave the
// original array unchanged; GetBytes() returns the real backing []byte for
// byte arrays, so it must be used instead in that one case.
func sortableSlice(array *data.Array) any {
	if array.Type().Kind() == data.ByteKind {
		return array.GetBytes()
	}

	return array.BaseArray()
}

// sortSlice implements the sort.sortSlice() function.
func sortSlice(s *symbols.SymbolTable, args data.List) (any, error) {
	var funcError error

	array, ok := args.Get(0).(*data.Array)
	if !ok {
		err := errors.ErrArgumentType.Context(fmt.Sprintf("argument %d: %s", 1, data.TypeOf(args.Get(0)).String()))

		return data.NewList(nil, err), err
	}

	fn, ok := args.Get(1).(*bytecode.ByteCode)
	if !ok {
		err := errors.ErrArgumentType.Context(fmt.Sprintf("argument %d: %s", 2, data.TypeOf(args.Get(1)).String()))

		return data.NewList(nil, err), err
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
	sort.Slice(sortableSlice(array), func(i, j int) bool {
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

	return data.NewList(array, funcError), funcError
}

// sortSliceStable implements sort.SliceStable.  It is identical to sortSlice
// except that it uses sort.SliceStable, so equal elements (as determined by
// the comparator) keep their original relative positions.
func sortSliceStable(s *symbols.SymbolTable, args data.List) (any, error) {
	var funcError error

	array, ok := args.Get(0).(*data.Array)
	if !ok {
		err := errors.ErrArgumentType.Context(fmt.Sprintf("argument %d: %s", 1, data.TypeOf(args.Get(0)).String()))

		return data.NewList(nil, err), err
	}

	fn, ok := args.Get(1).(*bytecode.ByteCode)
	if !ok {
		err := errors.ErrArgumentType.Context(fmt.Sprintf("argument %d: %s", 2, data.TypeOf(args.Get(1)).String()))

		return data.NewList(nil, err), err
	}

	sliceSymbols := symbols.NewChildSymbolTable("sort slice stable", s)

	if fn.Name() == "" {
		fn.SetName(defs.Anon)
	}

	ctx := bytecode.NewContext(sliceSymbols, fn)

	sort.SliceStable(sortableSlice(array), func(i, j int) bool {
		sliceSymbols.SetAlways(defs.ArgumentListVariable,
			data.NewArrayFromInterfaces(data.IntType, i, j))

		if err := ctx.Run(); err != nil {
			if funcError == nil {
				funcError = err
			}

			return false
		}

		return data.BoolOrFalse(ctx.Result())
	})

	return data.NewList(array, funcError), funcError
}
