package runtime

import (
	"database/sql"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

var dbRowsTypeDef *datatypes.Type

func initDBRowsTypeDef() {
	if dbRowsTypeDef == nil {
		t := datatypes.Structure()
		_ = t.DefineField(clientFieldName, datatypes.InterfaceType)
		_ = t.DefineField(rowsFieldName, datatypes.InterfaceType)
		_ = t.DefineField(dbFieldName, datatypes.InterfaceType)

		t.DefineFunction(asStructFieldName, DataBaseAsStruct)

		t.DefineFunction("Next", rowsNext)
		t.DefineFunction("Scan", rowsScan)
		t.DefineFunction("Close", rowsClose)
		t.DefineFunction("Headings", rowsHeadings)

		typeDef := datatypes.TypeDefinition(databaseRowsTypeDefinitionName, t)
		dbRowsTypeDef = &typeDef
	}
}

// DBQueryRows executes a query, with optional parameter substitution, and returns row object
// for subsequent calls to fetch the data.
func DBQueryRows(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	db, tx, err := getDBClient(s)
	if !errors.Nil(err) {
		return functions.MultiValueReturn{Value: []interface{}{nil, err}}, err
	}

	this := getThisStruct(s)
	this.SetAlways(rowCountFieldName, -1)

	query := util.GetString(args[0])

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
		return functions.MultiValueReturn{Value: []interface{}{nil, errors.New(e2)}}, errors.New(e2)
	}

	initDBRowsTypeDef()

	result := datatypes.NewStruct(*dbRowsTypeDef)
	result.SetAlways(rowsFieldName, rows)
	result.SetAlways(clientFieldName, db)
	result.SetAlways(dbFieldName, this)
	result.SetReadonly(true)

	return functions.MultiValueReturn{Value: []interface{}{result, err}}, err
}

func rowsClose(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	this := getThisStruct(s)
	rows := this.GetAlways(rowsFieldName).(*sql.Rows)

	err := rows.Close()

	this.SetAlways(rowsFieldName, nil)
	this.SetAlways(clientFieldName, nil)

	ui.Debug(ui.DBLogger, "rows.Close() called")

	return err, nil
}

func rowsHeadings(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	this := getThisStruct(s)
	rows := this.GetAlways(rowsFieldName).(*sql.Rows)
	result := make([]interface{}, 0)

	columns, err := rows.Columns()
	if errors.Nil(err) {
		for _, name := range columns {
			result = append(result, name)
		}
	}

	return result, errors.New(err)
}

func rowsNext(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	this := getThisStruct(s)
	if this == nil {
		return nil, errors.New(errors.NoFunctionReceiver)
	}

	rows := this.GetAlways(rowsFieldName).(*sql.Rows)
	active := rows.Next()

	ui.Debug(ui.DBLogger, "rows.Next() = %v", active)

	return active, nil
}

func rowsScan(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	this := getThisStruct(s)
	if this == nil {
		return nil, errors.New(errors.NoFunctionReceiver)
	}

	rows := this.GetAlways(rowsFieldName).(*sql.Rows)
	db := this.GetAlways(dbFieldName).(*datatypes.EgoStruct)
	asStruct := util.GetBool(db.GetAlways(asStructFieldName))
	columns, _ := rows.Columns()
	colTypes, _ := rows.ColumnTypes()
	colCount := len(columns)
	rowTemplate := make([]interface{}, colCount)
	rowValues := make([]interface{}, colCount)

	for i := range colTypes {
		rowTemplate[i] = &rowValues[i]
	}

	if err := rows.Scan(rowTemplate...); !errors.Nil(err) {
		return functions.MultiValueReturn{Value: []interface{}{nil, errors.New(err)}}, errors.New(err)
	}

	if asStruct {
		rowMap := map[string]interface{}{}

		for i, v := range columns {
			rowMap[v] = rowValues[i]
		}

		return functions.MultiValueReturn{Value: []interface{}{datatypes.NewMapFromMap(rowMap), nil}}, nil
	}

	return functions.MultiValueReturn{Value: []interface{}{datatypes.NewFromArray(datatypes.InterfaceType, rowValues), nil}}, nil
}
