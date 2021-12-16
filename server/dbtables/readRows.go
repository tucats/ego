package dbtables

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
)

// ReadTable reads the metadata for a given table, and returns it as an array
// of column names and types
func ReadRows(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	db, err := OpenDB(sessionID, user, "")

	if err == nil && db != nil {

		q := formQuery(r.URL, user)
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
