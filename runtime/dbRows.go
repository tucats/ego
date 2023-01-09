package runtime

import (
	"database/sql"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
)

var dbRowsTypeDef *data.Type
var dbRowsTypeDefLock sync.Mutex

func initDBRowsTypeDef() {
	dbRowsTypeDefLock.Lock()
	defer dbRowsTypeDefLock.Unlock()

	if dbRowsTypeDef == nil {
		t, _ := compiler.CompileTypeSpec(dbRowsTypeSpec)
		t.DefineFunction("Next", rowsNext)
		t.DefineFunction("Scan", rowsScan)
		t.DefineFunction("Close", rowsClose)
		t.DefineFunction("Headings", rowsHeadings)

		dbRowsTypeDef = t
	}
}

// DBQueryRows executes a query, with optional parameter substitution, and returns row object
// for subsequent calls to fetch the data.
func DBQueryRows(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	db, tx, err := getDBClient(s)
	if err != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, err}}, err
	}

	this := getThisStruct(s)
	this.SetAlways(rowCountFieldName, -1)

	query := data.String(args[0])

	var rows *sql.Rows

	var e2 error

	if tx == nil {
		ui.Debug(ui.DBLogger, "QueryRows: %s", query)

		rows, e2 = db.Query(query, args[1:]...)
	} else {
		ui.Debug(ui.DBLogger, "(Tx) QueryRows: %s", query)

		rows, e2 = tx.Query(query, args[1:]...)
	}

	if e2 != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, errors.EgoError(e2)}}, errors.EgoError(e2)
	}

	initDBRowsTypeDef()

	result := data.NewStruct(dbRowsTypeDef)
	result.SetAlways(rowsFieldName, rows)
	result.SetAlways(clientFieldName, db)
	result.SetAlways(dbFieldName, this)
	result.SetReadonly(true)

	return functions.MultiValueReturn{Value: []interface{}{result, err}}, err
}

func rowsClose(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	this := getThisStruct(s)
	rows := this.GetAlways(rowsFieldName).(*sql.Rows)

	err := rows.Close()

	this.SetAlways(rowsFieldName, nil)
	this.SetAlways(clientFieldName, nil)

	ui.Debug(ui.DBLogger, "rows.Close() called")

	return err, nil
}

func rowsHeadings(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	this := getThisStruct(s)
	rows := this.GetAlways(rowsFieldName).(*sql.Rows)
	result := make([]interface{}, 0)

	columns, err := rows.Columns()
	if err == nil {
		for _, name := range columns {
			result = append(result, name)
		}
	}

	if err != nil {
		err = errors.EgoError(err)
	}

	return result, err
}

func rowsNext(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	this := getThisStruct(s)
	if this == nil {
		return nil, errors.EgoError(errors.ErrNoFunctionReceiver)
	}

	rows := this.GetAlways(rowsFieldName).(*sql.Rows)
	active := rows.Next()

	ui.Debug(ui.DBLogger, "rows.Next() = %v", active)

	return active, nil
}

func rowsScan(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.EgoError(errors.ErrArgumentCount)
	}

	this := getThisStruct(s)
	if this == nil {
		return nil, errors.EgoError(errors.ErrNoFunctionReceiver)
	}

	rows := this.GetAlways(rowsFieldName).(*sql.Rows)
	db := this.GetAlways(dbFieldName).(*data.EgoStruct)
	asStruct := data.Bool(db.GetAlways(asStructFieldName))
	columns, _ := rows.Columns()
	colTypes, _ := rows.ColumnTypes()
	colCount := len(columns)
	rowTemplate := make([]interface{}, colCount)
	rowValues := make([]interface{}, colCount)

	for i := range colTypes {
		rowTemplate[i] = &rowValues[i]
	}

	if err := rows.Scan(rowTemplate...); err != nil {
		return functions.MultiValueReturn{Value: []interface{}{nil, errors.EgoError(err)}}, errors.EgoError(err)
	}

	if asStruct {
		rowMap := map[string]interface{}{}

		for i, v := range columns {
			rowMap[v] = rowValues[i]
		}

		return functions.MultiValueReturn{Value: []interface{}{data.NewMapFromMap(rowMap), nil}}, nil
	}

	return functions.MultiValueReturn{Value: []interface{}{data.NewArrayFromArray(&data.InterfaceType, rowValues), nil}}, nil
}
