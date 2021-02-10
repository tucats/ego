package runtime

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"

	_ "github.com/lib/pq"
)

// DBNew implements the New() db function. This allocated a new structure that
// contains all the info needed to call the database, including the function pointers
// for the functions available to a specific handle.
func DBNew(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ArgumentCountError)
	}

	// Get the connection string, which MUST be in URL format.
	connStr := util.GetString(args[0])

	url, err := url.Parse(connStr)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// If there was a password specified in the URL, blank it out now before we log it.
	if pstr, found := url.User.Password(); found {
		connStr = strings.ReplaceAll(connStr, ":"+pstr+"@", ":"+strings.Repeat("*", len(pstr))+"@")
	}

	ui.Debug(ui.DBLogger, "Connecting to %s", connStr)

	result := map[string]interface{}{
		"client":      db,
		"AsStruct":    DBAsStruct,
		"Begin":       DBBegin,
		"Commit":      DBCommit,
		"Rollback":    DBRollback,
		"Query":       DBQueryRows,
		"QueryResult": DBQuery,
		"Execute":     DBExecute,
		"Close":       DBClose,
		"constr":      connStr,
		"asStruct":    false,
		"rowCount":    0,
		"transaction": nil,
	}

	datatypes.SetMetadata(result, datatypes.ReadonlyMDKey, true)
	datatypes.SetMetadata(result, datatypes.TypeMDKey, "database")

	return result, nil
}

// DBBegin implements the Begin() db function. This allocated a new structure that
// contains all the info needed to call the database, including the function pointers
// for the functions available to a specific handle.
func DBBegin(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var tx *sql.Tx

	d, tx, err := getDBClient(s)
	if err == nil {
		this := getThis(s)

		if tx == nil {
			tx, err = d.Begin()
			if err == nil {
				this["transaction"] = tx
			}
		} else {
			err = errors.New("transaction already active")
		}
	}

	return nil, err
}

// DBCommit implements the Commit() db function.
func DBRollback(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var tx *sql.Tx

	_, tx, err := getDBClient(s)
	if err == nil {
		this := getThis(s)

		if tx != nil {
			err = tx.Rollback()
		} else {
			err = errors.New("no transaction active")
		}

		this["transaction"] = nil
	}

	return nil, err
}

// DBCommit implements the Commit() db function.
func DBCommit(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var tx *sql.Tx

	_, tx, err := getDBClient(s)
	if err == nil {
		this := getThis(s)

		if tx != nil {
			err = tx.Commit()
		} else {
			err = errors.New("no transaction active")
		}

		this["transaction"] = nil
	}

	return nil, err
}

// DBAsStruct sets the asStruct flag. When true, result sets from queries are an array
// of structs, where the struct members are the same as the result set column names. When
// not true, the result set is an array of arrays, where the inner array contains the
// column data in the order of the result set, but with no labels, etc.
func DBAsStruct(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ArgumentCountError)
	}

	_, _, err := getDBClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)
	this["asStruct"] = util.GetBool(args[0])

	return this, nil
}

// DBClose closes the database connection, frees up any resources held, and resets the
// handle contents to prevent re-using the connection.
func DBClose(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	_, tx, err := getDBClient(s)
	if err != nil {
		return nil, err
	}

	if tx != nil {
		err = tx.Rollback()
	}

	this := getThis(s)
	this["client"] = nil
	this["AsStruct"] = dbReleased
	this["Query"] = dbReleased
	this["QueryRows"] = dbReleased
	this["Execute"] = dbReleased
	this["constr"] = ""
	this["transaction"] = nil
	this["asStruct"] = false
	this["rowCount"] = -1

	return true, err
}

// DBQuery executes a query, with optional parameter substitution, and returns the
// entire result set as an array.
func DBQuery(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	db, tx, err := getDBClient(s)
	if err != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, err}}, err
	}

	this := getThis(s)
	asStruct := util.GetBool(this["asStruct"])
	this["rowCount"] = -1

	var rows *sql.Rows

	query := util.GetString(args[0])
	ui.Debug(ui.DBLogger, "Query: %s", query)

	if tx == nil {
		ui.Debug(ui.DBLogger, "Query: %s", query)

		rows, err = db.Query(query, args[1:]...)
	} else {
		ui.Debug(ui.DBLogger, "(Tx) Query: %s", query)

		rows, err = tx.Query(query, args[1:]...)
	}

	if rows != nil {
		defer rows.Close()
	}

	if err != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, err}}, err
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
			return nil, err
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
		return functions.MultiValueReturn{Value: []interface{}{nil, err}}, err
	}

	// Rows.Err will report the last error encountered by Rows.Scan.
	if err := rows.Err(); err != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, err}}, err
	}

	// Need to convert the results from a slice to an actual array
	this["rowCount"] = size
	r := make([]interface{}, size)

	if asStruct {
		for i, v := range mapResult {
			r[i] = v
		}
	} else {
		for i, v := range arrayResult {
			r[i] = v
		}
	}

	return functions.MultiValueReturn{Value: []interface{}{r, err}}, err
}

// DBQueryRows executes a query, with optional parameter substitution, and returns row object
// for subsequent calls to fetch the data.
func DBQueryRows(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	db, tx, err := getDBClient(s)
	if err != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, err}}, err
	}

	this := getThis(s)
	this["rowCount"] = -1
	query := util.GetString(args[0])

	var rows *sql.Rows

	if tx == nil {
		ui.Debug(ui.DBLogger, "QueryRows: %s", query)

		rows, err = db.Query(query, args[1:]...)
	} else {
		ui.Debug(ui.DBLogger, "(Tx) QueryRows: %s", query)

		rows, err = tx.Query(query, args[1:]...)
	}

	if err != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, err}}, err
	}

	result := map[string]interface{}{}
	result["rows"] = rows
	result["client"] = db
	result["db"] = this
	result["Next"] = rowsNext
	result["Scan"] = rowsScan
	result["Close"] = rowsClose
	result["Headings"] = rowsHeadings
	datatypes.SetMetadata(result, datatypes.ReadonlyMDKey, true)
	datatypes.SetMetadata(result, datatypes.TypeMDKey, "rows")

	return functions.MultiValueReturn{Value: []interface{}{result, err}}, err
}

func rowsClose(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	this := getThis(s)
	rows := this["rows"].(*sql.Rows)

	err := rows.Close()

	this["rows"] = nil
	this["client"] = nil
	this["Next"] = dbReleased
	this["Scan"] = dbReleased
	this["Headings"] = dbReleased

	ui.Debug(ui.DBLogger, "rows.Close() called")

	return err, nil
}

func rowsHeadings(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	this := getThis(s)
	rows := this["rows"].(*sql.Rows)
	result := make([]interface{}, 0)

	columns, err := rows.Columns()
	if err == nil {
		for _, name := range columns {
			result = append(result, name)
		}
	}

	return result, err
}

func rowsNext(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	this := getThis(s)
	rows := this["rows"].(*sql.Rows)
	active := rows.Next()

	ui.Debug(ui.DBLogger, "rows.Next() = %v", active)

	return active, nil
}

func rowsScan(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	this := getThis(s)
	rows := this["rows"].(*sql.Rows)
	db := this["db"].(map[string]interface{})
	asStruct := util.GetBool(db["asStruct"])
	columns, _ := rows.Columns()
	colTypes, _ := rows.ColumnTypes()
	colCount := len(columns)
	rowTemplate := make([]interface{}, colCount)
	rowValues := make([]interface{}, colCount)

	for i := range colTypes {
		rowTemplate[i] = &rowValues[i]
	}

	if err := rows.Scan(rowTemplate...); err != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, err}}, err
	}

	if asStruct {
		rowMap := map[string]interface{}{}

		for i, v := range columns {
			rowMap[v] = rowValues[i]
		}

		return functions.MultiValueReturn{Value: []interface{}{rowMap, nil}}, nil
	}

	return functions.MultiValueReturn{Value: []interface{}{rowValues, nil}}, nil
}

// DBExecute executes a SQL statement, and returns the number of rows that were
// affected by the statement (such as number of rows deleted for a DELETE statement).
func DBExecute(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	db, tx, e2 := getDBClient(s)
	if e2 != nil {
		return nil, e2
	}

	var sqlResult sql.Result

	var err error

	query := util.GetString(args[0])

	ui.Debug(ui.DBLogger, "Executing: %s", query)

	if tx == nil {
		ui.Debug(ui.DBLogger, "Execute: %s", query)

		sqlResult, err = db.Exec(query, args[1:]...)
	} else {
		ui.Debug(ui.DBLogger, "(Tx) Execute: %s", query)

		sqlResult, err = tx.Exec(query, args[1:]...)
	}

	if err != nil {
		return nil, errors.New(err)
	}

	r, err := sqlResult.RowsAffected()
	this := getThis(s)
	this["rowCount"] = int(r)

	ui.Debug(ui.DBLogger, "%d rows affected", r)

	return functions.MultiValueReturn{Value: []interface{}{int(r), err}}, errors.New(err)
}

// getClient searches the symbol table for the client receiver ("__this")
// variable, validates that it contains a database client object, and returns
// the native client object.
func getDBClient(symbols *symbols.SymbolTable) (*sql.DB, *sql.Tx, *errors.EgoError) {
	if g, ok := symbols.Get("__this"); ok {
		if gc, ok := g.(map[string]interface{}); ok {
			if client, ok := gc["client"]; ok {
				if cp, ok := client.(*sql.DB); ok {
					if cp == nil {
						return nil, nil, errors.New(errors.DatabaseClientClosedError)
					}

					tx := gc["transaction"]
					if tx == nil {
						return cp, nil, nil
					} else {
						return cp, tx.(*sql.Tx), nil
					}
				}
			}
		}
	}

	return nil, nil, errors.New(errors.NoFunctionReceiver)
}

// Utility function that becomes the db handle function pointer for a closed
// db connection handle.
func dbReleased(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return nil, errors.New(errors.DatabaseClientClosedError)
}
