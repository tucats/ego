package dbtables

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

func ReadPermissions(user string, hasAdminPermission bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	db, err := OpenDB(sessionID, user, "")
	if err != nil {
		ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

		return
	}

	_, _ = db.Exec(permissionsCreateTableQuery)

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

	rows, err := db.Query(permissionsSelectQuery, user, table)
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
	_, _ = db.Exec(permissionsCreateTableQuery)

	reply := defs.AllPermissionResponse{
		Permissions: []defs.PermissionObject{},
	}

	filter := ""
	if f := requestForUser("", r.URL); f != "" {
		filter = fmt.Sprintf("WHERE username = '%s'", sqlEscape(f))
	}

	q := fmt.Sprintf(`SELECT username, tablename, permissions FROM admin.privileges %s ORDER BY username,tablename`, filter)
	ui.Debug(ui.ServerLogger, "[%d] Query: %s", sessionID, q)
	rows, err := db.Query(q)
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
	_, _ = db.Exec(permissionsCreateTableQuery)

	user = requestForUser(user, r.URL)
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
	_, _ = db.Exec(permissionsCreateTableQuery)

	table, fullyQualified := fullName(user, tableName)
	if !hasAdminPermission && !fullyQualified {
		ErrorResponse(w, sessionID, "Not authorized to delete permissions", http.StatusForbidden)

		return
	}

	_, err = db.Exec(permissionsDeleteQuery, user, table)
	if err != nil {
		ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}
