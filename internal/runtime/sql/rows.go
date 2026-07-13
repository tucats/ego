package sql

import (
	goSQL "database/sql"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
)

// rowsClose implements sql.Rows.Close(). It closes the underlying *goSQL.Rows
// cursor and sets the rows and client fields on the Rows struct to nil,
// releasing the resources held by the cursor. Callers should always close a
// cursor when they are done iterating, even if all rows were consumed.
//
// Returns ErrArgumentCount if any arguments are passed (method takes none).
// Returns ErrDatabaseClientClosed if the cursor is already closed.
func rowsClose(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() > 0 {
		return data.NewList(errors.ErrArgumentCount), errors.ErrArgumentCount
	}

	this := getThis(s)

	rowsVal := this.GetAlways(rowsFieldName)
	if rowsVal == nil {
		return data.NewList(errors.ErrDatabaseClientClosed), errors.ErrDatabaseClientClosed
	}

	rows := rowsVal.(*goSQL.Rows)

	var err error
	if closeErr := rows.Close(); closeErr != nil {
		err = errors.New(closeErr)
	}

	this.SetAlways(rowsFieldName, nil)
	this.SetAlways(clientFieldName, nil)

	ui.Log(ui.DBLogger, "db.rows.close", nil)

	return data.NewList(err), err
}

// rowsHeadings implements sql.Rows.Headings(). It returns a []any slice of
// column name strings in the same order as the SELECT list. This is useful
// when the caller needs to interpret a row returned by Scan() in no-arg mode,
// which produces a positional *data.Array without field labels.
//
// Returns ErrArgumentCount if any arguments are passed (method takes none).
// Returns ErrDatabaseClientClosed if the cursor has already been closed.
func rowsHeadings(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() > 0 {
		return data.NewList(nil, errors.ErrArgumentCount), errors.ErrArgumentCount
	}

	this := getThis(s)

	rowsVal := this.GetAlways(rowsFieldName)
	if rowsVal == nil {
		return data.NewList(nil, errors.ErrDatabaseClientClosed), errors.ErrDatabaseClientClosed
	}

	rows := rowsVal.(*goSQL.Rows)

	columns, err := rows.Columns()
	if err != nil {
		err = errors.New(err)

		return data.NewList(nil, err), err
	}

	result := make([]any, len(columns))
	for i, name := range columns {
		result[i] = name
	}

	headings := data.NewArrayFromInterfaces(data.StringType, result...)

	return data.NewList(headings, nil), nil
}

// rowsNext implements sql.Rows.Next(). It advances the cursor to the next row
// and returns true if a row is available, or false when the result set is
// exhausted or the cursor has been closed. Callers must call Next() before
// each Scan().
//
// Returns ErrArgumentCount if any arguments are passed (method takes none).
func rowsNext(s *symbols.SymbolTable, args data.List) (any, error) {
	if args.Len() > 0 {
		return data.NewList(false, errors.ErrArgumentCount), errors.ErrArgumentCount
	}

	this := getThis(s)
	if this == nil {
		return data.NewList(false, nil), nil
	}

	rowsVal := this.GetAlways(rowsFieldName)
	if rowsVal == nil {
		return data.NewList(false, nil), nil
	}

	rows := rowsVal.(*goSQL.Rows)
	active := rows.Next()

	if ui.IsActive(ui.DBLogger) {
		ui.Log(ui.DBLogger, "db.rows.next", ui.A{
			"flag": active})
	}

	return data.NewList(active, nil), nil
}

// rowsScan implements sql.Rows.Scan(values ...*any). It reads the current row
// (the cursor must have been advanced by a successful Next() call) and returns
// the column values. There are two calling modes:
//
// No-args mode (args is empty):
//   - StructMode == false — returns data.List{*data.Array, nil}, where the
//     array contains column values in SELECT-list order.
//   - StructMode == true  — returns data.List{*data.Struct, nil}, where the
//     struct has field names matching the column names.
//
// Pointer mode (args contains one *interface{} per column):
//   - Writes each column value back into the caller's pointer, mimicking
//     the standard goSQL.Rows.Scan() API.
//   - Returns data.List{nil, nil} on success.
//
// Returns ErrNoFunctionReceiver if called without a receiver, or
// ErrDatabaseClientClosed if the cursor has already been closed.
func rowsScan(s *symbols.SymbolTable, args data.List) (any, error) {
	this := getThis(s)
	if this == nil {
		return data.NewList(nil, errors.ErrNoFunctionReceiver), errors.ErrNoFunctionReceiver
	}

	rowsVal := this.GetAlways(rowsFieldName)
	if rowsVal == nil {
		return data.NewList(nil, errors.ErrDatabaseClientClosed), errors.ErrDatabaseClientClosed
	}

	rows := rowsVal.(*goSQL.Rows)
	db := this.GetAlways(dbFieldName).(*data.Struct)
	asStruct := data.BoolOrFalse(db.GetAlways(asStructFieldName))
	columns, _ := rows.Columns()
	colTypes, _ := rows.ColumnTypes()
	colCount := len(columns)
	rowTemplate := make([]any, colCount)
	rowValues := make([]any, colCount)

	for i := range colTypes {
		rowTemplate[i] = &rowValues[i]
	}

	if err := rows.Scan(rowTemplate...); err != nil {
		if ui.IsActive(ui.DBLogger) {
			ui.Log(ui.DBLogger, "db.rows.scan.error", ui.A{
				"err": err.Error()})
		}

		return data.NewList(nil, errors.New(err)), errors.New(err)
	}

	if asStruct {
		rowMap := map[string]any{}

		for i, v := range columns {
			rowMap[v] = rowValues[i]
		}

		if ui.IsActive(ui.DBLogger) {
			ui.Log(ui.DBLogger, "db.rows.scan.struct", ui.A{
				"rpw": rowMap})
		}

		return data.NewList(data.NewStructFromMap(rowMap), nil), nil
	}

	if ui.IsActive(ui.DBLogger) {
		ui.Log(ui.DBLogger, "db.rows.scan.array", ui.A{
			"row": rowValues})
	}

	// If we got arguments that are arrays of pointers, it's the classic (Go) style of
	// a r.Scan() call. Write the values back to the caller's arguments.
	if args.Len() > 0 {
		for i := range args.Len() {
			ptr := args.Get(i)
			if ptrValue, ok := ptr.(*interface{}); ok {
				*ptrValue = rowValues[i]
			} else {
				ptrErr := errors.ErrInvalidPointerType.In("Scan").Context(data.TypeOf(ptr))
				
				return data.NewList(nil, ptrErr), ptrErr
			}
		}

		return data.NewList(nil, nil), nil
	}

	// We did not get any arguments, so the result is the array of values and it's up to the
	// caller to do with them as they wish.
	return data.NewList(data.NewArrayFromInterfaces(data.InterfaceType, rowValues...), nil), nil
}
