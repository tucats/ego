package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/server/dbtables"
	"github.com/tucats/ego/util"
)

const (
	rootPrivileges       = "root"
	tablesPrivileges     = "tables"
	tableAccessPrivilege = "table_read"
	tableAdminPrivilege  = "table_admin"
	tableUpdatePrivilege = "table_modify"

	unsupportedMethodMessage = "Unsupported method"
)

func TablesHandler(w http.ResponseWriter, r *http.Request) {
	CountRequest(TableRequestCounter)

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

		if ui.LoggerIsActive(ui.AuthLogger) {
			ui.Debug(ui.AuthLogger, "[%d] No authentication credentials given", sessionID)

			// Because this is may be an unexpected result, let's dump the headers now.
			headerMsg := strings.Builder{}
			for k, v := range r.Header {
				for _, i := range v {
					headerMsg.WriteString("   ")
					headerMsg.WriteString(k)
					headerMsg.WriteString(": ")
					headerMsg.WriteString(i)
					headerMsg.WriteString("\n")
				}
			}

			ui.Debug(ui.AuthLogger, "[%d] Received headers:\n%s",
				sessionID,
				util.SessionLog(sessionID,
					strings.TrimSuffix(headerMsg.String(), "\n"),
				))
		}
	} else if strings.HasPrefix(strings.ToLower(auth), defs.AuthScheme) {
		// Bearer token provided. Extract the token part of the header info, and
		// attempt to validate it.
		token := strings.TrimSpace(auth[len(defs.AuthScheme):])
		authenticatedCredentials = validateToken(token)
		user = tokenUser(token)

		// If doing INFO logging, make a neutered version of the token showing
		// only the first few bytes of the token string.
		if ui.LoggerIsActive(ui.AuthLogger) {
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

			ui.Debug(ui.AuthLogger, "[%d] Auth using token %s, user %s%s", sessionID, tokenstr, user, valid)
		}
	} else {
		// Must have a valid username:password. This must be syntactically valid, and
		// if so, is also checked to see if the credentials are valid for our user
		// database.
		var ok bool

		user, pass, ok = r.BasicAuth()
		if !ok {
			ui.Debug(ui.AuthLogger, "[%d] BasicAuth invalid", sessionID)
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

		ui.Debug(ui.AuthLogger, "[%d] Auth using user \"%s\"%s", sessionID,
			user, valid)
	}

	// If we don't have a user after all this, fail. Table servers REQUIRE
	// authorization
	if user == "" || !authenticatedCredentials {
		msg := fmt.Sprintf("Operation %s from user %s requires valid user credentials",
			r.Method, user)

		util.ErrorResponse(w, sessionID, msg, http.StatusUnauthorized)

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
		msg := fmt.Sprintf("User %s does not have tables permissions", user)

		util.ErrorResponse(w, sessionID, msg, http.StatusForbidden)

		return
	}

	ui.Debug(ui.ServerLogger, "[%d] Table request for user %s (admin=%v); %s %s",
		sessionID, user, hasAdminPermission, r.Method, path)
	ui.Debug(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	urlParts, valid := functions.ParseURLPattern(path, "/tables/{{table}}/rows")
	if !valid {
		urlParts, valid = functions.ParseURLPattern(path, "/tables/{{table}}/permissions")
	}
	if !valid {
		urlParts, valid = functions.ParseURLPattern(path, "/tables/{{table}}/transaction")
	}

	if !valid || !datatypes.GetBool(urlParts["tables"]) {
		msg := "Invalid tables path specified, " + path
		util.ErrorResponse(w, sessionID, msg, http.StatusBadRequest)

		return
	}

	tableName := datatypes.GetString(urlParts["table"])
	rows := datatypes.GetBool(urlParts["rows"])
	perms := datatypes.GetBool(urlParts["permissions"])
	transaction := strings.EqualFold(tableName, "@transaction")

	if transaction {
		if r.Method != http.MethodPost {
			msg := fmt.Sprintf("Unsupported transaction method %s", r.Method)

			util.ErrorResponse(w, sessionID, msg, http.StatusBadRequest)

			return
		}

		if !hasUpdatePermission {
			msg := fmt.Sprintf("User %s does not have permission to modify tables", user)

			util.ErrorResponse(w, sessionID, msg, http.StatusForbidden)

			return
		}

		dbtables.Transaction(user, hasAdminPermission || hasUpdatePermission, sessionID, w, r)

		return
	}

	if tableName == "" && r.Method != http.MethodGet {
		msg := unsupportedMethodMessage
		util.ErrorResponse(w, sessionID, msg, http.StatusBadRequest)

		return
	}

	if tableName == "" {
		dbtables.ListTables(user, hasAdminPermission, sessionID, w, r)

		return
	}

	if rows {
		if r.Method != http.MethodGet && !hasUpdatePermission {
			msg := fmt.Sprintf("User %s does not have permission to modify tables", user)

			util.ErrorResponse(w, sessionID, msg, http.StatusForbidden)

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
			util.ErrorResponse(w, sessionID, unsupportedMethodMessage, http.StatusMethodNotAllowed)
		}

		return
	}

	if perms {
		if r.Method != http.MethodGet && !hasUpdatePermission {
			msg := "User does not have permission to modify tables"

			util.ErrorResponse(w, sessionID, msg, http.StatusForbidden)

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
			util.ErrorResponse(w, sessionID, unsupportedMethodMessage, http.StatusMethodNotAllowed)
		}

		return
	}

	// Since it's not a row operation, it must be a table-level operation, which is
	// only permitted for root users or those with "table_admin" privilege
	if !hasAdminPermission && !hasUpdatePermission && !hasReadPermission {
		msg := "User does not have permission to access tables"
		util.ErrorResponse(w, sessionID, msg, http.StatusForbidden)

		return
	}

	switch r.Method {
	case http.MethodGet:
		if !hasAdminPermission && !hasReadPermission {
			msg := "User does not have permission to read tables"
			util.ErrorResponse(w, sessionID, msg, http.StatusForbidden)

			return
		}

		dbtables.ReadTable(user, hasAdminPermission, tableName, sessionID, w, r)

	case http.MethodPut:
		if !hasAdminPermission && !hasUpdatePermission {
			msg := "User does not have permission to create tables"
			util.ErrorResponse(w, sessionID, msg, http.StatusForbidden)

			return
		}

		dbtables.TableCreate(user, hasAdminPermission, tableName, sessionID, w, r)

	case http.MethodDelete:
		if !hasAdminPermission && !hasUpdatePermission {
			msg := "User does not have permission to delete tables"
			util.ErrorResponse(w, sessionID, msg, http.StatusForbidden)

			return
		}

		dbtables.DeleteTable(user, hasAdminPermission, tableName, sessionID, w, r)

	case http.MethodPatch:
		if !hasAdminPermission && !hasUpdatePermission {
			msg := "User does not have permission to alter tables"
			util.ErrorResponse(w, sessionID, msg, http.StatusForbidden)

			return
		}

		alterTable(user, tableName, sessionID, w, r)

	default:
		util.ErrorResponse(w, sessionID, unsupportedMethodMessage, http.StatusMethodNotAllowed)
	}
}

// @tomcole to be implemented.
func alterTable(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("Unsupported request to alter metadata from table %s for user %s", tableName, user)
	util.ErrorResponse(w, sessionID, msg, http.StatusTeapot)
}
