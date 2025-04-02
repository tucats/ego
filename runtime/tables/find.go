package tables

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Find implements the Find() method for tables.Table object. This accepts an Ego
// function as the first argument, and calls this function for each row in the table.
// to "find" the matching rows of the table.
//
// The function must be declared such that it accepts the column names as parameters,
// in the order of the columns in the row. Each argument is a string value; the find
// function should do whatever is needed to evaluate the string values of each field
// to determine if this row should be considered as "found". If os, the function
// should return true, else false.
//
// The result of the findRows function is an array of integers indicating which table
// rows were "found".
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
			_ = rowArray.Set(j, value)
		}

		// Set the row as the current function argument. This means
		// each column will be one of the variadic function arguments.
		findSymbols.SetAlways(defs.ArgumentListVariable, rowArray)

		// Run the comparator function
		if err := ctx.RunFromAddress(0); err != nil {
			funcError = err

			break
		}

		// If the function returns true, add the row index to the result set.
		if data.BoolOrFalse(ctx.Result()) {
			array.Append(data.IntOrZero(i))
		}
	}

	return data.NewList(array, funcError), funcError
}
