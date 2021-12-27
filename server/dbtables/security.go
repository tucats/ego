package dbtables

import (
	"database/sql"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
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
	_, _ = db.Exec(permissionsCreateTableQuery)

	rows, err := db.Query(permissionsSelectQuery, user, table)
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
	_, _ = db.Exec(permissionsCreateTableQuery)
	result, err := db.Exec(permissionsDeleteAllQuery, table)
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
	_, _ = db.Exec(permissionsCreateTableQuery)

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
	_, err := db.Exec(permissionsDeleteAllQuery, user, table)
	if err != nil {
		ui.Debug(ui.ServerLogger, "[%d] Error updating permissions: %v", sessionID, err)

		return false
	}

	_, err = db.Exec(permissionsInsertQuery, user, table, permissionList)
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

	ui.Debug(ui.ServerLogger, "[%d] Attempting to set %s permissions for %s to %s", sessionID, user, tableName, permissionNames)

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
	result, err = db.Exec(permissionsUpdateQuery, user, tableName, permissions)
	if err == nil {
		if rowCount, _ := result.RowsAffected(); rowCount == 0 {
			context = "adding permissions"
			_, err = db.Exec(permissionsInsertQuery, user, tableName, permissions)
		}
	}

	if err != nil {
		return errors.New(err).Context(context)
	}

	return nil

}
