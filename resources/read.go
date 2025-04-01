package resources

import (
	"encoding/json"
	"reflect"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
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

	if r.Err != nil {
		return nil, r.Err
	}

	sql := generateReadSQL(r, filters)

	rows, err := r.Database.Query(sql)
	if rows != nil {
		defer rows.Close()
	}

	if err == nil {
		for rows.Next() {
			rowData := make([]interface{}, len(r.Columns))
			rowDataPointers := make([]interface{}, len(r.Columns))

			for i := range rowDataPointers {
				rowDataPointers[i] = &rowData[i]
			}

			err = rows.Scan(rowDataPointers...)

			if err == nil {
				value := reflect.New(r.Type).Interface()
				count++

				for i := 0; i < len(rowData); i++ {
					switch r.Columns[i].SQLType {
					case "integer":
						reflect.ValueOf(value).Elem().Field(i).SetInt(data.Int64OrZero(rowData[i]))
					case "float", "double":
						reflect.ValueOf(value).Elem().Field(i).SetFloat(data.Float64OrZero(rowData[i]))
					case "boolean":
						reflect.ValueOf(value).Elem().Field(i).SetBool(data.BoolOrFalse(rowData[i]))
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
		ui.Log(ui.ResourceLogger, "resource.read.rows", ui.A{
			"count": count})
	}

	return results, err
}

// generateReadSQL generates the SQL query for reading objects from the
// database, given the list of filters and any attached sorting specification
// associated with the resource handle.
func generateReadSQL(r *ResHandle, filters []*Filter) string {
	sql := r.readRowSQL()

	for index, filter := range filters {
		if index == 0 {
			sql = sql + whereClause
		} else {
			sql = sql + andClause
		}

		sql = sql + filter.Generate()
	}

	// Add any active order-by clause
	sql = sql + r.OrderBy()

	ui.Log(ui.ResourceLogger, "resource.read",ui.A{
		"sql": sql})

	return sql
}

// Read a single value from the resources using the default ID
// column as the key. This is a convenience function that reads
// the first row returned by the Read() method.
//
// The default key is the "id" column, but this can be overridden
// using the SetIDField() method.
func (r *ResHandle) ReadOne(key interface{}) (interface{}, error) {
	// Reset the deferred error state for a fresh start.
	r.Err = nil

	keyField := r.PrimaryKey()
	if keyField == "" {
		return nil, errors.ErrNotFound
	}

	rows, err := r.Read(r.Equals(keyField, key))
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, errors.ErrNotFound
	}

	return rows[0], nil
}

// PrimaryKey returns the name of the primary key field in the
// resource object. If there is no key, the empty string is returned.
func (r *ResHandle) PrimaryKey() string {
	for _, column := range r.Columns {
		if column.Primary {
			return column.Name
		}
	}

	return ""
}

// PrimaryKeyIndex returns the index of the primary key field in the
// resource object. If there is no key, -1 is returned.
func (r *ResHandle) PrimaryKeyIndex() int {
	for index, column := range r.Columns {
		if column.Primary {
			return index
		}
	}

	return -1
}
