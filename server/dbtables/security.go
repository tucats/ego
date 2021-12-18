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

	permissionsTable = "admin.privileges"
)

// authorized uses the database located in the the Ego tables database
// to determine if the proposed operation is permitted for the given table.
//
// The permissions string for the table and user is read, if it exists,
// must contain the given permission.
func Authorized(sessionID int32, db *sql.DB, user string, table string, operations ...string) bool {

	q := "SELECT permissions FROM $1 WHERE username = $2 and tablename = $3"
	rows, err := db.Query(q, permissionsTable, user, table)
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

	q := "delete FROM $1 WHERE tablename = $2"
	result, err := db.Exec(q, permissionsTable, table)
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
	var permissionList string

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

	q := "insert into $1 (username, tablename, permissions) values($2, $3, $4)"
	_, err := db.Exec(q, permissionsTable, user, table, permissionList)
	if err != nil {
		ui.Debug(ui.ServerLogger, "[%d] Error updating permissions: %v", sessionID, err)

		return false
	}

	ui.Debug(ui.ServerLogger, "[%d] permissions for %s set to %s", sessionID, table, permissionList)

	return true
}
