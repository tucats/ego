package dbtables

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

const (
	readOperation   = "read"
	deleteOperation = "delete"
	adminOperation  = "admin"
	updateOperation = "update"
)

// authorized uses the database located in the the Ego tables database
// to determine if the proposed operation is permitted for the given table.
//
// The permissions string for the table and user is read, if it exists,
// must contain the given permission.
func Authorized(sessionID int32, db *sql.DB, user string, table string, operations ...string) bool {
	_, _ = db.Exec(createPermissionString)

	rows, err := db.Query(permissionsSelectString, user, table)
	if err != nil {
		ui.Debug(ui.ServerLogger, "[%d] Error reading permissions: %v", sessionID, err)

		return false
	}

	if !rows.Next() {
		ui.Debug(ui.ServerLogger, "[%d] No permissions record for %s:%s", sessionID, user, table)

		return false
	}

	permissions := ""
	err = rows.Scan(&permissions)
	if err != nil {
		ui.Debug(ui.ServerLogger, "[%d] Error reading permissions: %v", sessionID, err)

		return false
	}

	permissions = strings.ToLower(permissions)
	auth := true
	for _, operation := range operations {
		if !strings.Contains(permissions, strings.ToLower(operation)) {
			auth = false
		}
	}

	_ = rows.Close()

	if !auth && ui.LoggerIsActive(ui.ServerLogger) {
		operationsList := ""
		for i, operation := range operations {
			if i > 0 {
				operationsList = operationsList + ","
			}

			operationsList = operationsList + strings.ToLower(operation)
		}

		ui.Debug(ui.ServerLogger, "[%d] %s:%s does not have %s permission", sessionID, user, table, operationsList)
	}

	return auth
}

// RemoveTablePermissions updates the permissions data to remove references to
// the named table.
func RemoveTablePermissions(sessionID int32, db *sql.DB, table string) bool {
	_, _ = db.Exec(createPermissionString)
	result, err := db.Exec(permissionsDeleteString, table)
	if err != nil {
		ui.Debug(ui.ServerLogger, "[%d] Error deleting permissions: %v", sessionID, err)

		return false
	}

	if count, err := result.RowsAffected(); err != nil || count == 0 {
		ui.Debug(ui.ServerLogger, "[%d] No permissions found for %s", sessionID, table)

		return false
	} else {
		ui.Debug(ui.ServerLogger, "[%d] %d permissions for %s deleted", sessionID, count, table)
	}

	return true
}

// RemoveTablePermissions updates the permissions data to remove references to
// the named table.
func CreateTablePermissions(sessionID int32, db *sql.DB, user, table string, permissions ...string) bool {
	_, _ = db.Exec(createPermissionString)

	// If this is a two-part name, we must create a permissions object for the owner/schema of the table
	if dot := strings.Index(table, "."); dot > 0 {
		schema := table[:dot]
		//table = table[dot+1:]
		ok := doCreateTablePermissions(sessionID, db, schema, table, permissions...)

		//If this failed, or the two part name was already correct for this user, no more work.
		if !ok || schema == user {
			return ok
		}
	}

	// Also create an entry for the current user.
	return doCreateTablePermissions(sessionID, db, user, table, permissions...)

}

func doCreateTablePermissions(sessionID int32, db *sql.DB, user, table string, permissions ...string) bool {
	var permissionList string

	_, _ = db.Exec(createPermissionString)

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
	q := "DELETE FROM admin.privileges WHERE username = $1 AND tablename = $2"
	_, err := db.Exec(q, user, table)
	if err != nil {
		ui.Debug(ui.ServerLogger, "[%d] Error updating permissions: %v", sessionID, err)

		return false
	}

	_, err = db.Exec(permissionsInsertString, user, table, permissionList)
	if err != nil {
		ui.Debug(ui.ServerLogger, "[%d] Error updating permissions: %v", sessionID, err)

		return false
	}

	ui.Debug(ui.ServerLogger, "[%d] permissions for %s, table %s, set to %s", sessionID, user, table, permissionList)

	return true
}

func grantPermissions(sessionID int32, db *sql.DB, user string, table string, permissions string) *errors.EgoError {
	// Decompose the permissions list
	permissionNames := strings.Split(permissions, ",")
	tableName, _ := fullName(user, table)

	rows, err := db.Query(`select permissions from admin.privileges where username=$1 and tablename=$2`, user, tableName)
	if err != nil {
		return errors.New(err).Context(user + ":" + tableName)
	}

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
		if len(permissions) > 0 {
			permissions = permissions + ","
		}
		permissions = permissions + key
	}

	// Attempt to update the permissions.
	var result sql.Result
	context := "updating permissions"
	result, err = db.Exec(`update admin.privileges set permissions=$1 where username=$2 and tablename=$3`, permissions, user, tableName)
	if err == nil {
		if rowCount, _ := result.RowsAffected(); rowCount == 0 {
			context = "adding permissions"
			_, err = db.Exec(`insert into admin.privileges(permissions, username, tablename) values($1,$2,$3)`, permissions, user, tableName)
		}
	}

	if err != nil {
		return errors.New(err).Context(context)
	}

	return nil

}

func ReadPermissions(user string, hasAdminPermission bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	db, err := OpenDB(sessionID, user, "")
	if err != nil {
		ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

		return
	}

	_, _ = db.Exec(createPermissionString)

	table, fullyQualified := fullName(user, tableName)
	if !hasAdminPermission && !fullyQualified {
		ErrorResponse(w, sessionID, "Not authorized to read permissions", http.StatusForbidden)

		return
	}

	reply := defs.PermissionResponse{}
	parts := tableNameParts(user, table)
	reply.User = user
	reply.Schema = parts[0]
	reply.Table = parts[1]

	rows, err := db.Query(permissionsSelectString, user, table)
	if err != nil {
		ui.Debug(ui.ServerLogger, "[%d] Error reading permissions field: %v", sessionID, err)
		ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)
		return
	}

	permissionsMap := map[string]bool{}

	for rows.Next() {
		permissionString := ""
		_ = rows.Scan(&permissionString)
		ui.Debug(ui.ServerLogger, "[%d] Read permissions field: %v", sessionID, permissionString)

		for _, perm := range strings.Split(strings.ToLower(permissionString), ",") {
			permissionsMap[strings.TrimSpace(perm)] = true
		}
	}

	reply.Permissions = make([]string, 0)
	for k := range permissionsMap {
		reply.Permissions = append(reply.Permissions, k)
	}

	sort.Strings(reply.Permissions)
	reply.Status = http.StatusOK
	w.WriteHeader(http.StatusOK)

	b, _ := json.MarshalIndent(reply, "", "  ")
	_, _ = w.Write(b)
}

func ReadAllPermissions(db *sql.DB, sessionID int32, w http.ResponseWriter, r *http.Request) {
	_, _ = db.Exec(createPermissionString)

	reply := defs.AllPermissionResponse{
		Permissions: []defs.PermissionObject{},
	}

	rows, err := db.Query(`SELECT username, tablename, permissions FROM admin.privileges ORDER BY username,tablename`)
	if err != nil {
		ui.Debug(ui.ServerLogger, "[%d] Error reading permissions: %v", sessionID, err)
		ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

		return
	}

	count := 0
	for rows.Next() {
		var user, table, permissionString string

		permObject := defs.PermissionObject{}
		permissionsMap := map[string]bool{}
		count = count + 1

		err = rows.Scan(&user, &table, &permissionString)
		if err != nil {
			ui.Debug(ui.ServerLogger, "[%d] Error scanning permissions: %v", sessionID, err)
			ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

			return
		}

		for _, perm := range strings.Split(strings.ToLower(permissionString), ",") {
			permissionsMap[strings.TrimSpace(perm)] = true
		}

		permObject.Permissions = make([]string, 0)
		for k := range permissionsMap {
			permObject.Permissions = append(permObject.Permissions, k)
		}

		sort.Strings(permObject.Permissions)

		parts := tableNameParts(user, table)
		permObject.User = user
		permObject.Schema = parts[0]
		permObject.Table = parts[1]

		reply.Permissions = append(reply.Permissions, permObject)
	}

	reply.Status = http.StatusOK
	reply.Count = count
	w.WriteHeader(http.StatusOK)

	b, _ := json.MarshalIndent(reply, "", "  ")
	_, _ = w.Write(b)
}

func GrantPermissions(user string, hasAdminPermission bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	db, err := OpenDB(sessionID, user, "")
	if err != nil {
		ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

		return
	}
	_, _ = db.Exec(createPermissionString)

	table, fullyQualified := fullName(user, tableName)
	if !hasAdminPermission && !fullyQualified {
		ErrorResponse(w, sessionID, "Not authorized to update permissions", http.StatusForbidden)

		return
	}

	permissionsList := []string{}

	err = json.NewDecoder(r.Body).Decode(&permissionsList)
	if err != nil {
		ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

		return
	}

	var buff strings.Builder
	for i, key := range permissionsList {
		if i > 0 {
			buff.WriteRune(',')
		}
		buff.WriteString(strings.TrimSpace(strings.ToLower(key)))
	}

	err = grantPermissions(sessionID, db, user, table, buff.String())

	if !errors.Nil(err) {
		ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)
		return
	}

	ReadPermissions(user, hasAdminPermission, table, sessionID, w, r)
}

func DeletePermissions(user string, hasAdminPermission bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	db, err := OpenDB(sessionID, user, "")
	if err != nil {
		ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

	}
	_, _ = db.Exec(createPermissionString)

	table, fullyQualified := fullName(user, tableName)
	if !hasAdminPermission && !fullyQualified {
		ErrorResponse(w, sessionID, "Not authorized to delete permissions", http.StatusForbidden)

		return
	}

	_, err = db.Exec(`DELETE FROM admin.privileges where username=$1 and tablename=$2`, user, table)
	if err != nil {
		ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}
