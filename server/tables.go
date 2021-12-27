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

const (
	rootPrivileges       = "root"
	tablesPrivileges     = "tables"
	tableAccessPrivilege = "table_read"
	tableAdminPrivilege  = "table_admin"
	tableUpdatePrivilege = "table_modify"
)

func TablesHandler(w http.ResponseWriter, r *http.Request) {
	CountRequest(AssetRequestCounter)
	w.Header().Add("Content-type", "application/json")

	sessionID := atomic.AddInt32(&nextSessionID, 1)
	path := r.URL.Path

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
				if getPermission(user, rootPrivileges) {
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
			if getPermission(user, rootPrivileges) {
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
		_, _ = w.Write([]byte(msg))
		return
	}

	// Let's check in on permissions.
	//
	// 1. A user with table_read permission can read rows
	// 2. A user with table_modify permission can read and update rows
	// 3. A user with table_admin permission can list and modify tables
	// 4. A user with tables permission can do any table operation
	// 5. A user with root permission can do any table operation

	hasReadPermission := getPermission(user, tableAccessPrivilege)
	hasAdminPermission := getPermission(user, tableAdminPrivilege)
	hasUpdatePermission := getPermission(user, tableUpdatePrivilege)

	if !hasReadPermission {
		hasReadPermission = getPermission(user, rootPrivileges)
		if hasReadPermission {
			hasAdminPermission = true
			hasUpdatePermission = true
		} else {
			hasReadPermission = getPermission(user, tablesPrivileges)
			if hasReadPermission {
				hasAdminPermission = true
				hasUpdatePermission = true
			}
		}
	}

	if !hasReadPermission {
		msg := "User does not have tables permissions"

		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
			sessionID, r.Method, path, r.RemoteAddr, http.StatusForbidden)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(msg))
		return
	}

	ui.Debug(ui.ServerLogger, "[%d] Table request for user %s (admin=%v); %s %s",
		sessionID, user, hasAdminPermission, r.Method, path)

	urlParts, valid := functions.ParseURLPattern(path, "/tables/{{table}}/rows")
	if !valid {
		urlParts, valid = functions.ParseURLPattern(path, "/tables/{{table}}/permissions")
	}

	if !valid || !datatypes.GetBool(urlParts["tables"]) {
		msg := "Invalid tables path specified, " + path

		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
			sessionID, r.Method, path, r.RemoteAddr, http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(msg))
		return
	}

	tableName := datatypes.GetString(urlParts["table"])
	rows := datatypes.GetBool(urlParts["rows"])
	perms := datatypes.GetBool(urlParts["permissions"])

	if tableName == "" && r.Method != "GET" {
		msg := "Unsupported method"
		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
			sessionID, r.Method, path, r.RemoteAddr, http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(msg))
		return
	}

	if tableName == "" {
		dbtables.ListTables(user, hasAdminPermission, sessionID, w, r)

		return
	}

	if rows {

		if r.Method != http.MethodGet && !hasUpdatePermission {
			msg := "User does not have permission to modify tables"

			ui.Debug(ui.ServerLogger, "[%d] %s; %d", sessionID, msg, http.StatusForbidden)
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(msg))

			return
		}

		switch r.Method {
		case http.MethodGet:
			dbtables.ReadRows(user, hasAdminPermission, tableName, sessionID, w, r)

		case http.MethodPut:
			dbtables.InsertRows(user, hasAdminPermission, tableName, sessionID, w, r)

		case http.MethodDelete:
			dbtables.DeleteRows(user, hasAdminPermission, tableName, sessionID, w, r)

		case http.MethodPatch:
			dbtables.UpdateRows(user, hasAdminPermission, tableName, sessionID, w, r)

		default:
			msg := "Unsupported method"
			ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
				sessionID, r.Method, r.URL.Path, r.RemoteAddr, http.StatusBadRequest)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(msg))
		}

		return
	}

	if perms {

		if r.Method != http.MethodGet && !hasUpdatePermission {
			msg := "User does not have permission to modify tables"

			ui.Debug(ui.ServerLogger, "[%d] %s; %d", sessionID, msg, http.StatusForbidden)
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(msg))

			return
		}

		switch r.Method {
		case http.MethodGet:
			dbtables.ReadPermissions(user, hasAdminPermission, tableName, sessionID, w, r)

		case http.MethodPut:
			dbtables.GrantPermissions(user, hasAdminPermission, tableName, sessionID, w, r)

		case http.MethodDelete:
			dbtables.DeletePermissions(user, hasAdminPermission, tableName, sessionID, w, r)

		default:
			msg := "Unsupported method"
			ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
				sessionID, r.Method, r.URL.Path, r.RemoteAddr, http.StatusBadRequest)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(msg))
		}

		return
	}

	// Since it's not a row operation, it must be a table-level operation, which is
	// only permitted for root users or those with "table_admin" privilege
	if !hasAdminPermission {
		msg := "User does not have permission to admin tables"
		dbtables.ErrorResponse(w, sessionID, msg, http.StatusForbidden)

		return
	}

	switch r.Method {
	case http.MethodGet:
		dbtables.ReadTable(user, hasAdminPermission, tableName, sessionID, w, r)

	case http.MethodPut:
		dbtables.TableCreate(user, hasAdminPermission, tableName, sessionID, w, r)

	case http.MethodDelete:
		dbtables.DeleteTable(user, hasAdminPermission, tableName, sessionID, w, r)

	case http.MethodPatch:
		alterTable(user, tableName, sessionID, w, r)

	default:
		msg := "Unsupported method"
		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; %d",
			sessionID, r.Method, r.URL.Path, r.RemoteAddr, http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(msg))
	}

}

// @tomcole to be implemented
func alterTable(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {

	msg := fmt.Sprintf("Altering metadata from table %s for user %s", tableName, user)
	b, _ := json.MarshalIndent(msg, "", "  ")

	w.WriteHeader(http.StatusTeapot)
	_, _ = w.Write(b)
}
