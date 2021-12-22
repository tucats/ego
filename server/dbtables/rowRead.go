package dbtables

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

// ReadRows reads the data for a given table, and returns it as an array
// of structs for each row, with the struct tag being the column name. The
// query can also specify filter, sort, and column query parameters to refine
// the read operation.
func ReadRows(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	tableName, _ = fullName(user, tableName)

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {

		if !isAdmin && Authorized(sessionID, nil, user, tableName, readOperation) {
			ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)
			return
		}

		q := formSelectorDeleteQuery(r.URL, user, selectVerb)

		ui.Debug(ui.ServerLogger, "[%d] Query: %s", sessionID, q)

		err = readRowData(db, q, sessionID, w)
		if err == nil {
			return
		}

	}

	ui.Debug(ui.ServerLogger, "[%d] Error reading table, %v", sessionID, err)
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(err.Error()))
}

func readRowData(db *sql.DB, q string, sessionID int32, w http.ResponseWriter) error {
	var rows *sql.Rows
	var err error

	result := []map[string]interface{}{}
	rowCount := 0
	columnCount := 0

	rows, err = db.Query(q)
	if err == nil {
		columnNames, _ := rows.Columns()
		columnCount = len(columnNames)

		for rows.Next() {
			row := make([]interface{}, columnCount)
			rowptrs := make([]interface{}, columnCount)
			for i := range row {
				rowptrs[i] = &row[i]
			}

			err = rows.Scan(rowptrs...)
			if err == nil {
				newRow := map[string]interface{}{}
				for i, v := range row {
					newRow[columnNames[i]] = v
				}
				result = append(result, newRow)
				rowCount++
			}
		}

		resp := defs.DBRows{Rows: result,
			RestResponse: defs.RestResponse{
				Status: 200,
			},
		}

		b, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = w.Write(b)
		ui.Debug(ui.ServerLogger, "[%d] Read %d rows of %d columns", sessionID, rowCount, columnCount)
	}

	return err
}
