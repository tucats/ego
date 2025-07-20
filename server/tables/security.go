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
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
	"github.com/tucats/ego/util"
)

const (
	readOperation   = "read"
	deleteOperation = "delete"
	adminOperation  = "admin"
	updateOperation = "update"
)

// Given a list of permission strings, indicate if they are all valid. The permission
// string array elements can optionally have a prefix character "+" indicating the
// permission is granted or "-" indicating the permission is revoked.
func validPermissions(perms []string) bool {
	for _, perm := range perms {
		// Strip off the grant/revoke flag if present
		switch perm[:1] {
		case "+":
			perm = perm[1:]
		case "-":
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

	// Open the database connection on behalf of the session user.
	db, err := database.Open(session, "", 0)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// Create the permissions table if it doesn't exist.
	_, _ = db.Exec(permissionsCreateTableQuery)

	// Given the table name provided and the requesting user, determine the short and fully
	// qualified table name.
	table, fullyQualified := parsing.FullName(session.User, tableName)
	if !session.Admin && !fullyQualified {
		return util.ErrorResponse(w, session.ID, "Not authorized to read permissions", http.StatusForbidden)
	}

	// Construct a reply object to hold the requested permissions. Fill it to include the
	// table schema and name.
	reply := defs.PermissionObject{}
	parts := parsing.TableNameParts(session.User, table)
	reply.User = session.User
	reply.Schema = parts[0]
	reply.Table = parts[1]

	// Attempt to read the permission data from the database for the given user name and table name.
	rows, err := db.Query(permissionsSelectQuery, parsing.StripQuotes(session.User), parsing.StripQuotes(table))
	if err != nil {
		defer rows.Close()

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	permissionsMap := map[string]bool{}

	// Read all the matching rows and populate the permissionsMap, which enumerates the permissions
	// granted. The table will contain only entries where the user has permissions.
	for rows.Next() {
		permissionString := ""
		_ = rows.Scan(&permissionString)
		ui.Log(ui.TableLogger, "table.permissions", ui.A{
			"session":    session.ID,
			"user":       parsing.StripQuotes(session.User),
			"table":      parsing.StripQuotes(table),
			"permission": permissionString})

		for _, perm := range strings.Split(strings.ToLower(permissionString), ",") {
			permissionsMap[strings.TrimSpace(perm)] = true
		}
	}

	// Fill the reply with the permission(s) found in the database.
	reply.Permissions = make([]string, 0)
	for k := range permissionsMap {
		reply.Permissions = append(reply.Permissions, k)
	}

	// Sort the permissions array so the results are always consistent regardless of
	// the map iteration from the data collected.
	sort.Strings(reply.Permissions)

	// Convert the result to JSON and write to the response payload and we are done.
	w.WriteHeader(http.StatusOK)

	b, _ := json.MarshalIndent(reply, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// ReadAllPermissions reads all permissions for all tables. By default it is for all users, though you can use the
// ?user= parameter to specify permissions for a given user for all tables. The result is an array of permissions
// objects for each permutation of owner and table name visible to the user.
func ReadAllPermissions(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	db, err := database.Open(session, "", 0)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	_, _ = db.Exec(permissionsCreateTableQuery)

	reply := defs.AllPermissionResponse{
		Permissions: []defs.PermissionObject{},
	}

	filter := ""

	if f := parsing.RequestForUser("", r.URL); f != "" {
		text, err := parsing.SQLEscape(f)
		if err != nil {
			return util.ErrorResponse(w, session.ID, "Invalid filter", http.StatusBadRequest)
		}

		filter = fmt.Sprintf("WHERE username = '%s'", text)
	}

	q := fmt.Sprintf(`SELECT username, tablename, permissions FROM admin.privileges %s ORDER BY username,tablename`, filter)

	rows, err := db.Query(q)
	if err != nil {
		defer rows.Close()
		ui.Log(ui.TableLogger, "table.read.error", ui.A{
			"session": session.ID,
			"sql":     q,
			"error":   err.Error()})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	count := 0

	for rows.Next() {
		var (
			user, table, permissionString string
			permObject                    = defs.PermissionObject{}
			permissionsMap                = map[string]bool{}
		)

		count = count + 1

		if err = rows.Scan(&user, &table, &permissionString); err != nil {
			ui.Log(ui.TableLogger, "table.read.error", ui.A{
				"session": session.ID,
				"query":   q,
				"error":   err.Error()})

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
	reply.Status = http.StatusOK

	w.WriteHeader(http.StatusOK)

	b, _ := json.MarshalIndent(reply, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// GrantPermissions is used to grant and revoke permissions. The Request must be a JSON array of strings, each of which is
// a permission to be granted or revoked. The permissions is revoked if it starts with a "-" character, else it is granted.
// You must be the owner of the table or an admin user to perform this operation.
func GrantPermissions(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var buff strings.Builder

	tableName := data.String(session.URLParts["table"])

	db, err := database.Open(session, "", 0)
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

	for i, key := range permissionsList {
		if i > 0 {
			buff.WriteRune(',')
		}

		buff.WriteString(strings.TrimSpace(strings.ToLower(key)))
	}

	err = grantPermissions(session.ID, db, user, table, buff.String())

	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	return ReadPermissions(session, w, r)
}

// DeletePermissions deletes one or permissions records for a given username and table. The permissions data is deleted completely,
// which means this table will only be visible to admin users.
func DeletePermissions(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	db, err := database.Open(session, "", 0)
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

// Authorized uses the database located in the Ego tables database to determine if the
// proposed operation is permitted for the given table. This only applies for tables
// accessed in the "default" database, if configued. Authorization tests for databases
// accessed via a DSN are always allowed.
//
// The permissions string for the table and user is read and must contain the given permission.
func Authorized(db *database.Database, user string, table string, operations ...string) bool {
	// If this is opened via a DSN, then we don't apply permissions. All operations are allowed
	// at the server level, and we depend on the underlying database to enforce permissions.
	if db.DSN != "" {
		return true
	}

	_, err := db.Exec(permissionsCreateTableQuery)
	if err != nil {
		ui.Log(ui.TableLogger, "table.query.error", ui.A{
			"session": db.Session.ID,
			"query":   permissionsCreateTableQuery,
			"error":   err})
	}

	table, _ = parsing.FullName(user, table)

	rows, err := db.Query(permissionsSelectQuery, parsing.StripQuotes(user), parsing.StripQuotes(table))
	if err != nil {
		ui.Log(ui.TableLogger, "table.read.error", ui.A{
			"session": db.Session.ID,
			"query":   permissionsSelectQuery,
			"error":   err})

		return false
	}

	if !rows.Next() {
		return false
	}

	permissions := ""

	if err := rows.Scan(&permissions); err != nil {
		ui.Log(ui.TableLogger, "table.read.error", ui.A{
			"session": db.Session.ID,
			"query":   permissionsSelectQuery,
			"error":   err})

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
		if !auth {
			ui.WriteLog(ui.TableLogger, "table.no.auth", ui.A{
				"session": db.Session.ID,
				"user":    user,
				"perms":   operations,
				"table":   table})
		} else {
			ui.WriteLog(ui.TableLogger, "table.auth", ui.A{
				"session": db.Session.ID,
				"user":    user,
				"perms":   operations,
				"table":   table})
		}
	}

	return auth
}

// RemoveTablePermissions updates the permissions data to remove references to
// the named table.
func RemoveTablePermissions(sessionID int, db *database.Database, table string) bool {
	// If this is opened via a DSN, then we don't apply permissions. All operations are allowed
	// at the server level, and we depend on the underlying database to enforce permissions.
	if db.DSN != "" {
		return true
	}

	_, _ = db.Exec(permissionsCreateTableQuery)

	result, err := db.Exec(permissionsDeleteAllQuery, parsing.StripQuotes(table))
	if err != nil {
		ui.Log(ui.TableLogger, "table.read.error", ui.A{
			"session": sessionID,
			"query":   permissionsDeleteAllQuery,
			"error":   err})

		return false
	}

	if count, err := result.RowsAffected(); err != nil || count == 0 {
		return false
	} else {
		ui.Log(ui.TableLogger, "table.perms.deleted", ui.A{
			"session": sessionID,
			"count":   count,
			"table":   table})
	}

	return true
}

// CreateTablePermissions creates a row for the permissions data for a given user and named table, with
// the permissions enumerated as the last parameters.
func CreateTablePermissions(sessionID int, db *database.Database, user, table string, permissions ...string) bool {
	// If this is opened via a DSN, then we don't apply permissions. All operations are allowed
	// at the server level, and we depend on the underlying database to enforce permissions.
	if db.DSN != "" {
		return true
	}

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

func doCreateTablePermissions(sessionID int, db *database.Database, user, table string, permissions ...string) bool {
	// If this is opened via a DSN, then we don't apply permissions. All operations are allowed
	// at the server level, and we depend on the underlying database to enforce permissions.
	if db.DSN != "" {
		return true
	}

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
	// adding in the new one. We ignore any errors, etc. from this first operation
	// since any errors in the table or operation will be reported in the second
	// insert operation.
	_, _ = db.Exec(permissionsDeleteQuery, parsing.StripQuotes(user), parsing.StripQuotes(table))

	_, err := db.Exec(permissionsInsertQuery, parsing.StripQuotes(user), parsing.StripQuotes(table), permissionList)
	if err != nil {
		ui.Log(ui.TableLogger, "table.query.error", ui.A{
			"session": sessionID,
			"query":   permissionsInsertQuery,
			"error":   err.Error()})

		return false
	}

	return true
}

func grantPermissions(sessionID int, db *database.Database, user string, table string, permissions string) error {
	var (
		result            sql.Result
		permissionsString string
		context           = "updating permissions"
		permMap           = map[string]bool{}
	)

	// If this is opened via a DSN, then we don't apply permissions. All operations are allowed
	// at the server level, and we depend on the underlying database to enforce permissions.
	if db.DSN != "" {
		return nil
	}

	// Decompose the permissions list
	permissionNames := strings.Split(permissions, ",")
	tableName, _ := parsing.FullName(user, table)

	sort.Strings(permissionNames)

	q := `select permissions from admin.privileges where username=$1 and tablename=$2`

	rows, err := db.Query(q, parsing.StripQuotes(user), parsing.StripQuotes(tableName))
	if err != nil {
		ui.Log(ui.TableLogger, "table.query.error", ui.A{
			"session": sessionID,
			"query":   q,
			"error":   err})

		return errors.New(err).Context(user + ":" + tableName)
	}

	defer rows.Close()

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
	result, err = db.Exec(permissionsUpdateQuery, parsing.StripQuotes(user), parsing.StripQuotes(tableName), permissions)
	if err == nil {
		if rowCount, _ := result.RowsAffected(); rowCount == 0 {
			context = "adding permissions"

			_, err = db.Exec(permissionsInsertQuery, parsing.StripQuotes(user), parsing.StripQuotes(tableName), permissions)
		}
	}

	if err != nil {
		return errors.New(err).Context(context)
	}

	return nil
}
