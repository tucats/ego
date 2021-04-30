package runtime

import (
	"sort"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// sortSlice implements the sort.Slice() function. Beause this function requires a callback
// function written as bytecode, it cannot be in the functions package to avoid an import
// cycle problem. So this function (and others like it) are declared outside the functions
// package here in the runtime package, and are manually added to the dictionary when the
// run command is invoked.
func sortSlice(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	array, ok := args[0].(*datatypes.EgoArray)
	if !ok {
		return nil, errors.New(errors.ErrArgumentType)
	}

	fn, ok := args[1].(*bytecode.ByteCode)
	if !ok {
		return nil, errors.New(errors.ErrArgumentType)
	}

	var funcError *errors.EgoError

	// Create a symbol table to use for the slice comparator callback function.
	sliceSymbols := symbols.NewChildSymbolTable("sort slice", s)

	// Coerce the name of the bytecode to represent that it is the
	// anonymous compare function value. We only do this if it is
	// actually anonymous.
	if fn.Name == "" {
		fn.Name = "<anon>"
	}

	// Reusable context that will handle each callback.
	ctx := bytecode.NewContext(sliceSymbols, fn)

	// Use the native sort.Slice function, and provide a comparitor function
	// whose job is to run the supplied bytecode instructions, passing in
	// the two native arguments
	sort.Slice(array.BaseArray(), func(i, j int) bool {
		// Set the i,j variables as the current function arguments
		_ = sliceSymbols.SetAlways("__args", datatypes.NewFromArray(datatypes.IntType, []interface{}{i, j}))

		// Run the comparator function
		err := ctx.RunFromAddress(0)
		if err != nil {
			if funcError == nil {
				funcError = err
			}

			return false
		}

		// Return the result as this function's value.
		return util.GetBool(ctx.Result())
	})

	return array, funcError
}
