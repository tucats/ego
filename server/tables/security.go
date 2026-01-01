package tables

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/resources"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables/parsing"
	"github.com/tucats/ego/util"
)

const (
	readOperation   = "read"
	adminOperation  = "admin"
	updateOperation = "update"
	writeOperation  = "write"
	deleteOperation = "delete"
)

type PermissionsObject struct {
	ID     string `json:"id"`
	User   string `json:"user_name"`
	DSN    string `json:"dsn_name"`
	Table  string `json:"table_name"`
	Admin  bool   `json:"admin_perm"`
	Read   bool   `json:"read_perm"`
	Write  bool   `json:"write_perm"`
	Update bool   `json:"update_perm"`
	Delete bool   `json:"delete_perm"`
}

// This handle is used to manage access to the permissions table.
var pHandle *resources.ResHandle

// This flag indicates if the permissions system is initialized and valid.
var pValid bool

// Form the connection string to the permissions database. This is build using
// the default system catalog which contains the user database. If the persistence
// isn't a database, and this returns an empty string.
func permissionConstr() string {
	constr := settings.Get(defs.LogonUserdataSetting)
	if constr == "" {
		path := settings.Get(defs.EgoPathSetting)
		constr = defs.DefaultUserdataScheme + "://" + filepath.Join(path, defs.DefaultUserdataFileName)
	}

	if !strings.Contains(constr, "://") {
		return ""
	}

	parts := strings.SplitN(constr, "://", 2)

	keys := []string{}
	for key := range providers {
		keys = append(keys, key)
	}

	// Remap any scheme aliases
	parts[0] = providers[strings.ToLower(parts[0])]

	return strings.Join(parts, "://")
}

// Given a list of permission strings, indicate if they are all valid. The permission
// string array elements can optionally have a prefix character "+" indicating the
// permission is granted or "-" indicating the permission is revoked.
func validPermissions(perms []string) bool {
	for _, perm := range perms {
		perm = strings.TrimSpace(perm)
		if perm == "" {
			continue
		}

		// Strip off the grant/revoke flag if present
		switch perm[0] {
		case '+':
			perm = perm[1:]
		case '-':
			perm = perm[1:]
		}

		// The resulting permission name must match one of the permitted names.
		if !util.InList(strings.ToLower(perm),
			readOperation,
			writeOperation,
			adminOperation,
			updateOperation,
			deleteOperation,
		) {
			return false
		}
	}

	return true
}

// ReadPermissions reads the permissions data for a specific table. This operation requires either ownership
// of the table or admin privileges. The response is a Permission object for the given user, dsn, and table.
func ReadPermissions(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	tableName := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])

	userName := session.User
	if users := session.Parameters["user"]; len(users) == 1 {
		userName = users[0]
	}

	if !initPermissions() {
		err := errors.ErrPermissionsUnavailable.Clone().Context(dsnName + "." + tableName)

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	list, err := pHandle.Read(
		pHandle.Equals("user", userName),
		pHandle.Equals("dsn", dsnName),
		pHandle.Equals("table", tableName))

	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// Construct a reply object to hold the requested permissions. Fill it to include the
	// table schema and name.
	reply := defs.PermissionObject{}
	reply.User = userName
	reply.DSNName = dsnName
	reply.Table = tableName

	// Read all the matching rows and populate the permissionsMap, which enumerates the permissions
	// granted. The table will contain only entries where the user has permissions. IF this operation
	// is for the current user and the current user is an administrator, all permissions are granted.
	perms := []string{}
	if userName == session.User && session.Admin {
		perms = append(perms, adminOperation)
		perms = append(perms, readOperation)
		perms = append(perms, writeOperation)
		perms = append(perms, deleteOperation)
		perms = append(perms, updateOperation)
	} else {
		for _, item := range list {
			perm := item.(*PermissionsObject)

			if perm.Admin {
				perms = append(perms, adminOperation)
			}

			if perm.Read {
				perms = append(perms, readOperation)
			}

			if perm.Write {
				perms = append(perms, writeOperation)
			}

			if perm.Delete {
				perms = append(perms, deleteOperation)
			}

			if perm.Update {
				perms = append(perms, updateOperation)
			}
		}
	}

	permissionString := strings.Join(perms, ",")

	ui.Log(ui.TableLogger, "table.permissions", ui.A{
		"session":    session.ID,
		"user":       parsing.StripQuotes(userName),
		"dsn":        parsing.StripQuotes(dsnName),
		"table":      parsing.StripQuotes(tableName),
		"permission": permissionString})

	// Fill the reply with the permission(s) found in the database.
	reply.Permissions = perms

	// Sort the permissions array so the results are always consistent regardless of
	// the map iteration from the data collected.
	sort.Strings(reply.Permissions)

	// Convert the result to JSON and write to the response payload and we are done.
	w.Header().Set("Content-Type", defs.JSONMediaType)
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
	dsnName := data.String(session.URLParts["dsn"])
	if dsnName == "@all" {
		dsnName = ""
	}

	if !initPermissions() {
		err := errors.ErrPermissionsUnavailable.Clone()

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	reply := defs.AllPermissionResponse{
		Permissions: []defs.PermissionObject{},
	}

	var nameFilter, dsnFilter *resources.Filter

	if f := parsing.RequestForUser("", r.URL); f != "" {
		text, err := parsing.SQLEscape(f)
		if err != nil {
			return util.ErrorResponse(w, session.ID, "Invalid filter", http.StatusBadRequest)
		}

		nameFilter = pHandle.Equals("name", text)
	}

	if dsnName != "" {
		text, err := parsing.SQLEscape(dsnName)
		if err != nil {
			return util.ErrorResponse(w, session.ID, "Invalid filter", http.StatusBadRequest)
		}

		dsnFilter = pHandle.Equals("dsn", text)
	}

	list, err := pHandle.Read(dsnFilter, nameFilter)

	if err != nil {
		ui.Log(ui.TableLogger, "table.read.error", ui.A{
			"session": session.ID,
			"error":   err.Error()})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	count := 0

	for _, item := range list {
		var (
			permObject = defs.PermissionObject{}
		)

		p := item.(*PermissionsObject)
		if p == nil {
			continue
		}

		permissions := []string{}

		if session.Admin {
			permissions = append(permissions, adminOperation)
		}

		if p.Read {
			permissions = append(permissions, readOperation)
		}

		if p.Write {
			permissions = append(permissions, writeOperation)
		}

		if p.Update {
			permissions = append(permissions, updateOperation)
		}

		if p.Delete {
			permissions = append(permissions, deleteOperation)
		}

		permObject.Permissions = permissions

		sort.Strings(permObject.Permissions)

		permObject.User = p.User
		permObject.DSNName = p.DSN
		permObject.Table = p.Table

		reply.Permissions = append(reply.Permissions, permObject)
		count = count + 1
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
	if !initPermissions() {
		err := errors.ErrPermissionsUnavailable.Clone()

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	tableName := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])
	user := session.User

	if users := session.Parameters["user"]; len(users) == 1 {
		user = users[0]
	}

	items, err := pHandle.Read(
		pHandle.Equals("user", user),
		pHandle.Equals("table", tableName),
		pHandle.Equals("dsn", dsnName))

	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// If there are no permissions existing, let's create a new one. If more than one was found,
	// an ambiguous entry was found, which is an error.
	if len(items) != 1 {
		if len(items) == 0 {
			permObject := &PermissionsObject{
				ID:    uuid.NewString(),
				User:  user,
				Table: tableName,
				DSN:   dsnName,
			}
			items = append(items, permObject)

			err = pHandle.Insert(permObject)
			if err != nil {
				return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
			}
		} else {
			err = errors.ErrPermissionsUnavailable.Clone().Context(dsnName + "." + tableName)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusNotFound)
		}
	}

	item := items[0].(*PermissionsObject)

	permissionsList := []string{}

	if err = json.NewDecoder(r.Body).Decode(&permissionsList); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	sort.Strings(permissionsList)

	if !validPermissions(permissionsList) {
		return util.ErrorResponse(w, session.ID, fmt.Sprintf("invalid permissions list: %s", permissionsList), http.StatusBadRequest)
	}

	// Set the flags in the permission object based on the permission strings. Strip off any +/- prefixes,
	// but use them to set the settings flag appropriately.
	for _, key := range permissionsList {
		setting := true
		if key[0] == '-' {
			setting = false
			key = key[1:]
		} else {
			if key[0] == '+' {
				key = key[1:]
			}
		}

		switch strings.ToLower(key) {
		case readOperation:
			item.Read = setting
		case updateOperation:
			item.Update = setting
		case deleteOperation:
			item.Write = setting
		case writeOperation:
			item.Delete = setting
		case adminOperation:
			item.Admin = setting
		default:
			return util.ErrorResponse(w, session.ID, fmt.Sprintf("invalid permission: %s", key), http.StatusBadRequest)
		}
	}

	err = pHandle.Update(item)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	return ReadPermissions(session, w, r)
}

// DeletePermissions deletes one or more permissions records for a given username, dsn, and table.
func DeletePermissions(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	dsnName := data.String(session.URLParts["dsn"])
	if dsnName == "@all" {
		dsnName = ""
	}

	tableName := data.String(session.URLParts["table"])

	if !initPermissions() {
		err := errors.ErrPermissionsUnavailable.Clone()

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	var nameFilter, dsnFilter, tableFilter *resources.Filter

	if f := parsing.RequestForUser("", r.URL); f != "" {
		text, err := parsing.SQLEscape(f)
		if err != nil {
			return util.ErrorResponse(w, session.ID, "Invalid filter", http.StatusBadRequest)
		}

		nameFilter = pHandle.Equals("name", text)
	}

	if tableName != "" {
		tableFilter = pHandle.Equals("table", tableName)
	}

	if dsnName != "" {
		text, err := parsing.SQLEscape(dsnName)
		if err != nil {
			return util.ErrorResponse(w, session.ID, "Invalid filter", http.StatusBadRequest)
		}

		dsnFilter = pHandle.Equals("dsn", text)
	}

	list, err := pHandle.Read(dsnFilter, tableFilter, nameFilter)

	if err != nil {
		ui.Log(ui.TableLogger, "table.read.error", ui.A{
			"session": session.ID,
			"error":   err.Error()})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	count := 0

	for _, item := range list {
		p := item.(*PermissionsObject)
		if p == nil {
			continue
		}

		_, err := pHandle.Delete(pHandle.Equals("id", p.ID))
		if err != nil {
			ui.Log(ui.TableLogger, "table.delete.error", ui.A{
				"session": session.ID,
				"error":   err.Error()})

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
		}

		count = count + 1
	}

	ui.Log(ui.TableLogger, "table.perms.deleted", ui.A{
		"session": session.ID,
		"dsn":     dsnName,
		"count":   count})

	return http.StatusOK
}

// Authorized uses the system database to determine if the proposed operation is permitted
// for the given table. This only applies for tables with a DSN that is marked as "secured".
// By default, DSNS are not secured and depend on the underlying provider to handle all role
// and permissions checks. If a DSN is considered secured, then before the provider is even
// contacted, we verify if the user/dsn/table and operation are authorized.
func Authorized(session *server.Session, user string, table string, operations ...string) bool {
	dsn := ""

	// IF the session authentication is for the given user and that user is an admin, then allow any operation.
	if user == session.User && session.Admin {
		return true
	}

	if strings.Contains(table, ".") {
		parts := strings.SplitN(table, ".", 2)
		dsn = parts[0]
		table = parts[1]
	}

	// IS this a valid DSN name? If not, no access allowed.
	dsnName, err := dsns.DSNService.ReadDSN(session.ID, user, dsn, false)
	if err != nil {
		return false
	}

	// IF this DSN does not use security, then allow any operation.
	if !dsnName.Secured {
		return true
	}

	// If we the permissions subsystem is not initialized, then allow any operation.
	if !initPermissions() {
		return true
	}

	items, err := pHandle.Read(
		pHandle.Equals("dsn", dsn),
		pHandle.Equals("table", table),
		pHandle.Equals("name", user))
	if err != nil {
		ui.Log(ui.TableLogger, "table.read.error", ui.A{
			"session": session.ID,
			"error":   err})

		return false
	}

	if len(items) != 1 {
		return false
	}

	perm := items[0].(*PermissionsObject)
	auth := true

	for _, operation := range operations {
		switch strings.ToLower(operation) {
		case readOperation:
			if !perm.Read {
				auth = false
			}

		case writeOperation:
			if !perm.Write {
				auth = false
			}

		case adminOperation:
			if !perm.Admin {
				auth = false
			}

		case deleteOperation:
			if !perm.Delete {
				auth = false
			}

		case updateOperation:
			if !perm.Update {
				auth = false
			}

		default:
			auth = false
		}
	}

	if ui.IsActive(ui.TableLogger) {
		if !auth {
			ui.WriteLog(ui.TableLogger, "table.no.auth", ui.A{
				"session": session.ID,
				"user":    user,
				"perms":   operations,
				"table":   table})
		} else {
			ui.WriteLog(ui.TableLogger, "table.auth", ui.A{
				"session": session.ID,
				"user":    user,
				"perms":   operations,
				"table":   table})
		}
	}

	return auth
}

// createTablePermissions creates an entry in the permissions data for this
// user, dsn, and table. Because the create is being done by the user, the
// owner of the table gets all permissions.
func createTablePermissions(session *server.Session, user, dsn, table string) bool {
	if !initPermissions() {
		return false
	}

	p := PermissionsObject{
		ID:     uuid.NewString(),
		User:   user,
		DSN:    dsn,
		Table:  table,
		Read:   true,
		Write:  true,
		Admin:  true,
		Delete: true,
		Update: true,
	}

	err := pHandle.Insert(&p)
	if err == nil {
		ui.Log(ui.TableLogger, "table.perms.create", ui.A{
			"session": session.ID,
			"user":    user,
			"dsn":     dsn,
			"table":   table,
		})
	} else {
		ui.Log(ui.TableLogger, "table.perms.create.error", ui.A{
			"session": session.ID,
			"user":    user,
			"dsn":     dsn,
			"table":   table,
			"error":   err.Error(),
		})
	}

	return err == nil
}

// removeTablePermissions updates the permissions data to remove references to
// the named table. This is done when a table is deleted.
func removeTablePermissions(session *server.Session, table string) bool {
	dsnName := data.String(session.URLParts["dsn"])
	tableName := table

	if !initPermissions() {
		return false
	}

	var dsnFilter, tableFilter *resources.Filter

	if tableName != "" {
		tableFilter = pHandle.Equals("table", tableName)
	}

	if dsnName != "" {
		text, err := parsing.SQLEscape(dsnName)
		if err != nil {
			return false
		}

		dsnFilter = pHandle.Equals("dsn", text)
	}

	count, err := pHandle.Delete(dsnFilter, tableFilter)
	if err != nil {
		ui.Log(ui.TableLogger, "table.read.error", ui.A{
			"session": session.ID,
			"error":   err})

		return false
	}

	if count == 0 {
		return false
	} else {
		ui.Log(ui.TableLogger, "table.perms.deleted", ui.A{
			"session": session.ID,
			"count":   count,
			"table":   table})
	}

	return true
}

func initPermissions() bool {
	if !pValid {
		constr := permissionConstr()
		if constr != "" {
			var err error

			pHandle, err = resources.Open(PermissionsObject{}, "table_perms", constr)
			if err == nil {
				err = pHandle.CreateIf()
				if err == nil {
					pValid = true
				}
			}
		}
	}

	return pValid
}
