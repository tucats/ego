package runtime

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"

	// Blank imports to make sure we link in the database drivers.
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// DBNew implements the New() db function. This allocated a new structure that
// contains all the info needed to call the database, including the function pointers
// for the functions available to a specific handle.
func DBNew(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	initDBTypeDef()

	// Get the connection string, which MUST be in URL format.
	connStr := data.String(args[0])

	url, err := url.Parse(connStr)
	if err != nil {
		return nil, errors.NewError(err)
	}

	if scheme := url.Scheme; scheme == "sqlite3" {
		connStr = strings.TrimPrefix(connStr, scheme+"://")
	}

	db, err := sql.Open(url.Scheme, connStr)
	if err != nil {
		return nil, errors.NewError(err)
	}

	// If there was a password specified in the URL, blank it out now before we log it.
	if secretString, found := url.User.Password(); found {
		connStr = strings.ReplaceAll(connStr, ":"+secretString+"@", ":"+strings.Repeat("*", len(secretString))+"@")
	}

	ui.Debug(ui.DBLogger, "Connecting to %s", connStr)

	_ = s.Set(dbTypeDef.Name(), dbTypeDef)

	result := data.NewStruct(dbTypeDef)
	result.SetAlways(clientFieldName, db)
	result.SetAlways(constrFieldName, connStr)
	result.SetAlways(asStructFieldName, false)
	result.SetAlways(rowCountFieldName, 0)
	result.SetReadonly(true)

	return result, nil
}

// DBBegin implements the Begin() db function. This allocated a new structure that
// contains all the info needed to call the database, including the function pointers
// for the functions available to a specific handle.
func DBBegin(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	var tx *sql.Tx

	d, tx, err := getDBClient(s)
	if err == nil {
		this := getThisStruct(s)

		if tx == nil {
			var e2 error

			tx, e2 = d.Begin()
			if e2 == nil {
				this.SetAlways(transactionFieldName, tx)
			}
		} else {
			err = errors.ErrTransactionAlreadyActive
		}
	}

	return nil, err
}

// DBRollback implements the Rollback() db function.
func DBRollback(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	var tx *sql.Tx

	_, tx, err := getDBClient(s)
	if err == nil {
		this := getThisStruct(s)

		if tx != nil {
			err = tx.Rollback()
		} else {
			err = errors.ErrNoTransactionActive
		}

		this.SetAlways(transactionFieldName, nil)
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return nil, err
}

// DBCommit implements the Commit() db function.
func DBCommit(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	var tx *sql.Tx

	_, tx, err := getDBClient(s)
	if err == nil {
		this := getThisStruct(s)

		if tx != nil {
			err = tx.Commit()
		} else {
			err = errors.ErrNoTransactionActive
		}

		this.SetAlways(transactionFieldName, nil)
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return nil, err
}

// DataBaseAsStruct sets the asStruct flag. When true, result sets from queries are an array
// of structs, where the struct members are the same as the result set column names. When
// not true, the result set is an array of arrays, where the inner array contains the
// column data in the order of the result set, but with no labels, etc.
func DataBaseAsStruct(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	_, _, err := getDBClient(s)
	if err != nil {
		return nil, err
	}

	this := getThisStruct(s)
	this.SetAlways(asStructFieldName, data.Bool(args[0]))

	return this, nil
}

// DBClose closes the database connection, frees up any resources held, and resets the
// handle contents to prevent re-using the connection.
func DBClose(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	_, tx, err := getDBClient(s)
	if err != nil {
		return nil, err
	}

	if tx != nil {
		err = tx.Rollback()
	}

	this := getThisStruct(s)
	this.SetAlways(clientFieldName, nil)
	this.SetAlways(constrFieldName, "")
	this.SetAlways(transactionFieldName, nil)
	this.SetAlways(asStructFieldName, false)
	this.SetAlways(rowCountFieldName, -1)

	if err != nil {
		err = errors.NewError(err)
	}

	return true, err
}

// DBQuery executes a query, with optional parameter substitution, and returns the
// entire result set as an array.
func DBQuery(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, errors.ErrArgumentCount
	}

	db, tx, err := getDBClient(s)
	if err != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, err}}, err
	}

	this := getThisStruct(s)
	asStruct := data.Bool(this.GetAlways(asStructFieldName))
	this.SetAlways(rowCountFieldName, -1)

	var rows *sql.Rows

	var e2 error

	query := data.String(args[0])
	ui.Debug(ui.DBLogger, "Query: %s", query)

	if tx == nil {
		ui.Debug(ui.DBLogger, "Query: %s", query)

		rows, e2 = db.Query(query, args[1:]...)
	} else {
		ui.Debug(ui.DBLogger, "(Tx) Query: %s", query)

		rows, e2 = tx.Query(query, args[1:]...)
	}

	if rows != nil {
		defer rows.Close()
	}

	if e2 != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, errors.NewError(e2)}}, errors.NewError(e2)
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

	ui.Debug(ui.DBLogger, "Scanned %d rows, asStruct=%v", size, asStruct)

	if err := rows.Close(); err != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, errors.NewError(err)}}, errors.NewError(err)
	}

	// Rows.Err will report the last error encountered by Rows.Scan.
	if err := rows.Err(); err != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, errors.NewError(err)}}, errors.NewError(err)
	}

	// Need to convert the results from a slice to an actual array
	this.SetAlways(rowCountFieldName, size)
	r := data.NewArray(&data.InterfaceType, size)

	if asStruct {
		for i, v := range mapResult {
			r.SetAlways(i, data.NewStructFromMap(v))
		}
	} else {
		for i, v := range arrayResult {
			r.SetAlways(i, v)
		}
	}

	return functions.MultiValueReturn{Value: []interface{}{r, err}}, err
}

// DBExecute executes a SQL statement, and returns the number of rows that were
// affected by the statement (such as number of rows deleted for a DELETE statement).
func DBExecute(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, errors.ErrArgumentCount
	}

	db, tx, e2 := getDBClient(s)
	if e2 != nil {
		return nil, e2
	}

	var sqlResult sql.Result

	var err error

	query := data.String(args[0])

	ui.Debug(ui.DBLogger, "Executing: %s", query)

	if tx == nil {
		ui.Debug(ui.DBLogger, "Execute: %s", query)

		sqlResult, err = db.Exec(query, args[1:]...)
	} else {
		ui.Debug(ui.DBLogger, "(Tx) Execute: %s", query)

		sqlResult, err = tx.Exec(query, args[1:]...)
	}

	if err != nil {
		return nil, errors.NewError(err)
	}

	r, err := sqlResult.RowsAffected()
	this := getThisStruct(s)
	this.SetAlways(rowCountFieldName, int(r))

	ui.Debug(ui.DBLogger, "%d rows affected", r)

	if err != nil {
		err = errors.NewError(err)
	}

	return functions.MultiValueReturn{Value: []interface{}{int(r), err}}, err
}

// getClient searches the symbol table for the client receiver ("__this")
// variable, validates that it contains a database client object, and returns
// the native client object.
func getDBClient(symbols *symbols.SymbolTable) (*sql.DB, *sql.Tx, error) {
	if g, ok := symbols.Get("__this"); ok {
		if gc, ok := g.(*data.Struct); ok {
			if client, ok := gc.Get(clientFieldName); ok {
				if cp, ok := client.(*sql.DB); ok {
					if cp == nil {
						return nil, nil, errors.ErrDatabaseClientClosed
					}

					tx := gc.GetAlways(transactionFieldName)
					if tx == nil {
						return cp, nil, nil
					}

					return cp, tx.(*sql.Tx), nil
				}
			}
		}
	}

	return nil, nil, errors.ErrNoFunctionReceiver
}
