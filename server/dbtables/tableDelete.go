package dbtables

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
)

//DeleteTable will delete a database table from the user's schema
func DeleteTable(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	tableName, _ = fullName(user, tableName)
	db, err := OpenDB(sessionID, user, "")

	if err == nil && db != nil {

		if !isAdmin && Authorized(sessionID, nil, user, tableName, adminOperation) {
			ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)
			return
		}

		q := queryParameters(tableDeleteString, map[string]string{
			"table": tableName,
		})

		ui.Debug(ui.ServerLogger, "[%d] attempting to delete table %s", sessionID, tableName)
		ui.Debug(ui.ServerLogger, "[%d]    with query %s", sessionID, q)

		_, err = db.Exec(q)
		if err == nil {
			RemoveTablePermissions(sessionID, db, tableName)
			ErrorResponse(w, sessionID, "Table "+tableName+" successfully deleted", 200)

			return
		}

	}

	msg := fmt.Sprintf("Database table delete error, %v", err)
	if err == nil && db == nil {
		msg = "Unexpected nil database object pointer"
	}

	status := http.StatusBadRequest
	if strings.Contains(msg, "does not exist") {
		status = http.StatusNotFound
	}

	ErrorResponse(w, sessionID, msg, status)
}
