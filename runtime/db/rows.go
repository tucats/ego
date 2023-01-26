package db

import (
	"database/sql"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func rowsClose(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	this := getThisStruct(s)
	rows := this.GetAlways(rowsFieldName).(*sql.Rows)

	err := rows.Close()

	this.SetAlways(rowsFieldName, nil)
	this.SetAlways(clientFieldName, nil)

	ui.Log(ui.DBLogger, "rows.Close() called")

	return err, nil
}

func rowsHeadings(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
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
		err = errors.NewError(err)
	}

	return result, err
}

func rowsNext(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	this := getThisStruct(s)
	if this == nil {
		return nil, errors.ErrNoFunctionReceiver
	}

	rows := this.GetAlways(rowsFieldName).(*sql.Rows)
	active := rows.Next()

	ui.Log(ui.DBLogger, "rows.Next() = %v", active)

	return active, nil
}

func rowsScan(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	this := getThisStruct(s)
	if this == nil {
		return nil, errors.ErrNoFunctionReceiver
	}

	rows := this.GetAlways(rowsFieldName).(*sql.Rows)
	db := this.GetAlways(dbFieldName).(*data.Struct)
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
		return data.List(nil, errors.NewError(err)), errors.NewError(err)
	}

	if asStruct {
		rowMap := map[string]interface{}{}

		for i, v := range columns {
			rowMap[v] = rowValues[i]
		}

		return data.List(data.NewMapFromMap(rowMap), nil), nil
	}

	return data.List(data.NewArrayFromArray(data.InterfaceType, rowValues), nil), nil
}
