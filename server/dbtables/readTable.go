package dbtables

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tucats/ego/defs"
)

// ReadTable reads the metadata for a given table, and returns it as an array
// of column names and types
func ReadTable(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {

	db, err := OpenDB(sessionID, user, "")

	if err == nil && db != nil {
		var rows *sql.Rows

		q := queryParameters(tableMetadataQuerySting, map[string]string{
			"table":  tableName,
			"schema": user,
		})

		rows, err = db.Query(q)
		if err == nil {
			names, _ := rows.Columns()
			types, _ := rows.ColumnTypes()
			columns := make([]defs.DBColumn, len(names))

			for i, name := range names {
				typeInfo := types[i]
				typeName := typeInfo.ScanType().Name()
				size, _ := typeInfo.Length()
				nullable, _ := typeInfo.Nullable()
				columns[i] = defs.DBColumn{Name: name, Type: typeName, Size: int(size), Nullable: nullable}
			}

			resp := defs.TableColumnsInfo{
				Columns:      columns,
				RestResponse: defs.RestResponse{Status: 200},
			}

			b, _ := json.MarshalIndent(resp, "", "  ")
			w.Write(b)

			return
		}
	}

	msg := fmt.Sprintf("Database table metadata error, %v", err)
	status := http.StatusBadRequest
	if strings.Contains(err.Error(), "not found") {
		status = http.StatusNotFound
	}

	if err == nil && db == nil {
		msg = "Unexpected nil database object pointer"
		status = http.StatusInternalServerError
	}

	errorResponse(w, sessionID, msg, status)
}
