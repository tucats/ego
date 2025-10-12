package os

import (
	"fmt"
	"os"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// sortSlice implements the sort.sortSlice() function.
func expand(s *symbols.SymbolTable, args data.List) (any, error) {
	var funcError error

	value := data.String(args.Get(0))

	a1 := args.Get(1)
	// First, is a1 a builtin or runtime function? If so, we're going to
	// call the function directly.
	if fn, ok := a1.(data.Function); ok {
		if fnx, ok := fn.Value.(func(string) string); ok {
			return os.Expand(value, fnx), nil
		}
	}

	fn, ok := a1.(*bytecode.ByteCode)
	if !ok {
		return nil, errors.ErrArgumentType.Context(fmt.Sprintf("argument %d: %s", 2, data.TypeOf(args.Get(1)).String()))
	}

	// Create a symbol table to use for the slice comparator callback function.
	expandSymbols := symbols.NewChildSymbolTable("os.Expand", s)

	// Coerce the name of the bytecode to represent that it is the
	// anonymous compare function value. We only do this if it is
	// actually anonymous.
	if fn.Name() == "" {
		fn.SetName(defs.Anon)
	}

	// Reusable context that will handle each callback.
	ctx := bytecode.NewContext(expandSymbols, fn)

	// Use the native os.Expand function, and provide a lookup function
	// whose job is to run the supplied bytecode instructions, passing in
	// the native argument
	result := os.Expand(value, func(key string) string {
		// Set the i,j variables as the current function arguments
		expandSymbols.SetAlways(defs.ArgumentListVariable,
			data.NewArrayFromInterfaces(data.StringType, key))
		// Run the provided lookup function
		if err := ctx.Run(); err != nil {
			if funcError == nil {
				funcError = err
			}

			return ""
		}

		// Return the result as this function's value.
		return data.String(ctx.Result())
	})

	return result, funcError
}
