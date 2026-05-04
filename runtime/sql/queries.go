package sql

import (
	goSQL "database/sql"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// query implements db.Client.Query(sql string, args ...any). It executes the
// given SQL SELECT statement and returns a db.Rows cursor struct wrapped in a
// data.List: [rows *data.Struct, err error].
//
// Optional positional arguments (args[1:]) are passed as query parameters to
// the driver, providing safe parameter substitution (equivalent to "?" or "$N"
// placeholders depending on the driver).
//
// When a transaction is active on the Client struct, the query runs inside
// that transaction. Note: the non-tx path uses args.Elements()[1:args.Len()]
// while the tx path uses args.Elements()[1:] — these are functionally
// identical since Len() == len(elements), but the inconsistency is a minor
// readability issue.
//
// Returns ErrArgumentCount when called with no arguments.
func query(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		rows *goSQL.Rows
		e2   error
	)

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

	if tx == nil {
		ui.Log(ui.DBLogger, "db.query.rows", ui.A{
			"sql": query})

		rows, e2 = db.Query(query, args.Elements()[1:]...)
	} else {
		ui.Log(ui.DBLogger, "db.tx.query.rows", ui.A{
			"sql": query})

		rows, e2 = tx.Query(query, args.Elements()[1:]...)
	}

	if e2 != nil {
		return data.NewList(nil, errors.New(e2)), errors.New(e2)
	}

	result := data.NewStruct(RowsType).FromBuiltinPackage()
	result.SetAlways(rowsFieldName, rows)
	result.SetAlways(clientFieldName, db)
	result.SetAlways(dbFieldName, this)
	result.SetReadonly(true)

	return data.NewList(result, err), err
}

// queryResult implements goSQL.Database.QueryResult(sql string, args ...any). It
// executes the given SQL SELECT and eagerly fetches the entire result set,
// returning it as a data.List: [rows *data.Array, err error].
//
// The shape of each element in the array depends on the asStruct flag on the
// Client struct (set via AsStruct()):
//
//   - false (default) — each element is a *data.Array of column values in
//     SELECT-list order; row[0] is the first column, etc.
//   - true            — each element is a *data.Struct whose field names
//     match the column names from the result set metadata
//
// Optional positional arguments (args[1:]) are passed as query parameters.
// The underlying *goSQL.Rows cursor is closed automatically before this
// function returns.
//
// Returns ErrArgumentCount when called with no arguments.
func queryResult(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		rows *goSQL.Rows
		e2   error
	)

	if args.Len() == 0 {
		return nil, errors.ErrArgumentCount
	}

	db, tx, err := client(s)
	if err != nil {
		return data.NewList(nil, err), err
	}

	this := getThis(s)
	asStruct := data.BoolOrFalse(this.GetAlways(asStructFieldName))
	this.SetAlways(rowCountFieldName, -1)

	query := data.String(args.Get(0))

	if tx == nil {
		ui.Log(ui.DBLogger, "db.query.rows", ui.A{
			"sql": query})

		rows, e2 = db.Query(query, args.Elements()[1:]...)
	} else {
		ui.Log(ui.DBLogger, "db.tx.query.rows", ui.A{
			"sql": query})

		rows, e2 = tx.Query(query, args.Elements()[1:]...)
	}

	if rows != nil {
		defer rows.Close()
	}

	if e2 != nil {
		return data.NewList(nil, errors.New(e2)), errors.New(e2)
	}

	arrayResult := make([][]any, 0)
	mapResult := make([]map[string]any, 0)
	columns, _ := rows.Columns()
	colTypes, _ := rows.ColumnTypes()
	colCount := len(columns)

	for rows.Next() {
		rowTemplate := make([]any, colCount)
		rowValues := make([]any, colCount)

		for i := range colTypes {
			rowTemplate[i] = &rowValues[i]
		}

		if err := rows.Scan(rowTemplate...); err != nil {
			return nil, errors.New(err)
		}

		if asStruct {
			rowMap := map[string]any{}

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

	ui.Log(ui.DBLogger, "db.scan", ui.A{
		"count": size,
		"flag":  asStruct})

	if err := rows.Close(); err != nil {
		return data.NewList(nil, errors.New(err)), errors.New(err)
	}

	// Rows.Err will report the last error encountered by Rows.Scan.
	if err := rows.Err(); err != nil {
		return data.NewList(nil, errors.New(err)), errors.New(err)
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

// execute implements db.Client.Execute(sql string, args ...any). It runs a
// non-SELECT SQL statement (INSERT, UPDATE, DELETE, CREATE TABLE, etc.) and
// returns a data.List: [rowsAffected int, err error].
//
// rowsAffected is the count reported by the driver's RowsAffected() method.
// DDL statements (CREATE, DROP, ALTER) typically report 0.
//
// When a transaction is active on the Client struct, the statement runs inside
// that transaction. Optional positional arguments (args[1:]) are passed as
// query parameters.
//
// Returns ErrArgumentCount when called with no arguments.
func execute(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		sqlResult goSQL.Result
		err       error
	)

	if args.Len() == 0 {
		return nil, errors.ErrArgumentCount
	}

	db, tx, e2 := client(s)
	if e2 != nil {
		return nil, e2
	}

	query := data.String(args.Get(0))

	if tx == nil {
		ui.Log(ui.DBLogger, "db.exec", ui.A{
			"sql": query})

		sqlResult, err = db.Exec(query, args.Elements()[1:]...)
	} else {
		ui.Log(ui.DBLogger, "db.tx.exec", ui.A{
			"sql": query})

		sqlResult, err = tx.Exec(query, args.Elements()[1:]...)
	}

	if err != nil {
		return nil, errors.New(err)
	}

	r, err := sqlResult.RowsAffected()
	this := getThis(s)
	this.SetAlways(rowCountFieldName, int(r))

	ui.Log(ui.DBLogger, "db.rows", ui.A{
		"count": r})

	if err != nil {
		err = errors.New(err)
	}

	return data.NewList(int(r), err), err
}
