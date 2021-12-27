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
func ReadTable(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	var rows *sql.Rows
	var q string

	db, err := OpenDB(sessionID, user, "")

	if err == nil && db != nil {

		// Special case; if the table name is @permissions then the payload is processed as request
		// to read all the permissions data
		if strings.EqualFold(tableName, permissionsPseudoTable) {
			if !isAdmin {
				ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)

				return
			}

			ReadAllPermissions(db, sessionID, w, r)

			return
		}

		tableName, _ = fullName(user, tableName)

		if !isAdmin && Authorized(sessionID, nil, user, tableName, readOperation) {
			ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)
			return
		}

		q = queryParameters(tableMetadataQuery, map[string]string{
			"table": tableName,
		})

		rows, err = db.Query(q)
		if err == nil {
			names, _ := rows.Columns()
			types, _ := rows.ColumnTypes()
			columns := make([]defs.DBColumn, 0)

			for i, name := range names {
				// Special case, we synthetically create a rowIDName column
				// and it is always of type "UUID". But we don't return it
				// as a user column name.
				if name == rowIDName {
					continue
				}

				typeInfo := types[i]

				// Start by seeing what Go type it will become. IF that isn't
				// known, then get the underlying database type name instead.
				typeName := typeInfo.ScanType().Name()
				if typeName == "" {
					typeName = typeInfo.DatabaseTypeName()
				}

				size, _ := typeInfo.Length()
				nullable, _ := typeInfo.Nullable()

				columns = append(columns, defs.DBColumn{
					Name:     name,
					Type:     typeName,
					Size:     int(size),
					Nullable: nullable},
				)
			}

			resp := defs.TableColumnsInfo{
				Columns: columns,
				Count:   len(columns),
				RestResponse: defs.RestResponse{
					Status: 200,
				},
			}

			b, _ := json.MarshalIndent(resp, "", "  ")
			_, _ = w.Write(b)

			return
		}
	}

	msg := fmt.Sprintf("Database table metadata error, %v", err)
	status := http.StatusBadRequest
	if strings.Contains(err.Error(), "does not exist") {
		status = http.StatusNotFound
	}

	if err == nil && db == nil {
		msg = "Unexpected nil database object pointer"
		status = http.StatusInternalServerError
	}

	ErrorResponse(w, sessionID, msg, status)
}
