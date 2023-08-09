package resources

import (
	"encoding/json"
	"reflect"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
)

// Read reads an array of objects from the underlying database. The objects
// can be filtered by passing filter objects as parameters to the read
// operation (with no filters, the call returns all objects in the table).
//
// The result is an array of interfaces, where each interface in the array
// can be cast back to the type of the object used to open the resource table.
// If an error occurs during the call, the error is returned and the interface
// array is nil.
func (r *ResHandle) Read(filters ...*Filter) ([]interface{}, error) {
	var (
		err     error
		results []interface{}
		count   int
	)

	if r.Database == nil {
		return nil, ErrDatabaseNotOpen
	}

	sql := r.readRowSQL()

	for index, filter := range filters {
		if index == 0 {
			sql = sql + " where "
		} else {
			sql = sql + " and "
		}

		sql = sql + filter.Generate()
	}

	// Add any active order-by clause
	sql = sql + r.OrderBy()

	ui.Log(ui.DBLogger, "[0] Resource read: %s", sql)

	rows, err := r.Database.Query(sql)
	if rows != nil {
		defer rows.Close()
	}

	if err == nil {
		for rows.Next() {
			rowData := make([]interface{}, len(r.Columns))
			rowDataPtrs := make([]interface{}, len(r.Columns))

			for i := range rowDataPtrs {
				rowDataPtrs[i] = &rowData[i]
			}

			err = rows.Scan(rowDataPtrs...)

			if err == nil {
				value := reflect.New(r.Type).Interface()
				count++

				for i := 0; i < len(rowData); i++ {
					switch r.Columns[i].SQLType {
					case "integer":
						reflect.ValueOf(value).Elem().Field(i).SetInt(data.Int64(rowData[i]))
					case "boolean":
						reflect.ValueOf(value).Elem().Field(i).SetBool(data.Bool(rowData[i]))
					case SQLStringType:
						if r.Columns[i].IsJSON {
							s := data.String(rowData[i])
							j := []string{}
							err = json.Unmarshal([]byte(s), &j)
							reflect.ValueOf(value).Elem().Field(i).Set(reflect.ValueOf(j))
						} else if r.Columns[i].IsUUID {
							s := data.String(rowData[i])
							u, _ := uuid.Parse(s)
							reflect.ValueOf(value).Elem().Field(i).Set(reflect.ValueOf(u))
						} else {
							reflect.ValueOf(value).Elem().Field(i).SetString(data.String(rowData[i]))
						}
					}
				}

				results = append(results, value)
			}
		}
	}

	if err == nil {
		ui.Log(ui.DBLogger, "[0] Resource list read %d rows", count)
	}

	return results, err
}
