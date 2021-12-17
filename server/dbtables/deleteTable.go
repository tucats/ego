package dbtables

import (
	"fmt"
	"net/http"
	"strings"
)

//DeleteTable will delete a database table from the user's schema
func DeleteTable(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {

	db, err := OpenDB(sessionID, user, "")

	if err == nil && db != nil {
		q := queryParameters(tableDeleteString, map[string]string{
			"table":  tableName,
			"schema": user,
		})

		_, err = db.Exec(q)
		if err == nil {
			errorResponse(w, sessionID, "Table "+tableName+"successfully deleted", 200)
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

	errorResponse(w, sessionID, msg, status)
}
