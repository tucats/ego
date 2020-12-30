package runtime

import (
	"database/sql"
	"errors"
	"net/url"
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/functions"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"

	_ "github.com/lib/pq"
)

// DBNew implements the New() rest function
func DBNew(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	if len(args) != 1 {
		return nil, errors.New(defs.IncorrectArgumentCount)
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
	return map[string]interface{}{
		"client":     db,
		"Query":      DBQuery,
		"Execute":    DBExecute,
		"Close":      DBClose,
		"constr":     connStr,
		"status":     0,
		"__readonly": true,
	}, nil
}

func DBClose(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	_, err := getDBClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)
	this["client"] = nil
	this["Query"] = dbReleased
	this["Execute"] = dbReleased
	this["constr"] = ""
	this["status"] = -1

	return true, nil
}

func DBQuery(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	db, err := getDBClient(s)
	if err != nil {
		return nil, err
	}

	var rows *sql.Rows
	query := util.GetString(args[0])
	ui.Debug(ui.DBLogger, "Query: %s", query)
	if len(args) == 1 {
		rows, err = db.Query(query)
	} else {
		rows, err = db.Query(query, args[1:]...)
	}
	if rows != nil {
		defer rows.Close()
	}
	if err != nil {
		return nil, err
	}

	result := make([][]interface{}, 0)

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
		result = append(result, rowValues)
	}
	ui.Debug(ui.DBLogger, "Scanned %d rows", len(result))

	rerr := rows.Close()
	if rerr != nil {
		return nil, err
	}

	// Rows.Err will report the last error encountered by Rows.Scan.
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Need to convert the results from a slice to an actual array
	r := make([]interface{}, len(result))
	for i, v := range result {
		r[i] = v
	}
	return functions.MultiValueReturn{Value: []interface{}{r, err}}, err

}

func DBExecute(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	db, err := getDBClient(s)
	if err != nil {
		return nil, err
	}

	var sqlResult sql.Result
	query := util.GetString(args[0])
	ui.Debug(ui.DBLogger, "Executing: %s", query)
	if len(args) == 1 {
		sqlResult, err = db.Exec(query)
	} else {
		sqlResult, err = db.Exec(query, args[1:]...)
	}
	if err != nil {
		return nil, err
	}
	r, err := sqlResult.RowsAffected()
	ui.Debug(ui.DBLogger, "%d rows affected", r)

	return functions.MultiValueReturn{Value: []interface{}{r, err}}, err
}

// getClient searches the symbol table for the client receiver ("_this")
// variable, validates that it contains a REST client object, and returns
// the native client object.
func getDBClient(symbols *symbols.SymbolTable) (*sql.DB, error) {
	if g, ok := symbols.Get("_this"); ok {
		if gc, ok := g.(map[string]interface{}); ok {
			if client, ok := gc["client"]; ok {
				if cp, ok := client.(*sql.DB); ok {
					if cp == nil {
						return nil, errors.New("db client was closed")
					}
					return cp, nil
				}
			}
		}
	}

	return nil, errors.New(defs.NoFunctionReceiver)

}

func dbReleased(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return nil, errors.New("db client closed")
}
