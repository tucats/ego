package tables

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Find implmeents the Find() method for tables.Table object
func findRows(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var funcError error

	t, err := getTable(s)
	if err != nil {
		err = errors.New(err).In("Find")
		return data.NewList(nil, err), err
	}

	// Get the required function that tests each row.
	fn, ok := args.Get(0).(*bytecode.ByteCode)
	if !ok {
		err = errors.ErrArgumentType.In("Find")
		return data.NewList(nil, err), err
	}

	// Build a result set that will be a list of integers that
	// contain the matching rows in the table.
	array := data.NewArray(data.IntType, 0)

	// Create a symbol table to use for the slice comparator callback function.
	findSymbols := symbols.NewChildSymbolTable("table find", s)

	// Coerce the name of the bytecode to represent that it is the
	// anonymous compare function value. We only do this if it is
	// actually anonymous.
	if fn.Name() == "" {
		fn.SetName(defs.Anon)
	}

	// Reusable context that will handle each callback.
	ctx := bytecode.NewContext(findSymbols, fn)

	// Iterate over the rows in the table and call the user-provided
	// function to see if this row matches the criteria.
	for i := 0; i < t.Len(); i++ {
		// Get the row as an array of strings and convert it to
		// a native array.
		row, err := t.GetRow(i)
		if err != nil {
			err = errors.New(err).In("Find")
			return data.NewList(nil, err), err
		}

		rowArray := data.NewArray(data.StringType, len(row))
		for j, value := range row {
			rowArray.Set(j, value)
		}

		// Set the row as the current function argument. This meeans
		// each column will be one of the variadic function arguments.
		findSymbols.SetAlways(defs.ArgumentListVariable, rowArray)

		// Run the comparator function
		if err := ctx.RunFromAddress(0); err != nil {
			funcError = err

			break
		}

		// IF the function returns true, add the row index to the result set.
		if data.Bool(ctx.Result()) {
			array.Append(data.Int(i))
		}

	}

	return data.NewList(array, funcError), funcError
}
