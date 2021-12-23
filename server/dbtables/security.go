package dbtables

import (
	"database/sql"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
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
