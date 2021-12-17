package dbtables

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
)

// ReadRows reads the data for a given table, and returns it as an array
// of structs for each row, with the struct tag being the column name. The
// query can also specify filter, sort, and column query parameters to refine
// the read operation.
func ReadRows(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	db, err := OpenDB(sessionID, user, "")

	if err == nil && db != nil {

		q := formQuery(r.URL, user, selectVerb)
		var rows *sql.Rows

		result := []map[string]interface{}{}
		rowCount := 0
		columnCount := 0
		ui.Debug(ui.ServerLogger, "[%d] Query: %s", sessionID, q)
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

				err := rows.Scan(rowptrs...)
				if err == nil {
					newRow := map[string]interface{}{}
					for i, v := range row {
						newRow[columnNames[i]] = v
					}
					result = append(result, newRow)
					rowCount++
				}
			}

			b, _ := json.MarshalIndent(result, "", "  ")
			w.Write(b)
			ui.Debug(ui.ServerLogger, "[%d] Read %d rows of %d columns", sessionID, rowCount, columnCount)

			return
		}
	}

	ui.Debug(ui.ServerLogger, "[%d] Error reading table, %v", sessionID, err)
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
}

// ReadRows reads the data for a given table, and returns it as an array
// of structs for each row, with the struct tag being the column name. The
// query can also specify filter, sort, and column query parameters to refine
// the read operation.
func DeleteRows(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	db, err := OpenDB(sessionID, user, "")

	if err == nil && db != nil {

		q := formQuery(r.URL, user, deleteVerb)

		ui.Debug(ui.ServerLogger, "[%d] Exec: %s", sessionID, q)
		rows, err := db.Exec(q)
		if err == nil {
			rowCount, _ := rows.RowsAffected()

			b, _ := json.MarshalIndent(rowCount, "", "  ")
			w.Write(b)
			ui.Debug(ui.ServerLogger, "[%d] Deleted %d rows ", sessionID, rowCount)

			return
		}
	}

	ui.Debug(ui.ServerLogger, "[%d] Error deleting from table, %v", sessionID, err)
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
}
