package tables

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/http/tables/database"
	"github.com/tucats/ego/http/tables/parsing"
	"github.com/tucats/ego/util"
)

const (
	readOperation   = "read"
	deleteOperation = "delete"
	adminOperation  = "admin"
	updateOperation = "update"
)

// Given a list of permission strings, indicate if they are all valid.
func validPermissions(perms []string) bool {
	for _, perm := range perms {
		// Strip off the grant/revoke flag if present
		if perm[:1] == "+" {
			perm = perm[1:]
		} else if perm[:1] == "-" {
			perm = perm[1:]
		}

		// The resulting permission name must match one of the permitted names.
		if !util.InList(strings.ToLower(perm),
			readOperation,
			deleteOperation,
			adminOperation,
			updateOperation,
		) {
			return false
		}
	}

	return true
}

// ReadPermissions reads the permissions data for a specific table. This operation requires either ownership
// of the table or admin privileges. The response is a Permission object for the given user and table.
func ReadPermissions(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	tableName := data.String(session.URLParts["table"])

	db, err := database.Open(&session.User, "", 0)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	_, _ = db.Exec(permissionsCreateTableQuery)

	table, fullyQualified := parsing.FullName(session.User, tableName)
	if !session.Admin && !fullyQualified {
		return util.ErrorResponse(w, session.ID, "Not authorized to read permissions", http.StatusForbidden)
	}

	reply := defs.PermissionObject{}
	parts := parsing.TableNameParts(session.User, table)
	reply.User = session.User
	reply.Schema = parts[0]
	reply.Table = parts[1]

	rows, err := db.Query(permissionsSelectQuery, parsing.StripQuotes(session.User), parsing.StripQuotes(table))
	if err != nil {
		defer rows.Close()
		ui.Log(ui.TableLogger, "[%d] Error reading permissions field: %v", session.ID, err)

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	permissionsMap := map[string]bool{}

	for rows.Next() {
		permissionString := ""
		_ = rows.Scan(&permissionString)
		ui.Log(ui.TableLogger, "[%d] Permissions list for user %s, table %s: %v", session.ID,
			parsing.StripQuotes(session.User), parsing.StripQuotes(table), permissionString)

		for _, perm := range strings.Split(strings.ToLower(permissionString), ",") {
			permissionsMap[strings.TrimSpace(perm)] = true
		}
	}

	if len(permissionsMap) == 0 {
		ui.Log(ui.TableLogger, "[%d] No matching permissions entries for user %s, tabale %s", session.ID,
			parsing.StripQuotes(session.User), parsing.StripQuotes(table))
	}

	reply.Permissions = make([]string, 0)
	for k := range permissionsMap {
		reply.Permissions = append(reply.Permissions, k)
	}

	sort.Strings(reply.Permissions)
	w.WriteHeader(http.StatusOK)

	b, _ := json.MarshalIndent(reply, "", "  ")
	_, _ = w.Write(b)

	return http.StatusOK
}

// ReadAllPermissions reads all permissions for all tables. By default it is for all users, though you can use the
// ?user= parameter to specify permissions for a given user for all tables. The result is an array of permissions
// objects for each permutation of owner and table name visible to the user.
func ReadAllPermissions(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	db, err := database.Open(&session.User, "", 0)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	_, _ = db.Exec(permissionsCreateTableQuery)

	reply := defs.AllPermissionResponse{
		Permissions: []defs.PermissionObject{},
	}

	filter := ""
	if f := parsing.RequestForUser("", r.URL); f != "" {
		filter = fmt.Sprintf("WHERE username = '%s'", parsing.SQLEscape(f))
	}

	q := fmt.Sprintf(`SELECT username, tablename, permissions FROM admin.privileges %s ORDER BY username,tablename`, filter)

	ui.Log(ui.TableLogger, "[%d] Query: %s", session.ID, q)

	rows, err := db.Query(q)
	if err != nil {
		defer rows.Close()
		ui.Log(ui.TableLogger, "[%d] Error reading permissions: %v", session.ID, err)

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	count := 0

	for rows.Next() {
		var user, table, permissionString string

		permObject := defs.PermissionObject{}
		permissionsMap := map[string]bool{}
		count = count + 1

		if err = rows.Scan(&user, &table, &permissionString); err != nil {
			ui.Log(ui.TableLogger, "[%d] Error scanning permissions: %v", session.ID, err)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
		}

		for _, perm := range strings.Split(strings.ToLower(permissionString), ",") {
			permissionsMap[strings.TrimSpace(perm)] = true
		}

		permObject.Permissions = make([]string, 0)
		for k := range permissionsMap {
			permObject.Permissions = append(permObject.Permissions, k)
		}

		sort.Strings(permObject.Permissions)

		parts := parsing.TableNameParts(user, table)
		permObject.User = user
		permObject.Schema = parts[0]
		permObject.Table = parts[1]

		reply.Permissions = append(reply.Permissions, permObject)
	}

	reply.Count = count

	w.WriteHeader(http.StatusOK)

	b, _ := json.MarshalIndent(reply, "", "  ")
	_, _ = w.Write(b)

	return http.StatusOK
}

// GrantPermissions is used to grant and revoke permissions. The Request must be a JSON array of strings, each of which is
// a permission to be granted or revoked. The permissions is revoked if it starts with a "-" character, else it is granted.
// You must be the owner of the table or an admin user to perform this operation.
func GrantPermissions(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	tableName := data.String(session.URLParts["table"])

	db, err := database.Open(&session.User, "", 0)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	_, _ = db.Exec(permissionsCreateTableQuery)
	user := parsing.RequestForUser(session.User, r.URL)
	table, fullyQualified := parsing.FullName(session.User, tableName)

	if !session.Admin && !fullyQualified {
		return util.ErrorResponse(w, session.ID, "Not authorized to update permissions", http.StatusForbidden)
	}

	permissionsList := []string{}

	if err = json.NewDecoder(r.Body).Decode(&permissionsList); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	sort.Strings(permissionsList)

	if !validPermissions(permissionsList) {
		return util.ErrorResponse(w, session.ID, fmt.Sprintf("invalid permissions list: %s", permissionsList), http.StatusBadRequest)
	}

	var buff strings.Builder

	for i, key := range permissionsList {
		if i > 0 {
			buff.WriteRune(',')
		}

		buff.WriteString(strings.TrimSpace(strings.ToLower(key)))
	}

	err = grantPermissions(session.ID, db.Handle, user, table, buff.String())

	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	return ReadPermissions(session, w, r)
}

// DeletePermissions deletes one or permissions records for a given username and table. The permissions data is deleted completely,
// which means this table will only be visible to admin users.
func DeletePermissions(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	db, err := database.Open(&session.User, "", 0)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	_, _ = db.Exec(permissionsCreateTableQuery)

	tableName := data.String(session.URLParts["table"])

	table, fullyQualified := parsing.FullName(session.User, tableName)
	if !session.Admin && !fullyQualified {
		return util.ErrorResponse(w, session.ID, "Not authorized to delete permissions", http.StatusForbidden)
	}

	if _, err = db.Exec(permissionsDeleteQuery, session.User, table); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)

	return http.StatusOK
}

// Authorized uses the database located in the the Ego tables database
// to determine if the proposed operation is permitted for the given table.
//
// The permissions string for the table and user is read, if it exists,
// must contain the given permission.
func Authorized(sessionID int, db *sql.DB, user string, table string, operations ...string) bool {
	_, err := db.Exec(permissionsCreateTableQuery)
	if err != nil {
		ui.Log(ui.TableLogger, "[%d] Error in permissions table create: %v", sessionID, err)
	}

	table, _ = parsing.FullName(user, table)

	rows, err := db.Query(permissionsSelectQuery, parsing.StripQuotes(user), parsing.StripQuotes(table))
	if err != nil {
		ui.Log(ui.TableLogger, "[%d] Error reading permissions: %v", sessionID, err)

		return false
	}

	if !rows.Next() {
		ui.Log(ui.TableLogger, "[%d] No permissions record for %s:%s", sessionID, user, table)

		return false
	}

	permissions := ""

	if err := rows.Scan(&permissions); err != nil {
		ui.Log(ui.TableLogger, "[%d] Error reading permissions: %v", sessionID, err)

		return false
	}

	defer rows.Close()

	permissions = strings.ToLower(permissions)
	auth := true

	for _, operation := range operations {
		if !strings.Contains(permissions, strings.ToLower(operation)) {
			auth = false
		}
	}

	if ui.IsActive(ui.TableLogger) {
		operationsList := ""

		for i, operation := range operations {
			if i > 0 {
				operationsList = operationsList + ","
			}

			operationsList = operationsList + strings.ToLower(operation)
		}

		if !auth {
			ui.WriteLog(ui.TableLogger, "[%d] User %s does not have %s permission for table %s", sessionID, user, operationsList, table)
		} else {
			ui.WriteLog(ui.TableLogger, "[%d] User %s has %s permission for table %s", sessionID, user, operationsList, table)
		}
	}

	return auth
}

// RemoveTablePermissions updates the permissions data to remove references to
// the named table.
func RemoveTablePermissions(sessionID int, db *sql.DB, table string) bool {
	_, _ = db.Exec(permissionsCreateTableQuery)

	result, err := db.Exec(permissionsDeleteAllQuery, parsing.StripQuotes(table))
	if err != nil {
		ui.Log(ui.TableLogger, "[%d] Error deleting permissions: %v", sessionID, err)

		return false
	}

	if count, err := result.RowsAffected(); err != nil || count == 0 {
		ui.Log(ui.TableLogger, "[%d] No permissions found for %s", sessionID, table)

		return false
	} else {
		ui.Log(ui.TableLogger, "[%d] %d permissions for %s deleted", sessionID, count, table)
	}

	return true
}

// CreateTablePermissions creates a row for the permissions data for a given user and named table, with
// the permissions enumerated as the last parameters.
func CreateTablePermissions(sessionID int, db *sql.DB, user, table string, permissions ...string) bool {
	_, _ = db.Exec(permissionsCreateTableQuery)

	// If this is a two-part name, we must create a permissions object for the owner/schema of the table
	if dot := strings.Index(table, "."); dot > 0 {
		schema := table[:dot]
		ok := doCreateTablePermissions(sessionID, db, schema, table, permissions...)

		//If this failed, or the two part name was already correct for this user, no more work.
		if !ok || schema == user {
			return ok
		}
	}

	// Also create an entry for the current user.
	return doCreateTablePermissions(sessionID, db, user, table, permissions...)
}

func doCreateTablePermissions(sessionID int, db *sql.DB, user, table string, permissions ...string) bool {
	var permissionList string

	_, _ = db.Exec(permissionsCreateTableQuery)

	if len(permissions) == 0 {
		permissionList = strings.Join([]string{readOperation, deleteOperation}, ",")
	} else {
		for i, permission := range permissions {
			if i > 0 {
				permissionList = permissionList + ","
			}
			permissionList = permissionList + permission
		}
	}

	// Upsert isn't always available, so delete any candidate row(s) before
	// adding in the new one.
	_, err := db.Exec(permissionsDeleteQuery, parsing.StripQuotes(user), parsing.StripQuotes(table))
	if err != nil {
		ui.Log(ui.TableLogger, "[%d] Error updating permissions: %v", sessionID, err)

		return false
	}

	_, err = db.Exec(permissionsInsertQuery, parsing.StripQuotes(user), parsing.StripQuotes(table), permissionList)
	if err != nil {
		ui.Log(ui.TableLogger, "[%d] Error updating permissions: %v", sessionID, err)

		return false
	}

	ui.Log(ui.TableLogger, "[%d] permissions for %s, table %s, set to %s", sessionID, user, table, permissionList)

	return true
}

func grantPermissions(sessionID int, db *sql.DB, user string, table string, permissions string) error {
	// Decompose the permissions list
	permissionNames := strings.Split(permissions, ",")
	tableName, _ := parsing.FullName(user, table)

	sort.Strings(permissionNames)

	ui.Log(ui.TableLogger, "[%d] Attempting to set %s permissions for %s to %s", sessionID, user, tableName, permissionNames)

	rows, err := db.Query(`select permissions from admin.privileges where username=$1 and tablename=$2`, parsing.StripQuotes(user), parsing.StripQuotes(tableName))
	if err != nil {
		return errors.NewError(err).Context(user + ":" + tableName)
	}

	defer rows.Close()

	permMap := map[string]bool{}
	permissionsString := ""

	for rows.Next() {
		_ = rows.Scan(&permissionsString)

		for _, perm := range strings.Split(permissionsString, ",") {
			normalizedPermName := strings.ToLower(strings.TrimSpace(perm))
			permMap[normalizedPermName] = true
		}
	}

	// Apply the permissions we were given
	for _, perm := range permissionNames {
		normalizedName := strings.ToLower(strings.TrimSpace(perm))
		if normalizedName[0:1] == "-" {
			delete(permMap, normalizedName[1:])
		} else {
			if normalizedName[0:1] == "+" {
				normalizedName = normalizedName[1:]
			}
			permMap[normalizedName] = true
		}
	}

	// Build the new permissions string
	permissions = ""

	for key := range permMap {
		if len(key) == 0 {
			continue
		}

		if len(permissions) > 0 {
			permissions = permissions + ","
		}

		permissions = permissions + key
	}

	// Attempt to update the permissions.
	var result sql.Result

	context := "updating permissions"

	result, err = db.Exec(permissionsUpdateQuery, parsing.StripQuotes(user), parsing.StripQuotes(tableName), permissions)
	if err == nil {
		if rowCount, _ := result.RowsAffected(); rowCount == 0 {
			context = "adding permissions"

			_, err = db.Exec(permissionsInsertQuery, parsing.StripQuotes(user), parsing.StripQuotes(tableName), permissions)
			if err == nil {
				ui.Log(ui.TableLogger, "[%d] created permissions for %s", sessionID, tableName)
			}
		} else {
			ui.Log(ui.TableLogger, "[%d] updated permissions for %s", sessionID, tableName)
		}
	}

	if err != nil {
		return errors.NewError(err).Context(context)
	}

	return nil
}
