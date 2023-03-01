package tables

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/http/auth"
	"github.com/tucats/ego/http/server"
	runtime_strings "github.com/tucats/ego/runtime/strings"
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

func TablesHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	server.CountRequest(server.TableRequestCounter)

	path := r.URL.Path

	w.Header().Add(defs.EgoServerInstanceHeader, defs.ServerInstanceID)

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
	authorization := r.Header.Get("Authorization")

	if authorization == "" {
		// No authentication credentials provided
		authenticatedCredentials = false

		ui.Log(ui.AuthLogger, "[%d] No authentication credentials given", session.ID)
	} else if strings.HasPrefix(strings.ToLower(authorization), defs.AuthScheme) {
		// Bearer token provided. Extract the token part of the header info, and
		// attempt to validate it.
		token := strings.TrimSpace(authorization[len(defs.AuthScheme):])
		authenticatedCredentials = auth.ValidateToken(token)
		user = auth.TokenUser(token)

		// If doing INFO logging, make a neutered version of the token showing
		// only the first few bytes of the token string.
		if ui.IsActive(ui.AuthLogger) {
			tokenstr := token
			if len(tokenstr) > 10 {
				tokenstr = tokenstr[:10] + "..."
			}

			valid := ", invalid credential"
			if authenticatedCredentials {
				if auth.GetPermission(user, rootPrivileges) {
					valid = ", root privilege user"
				} else {
					valid = ", normal user"
				}
			}

			ui.Log(ui.AuthLogger, "[%d] Auth using token %s, user %s%s", session.ID, tokenstr, user, valid)
		}
	} else {
		// Must have a valid username:password. This must be syntactically valid, and
		// if so, is also checked to see if the credentials are valid for our user
		// database.
		var ok bool

		user, pass, ok = r.BasicAuth()
		if !ok {
			ui.Log(ui.AuthLogger, "[%d] BasicAuth invalid", session.ID)
		} else {
			authenticatedCredentials = auth.ValidatePassword(user, pass)
		}

		valid := ", invalid credential"
		if authenticatedCredentials {
			if auth.GetPermission(user, rootPrivileges) {
				valid = ", root privilege user"
			} else {
				valid = ", normal user"
			}
		}

		ui.Log(ui.AuthLogger, "[%d] Auth using user \"%s\"%s", session.ID,
			user, valid)
	}

	// If we don't have a user after all this, fail. Table servers REQUIRE
	// authorization
	if user == "" || !authenticatedCredentials {
		msg := fmt.Sprintf("Operation %s from user %s requires valid user credentials",
			r.Method, user)

		util.ErrorResponse(w, session.ID, msg, http.StatusUnauthorized)

		return http.StatusUnauthorized
	}

	// Let's check in on permissions.
	//
	// 1. A user with table_read permission can read rows
	// 2. A user with table_modify permission can read and update rows
	// 3. A user with table_admin permission can list and modify tables
	// 4. A user with tables permission can do any table operation
	// 5. A user with root permission can do any table operation

	hasReadPermission := auth.GetPermission(user, tableAccessPrivilege)
	hasAdminPermission := auth.GetPermission(user, tableAdminPrivilege)
	hasUpdatePermission := auth.GetPermission(user, tableUpdatePrivilege)

	if !hasReadPermission {
		hasReadPermission = auth.GetPermission(user, rootPrivileges)
		if hasReadPermission {
			hasAdminPermission = true
			hasUpdatePermission = true
		} else {
			hasReadPermission = auth.GetPermission(user, tablesPrivileges)
			if hasReadPermission {
				hasAdminPermission = true
				hasUpdatePermission = true
			}
		}
	}

	if !hasReadPermission {
		msg := fmt.Sprintf("User %s does not have tables permissions", user)

		util.ErrorResponse(w, session.ID, msg, http.StatusForbidden)

		return http.StatusForbidden
	}

	ui.Log(ui.ServerLogger, "[%d] Table request for user %s (admin=%v); %s %s",
		session.ID, user, hasAdminPermission, r.Method, path)
	ui.Log(ui.RestLogger, "[%d] User agent: %s", session.ID, r.Header.Get("User-Agent"))

	urlParts, valid := runtime_strings.ParseURLPattern(path, "/tables/{{table}}/rows")
	if !valid {
		urlParts, valid = runtime_strings.ParseURLPattern(path, "/tables/{{table}}/permissions")
	}

	if !valid {
		urlParts, valid = runtime_strings.ParseURLPattern(path, "/tables/{{table}}/transaction")
	}

	if !valid || !data.Bool(urlParts["tables"]) {
		msg := "Invalid tables path specified, " + path
		util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)

		return http.StatusBadRequest
	}

	tableName := data.String(urlParts["table"])
	rows := data.Bool(urlParts["rows"])
	perms := data.Bool(urlParts["permissions"])
	transaction := strings.EqualFold(tableName, "@transaction")

	if transaction {
		if r.Method != http.MethodPost {
			msg := fmt.Sprintf("Unsupported transaction method %s", r.Method)

			return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
		}

		if !hasUpdatePermission {
			msg := fmt.Sprintf("User %s does not have permission to modify tables", user)

			return util.ErrorResponse(w, session.ID, msg, http.StatusForbidden)
		}

		return Transaction(user, hasAdminPermission || hasUpdatePermission, session.ID, w, r)
	}

	if tableName == "" && r.Method != http.MethodGet {
		msg := unsupportedMethodMessage

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	if tableName == "" {
		return ListTables(user, hasAdminPermission, session.ID, w, r)
	}

	if rows {
		if r.Method != http.MethodGet && !hasUpdatePermission {
			msg := fmt.Sprintf("User %s does not have permission to modify tables", user)

			return util.ErrorResponse(w, session.ID, msg, http.StatusForbidden)
		}

		switch r.Method {
		case http.MethodGet:
			return ReadRows(user, hasAdminPermission, tableName, session.ID, w, r)

		case http.MethodPut:
			return InsertRows(user, hasAdminPermission, tableName, session.ID, w, r)

		case http.MethodDelete:
			return DeleteRows(user, hasAdminPermission, tableName, session.ID, w, r)

		case http.MethodPatch:
			return UpdateRows(user, hasAdminPermission, tableName, session.ID, w, r)

		default:
			return util.ErrorResponse(w, session.ID, unsupportedMethodMessage, http.StatusMethodNotAllowed)
		}
	}

	if perms {
		if r.Method != http.MethodGet && !hasUpdatePermission {
			msg := "User does not have permission to modify tables"

			return util.ErrorResponse(w, session.ID, msg, http.StatusForbidden)
		}

		switch r.Method {
		case http.MethodGet:
			return ReadPermissions(user, hasAdminPermission, tableName, session.ID, w, r)

		case http.MethodPut:
			return GrantPermissions(user, hasAdminPermission, tableName, session.ID, w, r)

		case http.MethodDelete:
			return DeletePermissions(user, hasAdminPermission, tableName, session.ID, w, r)

		default:
			return util.ErrorResponse(w, session.ID, unsupportedMethodMessage, http.StatusMethodNotAllowed)
		}
	}

	// Since it's not a row operation, it must be a table-level operation, which is
	// only permitted for root users or those with "table_admin" privilege
	if !hasAdminPermission && !hasUpdatePermission && !hasReadPermission {
		msg := "User does not have permission to access tables"

		return util.ErrorResponse(w, session.ID, msg, http.StatusForbidden)
	}

	switch r.Method {
	case http.MethodGet:
		if !hasAdminPermission && !hasReadPermission {
			msg := "User does not have permission to read tables"

			return util.ErrorResponse(w, session.ID, msg, http.StatusForbidden)
		}

		return ReadTable(user, hasAdminPermission, tableName, session.ID, w, r)

	case http.MethodPut, http.MethodPost:
		if !hasAdminPermission && !hasUpdatePermission {
			msg := "User does not have permission to create tables"

			return util.ErrorResponse(w, session.ID, msg, http.StatusForbidden)
		}

		// If the table is the SQL pseudo-table name, then dispatch to the
		// SQL statement handler. Otherwise, it's a table create operation.
		if strings.EqualFold(tableName, sqlPseudoTable) {
			return SQLTransaction(r, w, session.ID, user)
		} else {
			return TableCreate(user, hasAdminPermission, tableName, session.ID, w, r)
		}

	case http.MethodDelete:
		if !hasAdminPermission && !hasUpdatePermission {
			msg := "User does not have permission to delete tables"

			return util.ErrorResponse(w, session.ID, msg, http.StatusForbidden)
		}

		DeleteTable(user, hasAdminPermission, tableName, session.ID, w, r)

	case http.MethodPatch:
		if !hasAdminPermission && !hasUpdatePermission {
			msg := "User does not have permission to alter tables"

			return util.ErrorResponse(w, session.ID, msg, http.StatusForbidden)
		}

		alterTable(user, tableName, session.ID, w, r)

	default:
		return util.ErrorResponse(w, session.ID, unsupportedMethodMessage, http.StatusMethodNotAllowed)
	}

	return http.StatusOK
}

// Status 418 (Teapot); to be implemented.
func alterTable(user string, tableName string, sessionID int, w http.ResponseWriter, r *http.Request) int {
	msg := fmt.Sprintf("Unsupported request to alter metadata from table %s for user %s", tableName, user)

	return util.ErrorResponse(w, sessionID, msg, http.StatusTeapot)
}
