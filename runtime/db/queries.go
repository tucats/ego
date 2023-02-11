package db

import (
	"database/sql"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// query executes a query, with optional parameter substitution, and returns row object
// for subsequent calls to fetch the data.
func query(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if args.Len() == 0 {
		return nil, errors.ErrArgumentCount
	}

	db, tx, err := client(s)
	if err != nil {
		return data.NewList(nil, err), err
	}

	this := getThis(s)
	this.SetAlways(rowCountFieldName, -1)

	query := data.String(args.Get(0))

	var rows *sql.Rows

	var e2 error

	if tx == nil {
		ui.Log(ui.DBLogger, "QueryRows: %s", query)

		rows, e2 = db.Query(query, args.Elements()[1:args.Len()]...)
	} else {
		ui.Log(ui.DBLogger, "(Tx) QueryRows: %s", query)

		rows, e2 = tx.Query(query, args.Elements()[1:]...)
	}

	if e2 != nil {
		return data.NewList(nil, errors.NewError(e2)), errors.NewError(e2)
	}

	result := data.NewStruct(rowsType).FromBuiltinPackage()
	result.SetAlways(rowsFieldName, rows)
	result.SetAlways(clientFieldName, db)
	result.SetAlways(dbFieldName, this)
	result.SetReadonly(true)

	return data.NewList(result, err), err
}

// queryResult executes a query, with optional parameter substitution, and returns the
// entire result set as an array in a single operation.
func queryResult(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if args.Len() == 0 {
		return nil, errors.ErrArgumentCount
	}

	db, tx, err := client(s)
	if err != nil {
		return data.NewList(nil, err), err
	}

	this := getThis(s)
	asStruct := data.Bool(this.GetAlways(asStructFieldName))
	this.SetAlways(rowCountFieldName, -1)

	var rows *sql.Rows

	var e2 error

	query := data.String(args.Get(0))
	ui.Log(ui.DBLogger, "Query: %s", query)

	if tx == nil {
		ui.Log(ui.DBLogger, "Query: %s", query)

		rows, e2 = db.Query(query, args.Elements()[1:]...)
	} else {
		ui.Log(ui.DBLogger, "(Tx) Query: %s", query)

		rows, e2 = tx.Query(query, args.Elements()[1:]...)
	}

	if rows != nil {
		defer rows.Close()
	}

	if e2 != nil {
		return data.NewList(nil, errors.NewError(e2)), errors.NewError(e2)
	}

	arrayResult := make([][]interface{}, 0)
	mapResult := make([]map[string]interface{}, 0)
	columns, _ := rows.Columns()
	colTypes, _ := rows.ColumnTypes()
	colCount := len(columns)

	for rows.Next() {
		rowTemplate := make([]interface{}, colCount)
		rowValues := make([]interface{}, colCount)

		for i := range colTypes {
			rowTemplate[i] = &rowValues[i]
		}

		if err := rows.Scan(rowTemplate...); err != nil {
			return nil, errors.NewError(err)
		}

		if asStruct {
			rowMap := map[string]interface{}{}

			for i, v := range columns {
				rowMap[v] = rowValues[i]
			}

			mapResult = append(mapResult, rowMap)
		} else {
			arrayResult = append(arrayResult, rowValues)
		}
	}

	size := len(arrayResult)

	if asStruct {
		size = len(mapResult)
	}

	ui.Log(ui.DBLogger, "Scanned %d rows, asStruct=%v", size, asStruct)

	if err := rows.Close(); err != nil {
		return data.NewList(nil, errors.NewError(err)), errors.NewError(err)
	}

	// Rows.Err will report the last error encountered by Rows.Scan.
	if err := rows.Err(); err != nil {
		return data.NewList(nil, errors.NewError(err)), errors.NewError(err)
	}

	// Need to convert the results from a slice to an actual array
	this.SetAlways(rowCountFieldName, size)
	r := data.NewArray(data.InterfaceType, size)

	if asStruct {
		for i, v := range mapResult {
			r.SetAlways(i, data.NewStructFromMap(v))
		}
	} else {
		for i, v := range arrayResult {
			rv := data.NewArrayFromInterfaces(data.InterfaceType, v...)
			r.SetAlways(i, rv)
		}
	}

	return data.NewList(r, err), err
}

// execute executes a SQL statement, and returns the number of rows that were
// affected by the statement (such as number of rows deleted for a DELETE statement).
func execute(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if args.Len() == 0 {
		return nil, errors.ErrArgumentCount
	}

	db, tx, e2 := client(s)
	if e2 != nil {
		return nil, e2
	}

	var sqlResult sql.Result

	var err error

	query := data.String(args.Get(0))

	ui.Log(ui.DBLogger, "Executing: %s", query)

	if tx == nil {
		ui.Log(ui.DBLogger, "Execute: %s", query)

		sqlResult, err = db.Exec(query, args.Elements()[1:]...)
	} else {
		ui.Log(ui.DBLogger, "(Tx) Execute: %s", query)

		sqlResult, err = tx.Exec(query, args.Elements()[1:]...)
	}

	if err != nil {
		return nil, errors.NewError(err)
	}

	r, err := sqlResult.RowsAffected()
	this := getThis(s)
	this.SetAlways(rowCountFieldName, int(r))

	ui.Log(ui.DBLogger, "%d rows affected", r)

	if err != nil {
		err = errors.NewError(err)
	}

	return data.NewList(int(r), err), err
}
