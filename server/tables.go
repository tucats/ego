package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/server/dbtables"
)

func TablesHandler(w http.ResponseWriter, r *http.Request) {
	CountRequest(AssetRequestCounter)
	w.Header().Add("Content-type", "application/json")

	sessionID := atomic.AddInt32(&nextSessionID, 1)
	path := r.URL.Path

	//	Tables API
	//
	//	/tables/				- list of tables available to this user
	//	/tables/<name>/	        - list of columns for this table
	//	/tables/<name>/rows	    - GET reads rows, PUT writes rows, PATCH updates rows
	//							  optional filters:
	//								+ filter=
	//								+ limit=
	//								+ start=
	//								+ columns=
	//

	// Get the query parameters and store as a local variable
	queryParameters := r.URL.Query()
	parameterStruct := map[string]interface{}{}

	for k, v := range queryParameters {
		values := make([]interface{}, 0)
		for _, vs := range v {
			values = append(values, vs)
		}

		parameterStruct[k] = values
	}

	// Handle authentication. This can be either Basic authentiation or using a Bearer
	// token in the header. If found, validate the username:password or the token string,
	// and set up state variables accordingly.
	var authenticatedCredentials bool

	user := ""
	pass := ""
	auth := r.Header.Get("Authorization")

	if auth == "" {
		// No authentication credentials provided
		authenticatedCredentials = false

		ui.Debug(ui.InfoLogger, "[%d] No authentication credentials given", sessionID)
	} else if strings.HasPrefix(strings.ToLower(auth), defs.AuthScheme) {
		// Bearer token provided. Extract the token part of the header info, and
		// attempt to validate it.
		token := strings.TrimSpace(auth[len(defs.AuthScheme):])
		authenticatedCredentials = validateToken(token)
		user = tokenUser(token)

		// If doing INFO logging, make a neutered version of the token showing
		// only the first few bytes of the token string.
		if ui.LoggerIsActive(ui.InfoLogger) {
			tokenstr := token
			if len(tokenstr) > 10 {
				tokenstr = tokenstr[:10] + "..."
			}

			valid := ", invalid credential"
			if authenticatedCredentials {
				if getPermission(user, "root") {
					valid = ", root privilege user"
				} else {
					valid = ", normal user"
				}
			}

			ui.Debug(ui.InfoLogger, "[%d] Auth using token %s, user %s%s", sessionID, tokenstr, user, valid)
		}
	} else {
		// Must have a valid username:password. This must be syntactically valid, and
		// if so, is also checked to see if the credentials are valid for our user
		// database.
		var ok bool

		user, pass, ok = r.BasicAuth()
		if !ok {
			ui.Debug(ui.InfoLogger, "[%d] BasicAuth invalid", sessionID)
		} else {
			authenticatedCredentials = validatePassword(user, pass)
		}

		valid := ", invalid credential"
		if authenticatedCredentials {
			if getPermission(user, "root") {
				valid = ", root privilege user"
			} else {
				valid = ", normal user"
			}
		}

		ui.Debug(ui.InfoLogger, "[%d] Auth using user \"%s\"%s", sessionID,
			user, valid)
	}

	// If we don't have a user after all this, fail. Table servers REQUIRE
	// authorization
	if user == "" || !authenticatedCredentials {
		msg := "Operation requires valid user credentials"

		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
			sessionID, r.Method, path, r.RemoteAddr, http.StatusUnauthorized)
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(msg))
		return
	}

	hasPermission := getPermission(user, "tables")
	if !hasPermission {
		hasPermission = getPermission(user, "root")
	}

	if !hasPermission {
		msg := "User does not have tables permissions"

		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
			sessionID, r.Method, path, r.RemoteAddr, http.StatusForbidden)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(msg))
		return
	}

	ui.Debug(ui.ServerLogger, "[%d] Table request; user %s; %s %s", sessionID, user, r.Method, path)

	urlParts, valid := functions.ParseURLPattern(path, "/tables/{{table}}/rows")
	ui.Debug(ui.ServerLogger, "[%d] urlParts (valid=%v) %v", sessionID, valid, urlParts)

	if !valid || !datatypes.GetBool(urlParts["tables"]) {
		msg := "Invalid tables path specified, " + path

		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
			sessionID, r.Method, path, r.RemoteAddr, http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(msg))
		return
	}

	tableName := datatypes.GetString(urlParts["table"])
	rows := datatypes.GetBool(urlParts["rows"])

	if tableName == "" && r.Method != "GET" {
		msg := "Unsupported method"
		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
			sessionID, r.Method, path, r.RemoteAddr, http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(msg))
		return
	}

	if tableName == "" {
		dbtables.ListTables(user, sessionID, w, r)

		return
	}

	if rows {
		switch r.Method {
		case http.MethodGet:
			readRows(user, tableName, sessionID, w, r)

		case http.MethodPut:
			appendRows(user, tableName, sessionID, w, r)

		case http.MethodDelete:
			deleteRows(user, tableName, sessionID, w, r)

		case http.MethodPatch:
			updateRows(user, tableName, sessionID, w, r)

		default:
			msg := "Unsupported method"
			ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
				sessionID, r.Method, r.URL.Path, r.RemoteAddr, http.StatusBadRequest)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(msg))
		}

		return
	}

	switch r.Method {
	case http.MethodGet:
		dbtables.ReadTable(user, tableName, sessionID, w, r)

	case http.MethodPut:
		createTable(user, tableName, sessionID, w, r)

	case http.MethodDelete:
		deleteTable(user, tableName, sessionID, w, r)

	case http.MethodPatch:
		alterTable(user, tableName, sessionID, w, r)

	default:
		msg := "Unsupported method"
		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
			sessionID, r.Method, r.URL.Path, r.RemoteAddr, http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(msg))
	}

}

// @tomcole to be implemented
func readRows(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {

	msg := fmt.Sprintf("Reading rows from table %s for user %s", tableName, user)
	b, _ := json.MarshalIndent(msg, "", "  ")

	w.WriteHeader(http.StatusTeapot)
	w.Write(b)
}

// @tomcole to be implemented
func appendRows(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {

	msg := fmt.Sprintf("Append rows to table %s for user %s", tableName, user)
	b, _ := json.MarshalIndent(msg, "", "  ")

	w.WriteHeader(http.StatusTeapot)
	w.Write(b)
}

// @tomcole to be implemented
func deleteRows(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {

	msg := fmt.Sprintf("Delete rows from table %s for user %s", tableName, user)
	b, _ := json.MarshalIndent(msg, "", "  ")

	w.WriteHeader(http.StatusTeapot)
	w.Write(b)
}

// @tomcole to be implemented
func updateRows(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {

	msg := fmt.Sprintf("Updating rows in table %s for user %s", tableName, user)
	b, _ := json.MarshalIndent(msg, "", "  ")

	w.WriteHeader(http.StatusTeapot)
	w.Write(b)
}

// @tomcole to be implemented
func createTable(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {

	msg := fmt.Sprintf("Creating new table %s for user %s", tableName, user)
	b, _ := json.MarshalIndent(msg, "", "  ")

	w.WriteHeader(http.StatusTeapot)
	w.Write(b)
}

// @tomcole to be implemented
func deleteTable(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {

	msg := fmt.Sprintf("Deleting table %s for user %s", tableName, user)
	b, _ := json.MarshalIndent(msg, "", "  ")

	w.WriteHeader(http.StatusTeapot)
	w.Write(b)
}

// @tomcole to be implemented
func alterTable(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {

	msg := fmt.Sprintf("Altering metadata from table %s for user %s", tableName, user)
	b, _ := json.MarshalIndent(msg, "", "  ")

	w.WriteHeader(http.StatusTeapot)
	w.Write(b)
}
