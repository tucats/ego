package dbtables

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

// UpdateRows updates the rows (specified by a filter clause as needed) with the data from the payload
func InsertRows(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	var err error
	tableName, _ = fullName(user, tableName)

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {

		if !isAdmin && Authorized(sessionID, nil, user, tableName, updateOperation) {
			ErrorResponse(w, sessionID, "User does not have update permission", http.StatusForbidden)
			return
		}

		var data map[string]interface{}

		err = json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			ErrorResponse(w, sessionID, "Invalid UPDATE payload: "+err.Error(), http.StatusBadRequest)

			return
		}

		if _, found := data[rowIDName]; !found {
			data[rowIDName] = uuid.New().String()
		}

		q, values := formInsertQuery(r.URL, user, data)
		ui.Debug(ui.ServerLogger, "[%d] Query: %s", sessionID, q)

		counts, err := db.Exec(q, values...)
		if err == nil {

			rows, _ := counts.RowsAffected()
			result := defs.DBRowCount{
				Count: int(rows),
				RestResponse: defs.RestResponse{
					Status: 200,
				},
			}

			b, _ := json.MarshalIndent(result, "", "  ")
			_, _ = w.Write(b)
			ui.Debug(ui.ServerLogger, "[%d] Updated %d rows", sessionID, rows)

			return
		}

		ui.Debug(ui.ServerLogger, "[%d] Error inserting into table, %v", sessionID, err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))

		return
	}

	ui.Debug(ui.ServerLogger, "[%d] Error inserting into table, %v", sessionID, err)
	w.WriteHeader(http.StatusBadRequest)
	if err == nil {
		err = fmt.Errorf("unknown error")
	}
	_, _ = w.Write([]byte(err.Error()))

}
