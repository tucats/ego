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
		return nil, errors.New(errors.ArgumentTypeError)
	}

	fn, ok := args[1].(*bytecode.ByteCode)
	if !ok {
		return nil, errors.New(errors.ArgumentTypeError)
	}

	var funcError *errors.EgoError

	// Use the native sort.Slice function, and provide a comparitor function
	// whose job is to run the supplied bytecode instructions, passing in
	// the two native arguments
	sort.Slice(array.BaseArray(), func(i, j int) bool {
		sliceSymbols := symbols.NewChildSymbolTable("sort slice", s)
		_ = sliceSymbols.SetAlways("__args", []interface{}{i, j})

		// Coerce the name of the bytecode to represent that it is the
		// anonymous compare function value. We only do this if it is
		// actually anonymous.
		if fn.Name == "" {
			fn.Name = "(anon)"
		}

		ctx := bytecode.NewContext(sliceSymbols, fn)
		ctx.SetTracing(true)

		err := ctx.Run()
		if err != nil {
			if funcError == nil {
				funcError = err
			}

			return false
		}

		v := ctx.Result()

		return util.GetBool(v)
	})

	return array, funcError
}
