package tables

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// lenTable implements the Len function, which returns the number of rows in the table.
func lenTable(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	// Get the table associated with the receiver variable.
	t, err := getTable(s)
	if err != nil {
		return nil, errors.New(err).In("Len")
	}

	// Return the length of the table.
	return t.Len(), nil
}

// widthTable implements the Width function, which returns the number of columns in the table.
func widthTable(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTable(s)
	if err != nil {
		return nil, errors.New(err).In("Width")
	}

	return t.Width(), nil
}

// implements the Get function, which returns the value at the specified position in the table.
// The position is defined by the row number and column name.
func getTableElement(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTable(s)
	if err != nil {
		err = errors.New(err).In("Get")

		return data.NewList(nil, err), err
	}

	rowIndex, err := data.Int(args.Get(0))
	if err != nil {
		return nil, errors.New(err).In("Get")
	}

	columnName := data.String(args.Get(1))

	// If the row index is out of bounds, complain.
	if rowIndex < 0 || rowIndex >= t.Len() {
		err = errors.ErrInvalidRange.Context(rowIndex).In("Get")

		return data.NewList(nil, err), err
	}

	// Convert the column name to a column position.
	columnIndex := -1

	for i, column := range t.GetHeadings() {
		if strings.EqualFold(column, columnName) {
			columnIndex = i

			break
		}
	}

	if columnIndex == -1 {
		err = errors.ErrInvalidColumnName.Context(columnName).In("Get")

		return data.NewList(nil, err), err
	}

	row, err := t.GetRow(rowIndex)
	if err != nil {
		err = errors.New(err).In("Get")

		return data.NewList(nil, err), err
	}

	return data.NewList(row[columnIndex], nil), nil
}

// getRow implements the GetRow function, which returns the values at the specified row in the table.
func getRow(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTable(s)
	if err != nil {
		err = errors.New(err).In("GetRow")

		return data.NewList(nil, err), err
	}

	rowIndex, err := data.Int(args.Get(0))
	if err != nil {
		return nil, errors.New(err).In("GetRow")
	}

	if rowIndex < 0 || rowIndex >= t.Len() {
		err = errors.ErrInvalidRange.Context(rowIndex).In("GetRow")

		return data.NewList(nil, err), err
	}

	row, err := t.GetRow(rowIndex)
	if err != nil {
		err = errors.New(err).In("GetRow")

		return data.NewList(nil, err), err
	}

	result := make([]interface{}, len(row))
	for index, value := range row {
		result[index] = value
	}

	return data.NewList(data.NewArrayFromInterfaces(data.StringType, result...), nil), nil
}
