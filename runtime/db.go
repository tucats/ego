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
	_ "github.com/mattn/go-sqlite3"
)

var dbTypeDef *datatypes.Type

func initDBTypeDef() {
	if dbTypeDef == nil {
		t := datatypes.Structure()
		t.DefineField(clientFieldName, datatypes.InterfaceType)
		t.DefineField(asStructFieldName, datatypes.BoolType)
		t.DefineField(rowCountFieldName, datatypes.IntType)
		t.DefineField(transactionFieldName, datatypes.InterfaceType)
		t.DefineField(constrFieldName, datatypes.StringType)

		t.DefineFunction(asStructFieldName, DataBaseAsStruct)

		t.DefineFunction("Begin", DBBegin)
		t.DefineFunction("Commit", DBCommit)
		t.DefineFunction("Rollback", DBRollback)
		t.DefineFunction("Query", DBQueryRows)
		t.DefineFunction("QueryResult", DBQuery)
		t.DefineFunction("Execute", DBExecute)
		t.DefineFunction("Close", DBClose)
		t.DefineFunction("AsStruct", DataBaseAsStruct)

		typeDef := datatypes.TypeDefinition(databaseTypeDefinitionName, t)
		dbTypeDef = &typeDef
	}
}

// DBNew implements the New() db function. This allocated a new structure that
// contains all the info needed to call the database, including the function pointers
// for the functions available to a specific handle.
func DBNew(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ArgumentCountError)
	}

	initDBTypeDef()

	// Get the connection string, which MUST be in URL format.
	connStr := util.GetString(args[0])

	url, err := url.Parse(connStr)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	scheme := url.Scheme
	if scheme == "sqlite3" {
		connStr = strings.TrimPrefix(connStr, scheme+"://")
	}

	db, err := sql.Open(url.Scheme, connStr)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	// If there was a password specified in the URL, blank it out now before we log it.
	if secretString, found := url.User.Password(); found {
		connStr = strings.ReplaceAll(connStr, ":"+secretString+"@", ":"+strings.Repeat("*", len(secretString))+"@")
	}

	ui.Debug(ui.DBLogger, "Connecting to %s", connStr)

	result := datatypes.NewStruct(*dbTypeDef)
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
func DBBegin(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var tx *sql.Tx

	d, tx, err := getDBClient(s)
	if errors.Nil(err) {
		this := getThisStruct(s)

		if tx == nil {
			var e2 error

			tx, e2 = d.Begin()
			if e2 == nil {
				this.SetAlways(transactionFieldName, tx)
			}
		} else {
			err = errors.New(errors.TransactionAlreadyActive)
		}
	}

	return nil, err
}

// DBCommit implements the Commit() db function.
func DBRollback(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var tx *sql.Tx

	_, tx, err := getDBClient(s)
	if errors.Nil(err) {
		this := getThisStruct(s)

		if tx != nil {
			err = errors.New(tx.Rollback())
		} else {
			err = errors.New(errors.NoTransactionActiveError)
		}

		this.SetAlways(transactionFieldName, nil)
	}

	return nil, err
}

// DBCommit implements the Commit() db function.
func DBCommit(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var tx *sql.Tx

	_, tx, err := getDBClient(s)
	if errors.Nil(err) {
		this := getThisStruct(s)

		if tx != nil {
			err = errors.New(tx.Commit())
		} else {
			err = errors.New(errors.NoTransactionActiveError)
		}

		this.SetAlways(transactionFieldName, nil)
	}

	return nil, err
}

// DataBaseAsStruct sets the asStruct flag. When true, result sets from queries are an array
// of structs, where the struct members are the same as the result set column names. When
// not true, the result set is an array of arrays, where the inner array contains the
// column data in the order of the result set, but with no labels, etc.
func DataBaseAsStruct(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ArgumentCountError)
	}

	_, _, err := getDBClient(s)
	if !errors.Nil(err) {
		return nil, err
	}

	this := getThisStruct(s)
	this.SetAlways(asStructFieldName, util.GetBool(args[0]))

	return this, nil
}

// DBClose closes the database connection, frees up any resources held, and resets the
// handle contents to prevent re-using the connection.
func DBClose(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	_, tx, err := getDBClient(s)
	if !errors.Nil(err) {
		return nil, err
	}

	if tx != nil {
		err = errors.New(tx.Rollback())
	}

	this := getThisStruct(s)
	this.SetAlways(clientFieldName, nil)
	this.SetAlways(constrFieldName, "")
	this.SetAlways(transactionFieldName, nil)
	this.SetAlways(asStructFieldName, false)
	this.SetAlways(rowCountFieldName, -1)

	return true, err
}

// DBQuery executes a query, with optional parameter substitution, and returns the
// entire result set as an array.
func DBQuery(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	db, tx, err := getDBClient(s)
	if !errors.Nil(err) {
		return functions.MultiValueReturn{Value: []interface{}{nil, err}}, err
	}

	this := getThisStruct(s)
	asStruct := util.GetBool(this.GetAlways(asStructFieldName))
	this.SetAlways(rowCountFieldName, -1)

	var rows *sql.Rows

	var e2 error

	query := util.GetString(args[0])
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
		return functions.MultiValueReturn{Value: []interface{}{nil, errors.New(err)}}, errors.New(err)
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

		if err := rows.Scan(rowTemplate...); !errors.Nil(err) {
			return nil, errors.New(err)
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

	if err := rows.Close(); !errors.Nil(err) {
		return functions.MultiValueReturn{Value: []interface{}{nil, errors.New(err)}}, errors.New(err)
	}

	// Rows.Err will report the last error encountered by Rows.Scan.
	if err := rows.Err(); !errors.Nil(err) {
		return functions.MultiValueReturn{Value: []interface{}{nil, errors.New(err)}}, errors.New(err)
	}

	// Need to convert the results from a slice to an actual array
	this.SetAlways(rowCountFieldName, size)
	r := datatypes.NewArray(datatypes.InterfaceType, size)

	if asStruct {
		for i, v := range mapResult {
			r.SetAlways(i, datatypes.NewStructFromMap(v))
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

	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	r, err := sqlResult.RowsAffected()
	this := getThisStruct(s)
	this.SetAlways(rowCountFieldName, int(r))

	ui.Debug(ui.DBLogger, "%d rows affected", r)

	return functions.MultiValueReturn{Value: []interface{}{int(r), err}}, errors.New(err)
}

// getClient searches the symbol table for the client receiver ("__this")
// variable, validates that it contains a database client object, and returns
// the native client object.
func getDBClient(symbols *symbols.SymbolTable) (*sql.DB, *sql.Tx, *errors.EgoError) {
	if g, ok := symbols.Get("__this"); ok {
		if gc, ok := g.(*datatypes.EgoStruct); ok {
			if client, ok := gc.Get(clientFieldName); ok {
				if cp, ok := client.(*sql.DB); ok {
					if cp == nil {
						return nil, nil, errors.New(errors.DatabaseClientClosedError)
					}

					tx := gc.GetAlways(transactionFieldName)
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
