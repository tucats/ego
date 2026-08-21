package tables

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/resources"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/tables/parsing"
	"github.com/tucats/ego/internal/util"
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
			defs.TableReadPermission,
			defs.TableWritePermission,
			defs.TableAdminPermission,
			defs.TableUpdatePermission,
			defs.TableDeletePermission,
		) {
			return false
		}
	}

	return true
}

// authorizedForTablePermissions reports whether the caller may administer
// (read, list, grant, or revoke) the table_perms grants for the named table
// -- that is, whether they are allowed to reach any of the four handlers
// below (ReadPermissions, ReadTablePermissions, GrantPermissions,
// DeletePermissions) at all.
//
// DATA-SECURITY-2.md finding #4: docs/SERVER.md describes ego.table.admin
// as existing specifically so that "table administration is granted per
// table, by whoever already administers it or the DSN it lives in" -- but
// until this function existed, none of the four handlers checked anything
// beyond the route-level Permissions(defs.DSNAdminPermission) gate
// (routes.go), which -- like every other Route.Permissions() check in this
// codebase -- only ever looks at the caller's *identity-wide* permissions.
// A caller who held ego.table.admin on this one table (granted, for
// example, automatically to whoever created it -- see
// createTablePermissions below) but no identity-wide ego.dsn.admin was
// rejected before any of these four handlers ever ran, making
// ego.table.admin unusable for the exact purpose docs/SERVER.md says it
// exists for.
//
// This mirrors the same identity-OR-per-DSN-admin pattern used by
// DeleteDSNHandler and, since DATA-SECURITY-2.md findings #2/#3, by
// TableCreate/DeleteTable -- with one more link added at the end: a caller
// who is specifically an admin of *this table* (via table_perms, checked by
// Authorized() below) is now also accepted, which the DSN-level handlers
// have no equivalent of, because they have no notion of a resource smaller
// than a whole DSN.
//
// Go note for readers new to the language: "||" is the boolean OR operator.
// Each of the four conditions below is tried in turn, and Go stops (without
// evaluating any of the remaining ones) as soon as one of them is true --
// this is called "short-circuit evaluation". So for the overwhelmingly
// common case (an ordinary, non-admin caller checking their own table),
// only the cheap in-memory checks (session.Admin, IdentityAuthorizesAction)
// run before the two checks that each require a database read
// (AuthDSN, Authorized) -- and even those two stop as soon as either one
// succeeds.
func authorizedForTablePermissions(session *router.Session, dsnName, tableName string) bool {
	return session.Admin ||
		dsns.IdentityAuthorizesAction(session.Permissions, dsns.DSNAdminAction) ||
		dsns.DSNService.AuthDSN(session.ID, session.User, dsnName, dsns.DSNAdminAction) ||
		Authorized(session, session.User, dsnName+"."+tableName, defs.TableAdminPermission)
}

// ReadPermissions reads the permissions data for a specific table. This operation requires either ownership
// of the table or admin privileges. The response is a Permission object for the given user, dsn, and table.
func ReadPermissions(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	tableName := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])

	// DATA-SECURITY-2.md finding #4: this route (routes.go) no longer
	// requires identity-wide ego.dsn.admin on its own -- see
	// authorizedForTablePermissions' doc comment above for the full
	// explanation of what else now satisfies this check and why.
	if !authorizedForTablePermissions(session, dsnName, tableName) {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.perm.privilege", ui.A{"permission": defs.TableAdminPermission}), http.StatusForbidden)
	}

	userName := session.User
	if users := session.Parameters["user"]; len(users) == 1 {
		userName = users[0]
	}

	if !initPermissions() {
		err := errors.ErrPermissionsUnavailable.Clone().Context(dsnName + "." + tableName)

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	list, err := pHandle.Read(
		pHandle.Equals("user", userName),
		pHandle.Equals("dsn", dsnName),
		pHandle.Equals("table", tableName))

	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	// Construct a response object to hold the requested permissions. Fill it to include the
	// table schema and name.
	response := defs.PermissionObject{}
	response.User = userName
	response.DSNName = dsnName
	response.Table = tableName

	// Read all the matching rows and populate the permissionsMap, which enumerates the permissions
	// granted. The table will contain only entries where the user has permissions. IF this operation
	// is for the current user and the current user is an administrator, all permissions are granted.
	perms := []string{}
	if userName == session.User && session.Admin {
		perms = append(perms, defs.TableReadPermission)
		perms = append(perms, defs.TableWritePermission)
		perms = append(perms, defs.TableUpdatePermission)
		perms = append(perms, defs.TableDeletePermission)
		perms = append(perms, defs.TableUpdatePermission)
	} else {
		for _, item := range list {
			perm := item.(*PermissionsObject)

			if perm.Admin {
				perms = append(perms, defs.TableAdminPermission)
			}

			if perm.Read {
				perms = append(perms, defs.TableReadPermission)
			}

			if perm.Write {
				perms = append(perms, defs.TableWritePermission)
			}

			if perm.Delete {
				perms = append(perms, defs.TableDeletePermission)
			}

			if perm.Update {
				perms = append(perms, defs.TableUpdatePermission)
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
	response.Permissions = perms

	// Sort the permissions array so the results are always consistent regardless of
	// the map iteration from the data collected.
	sort.Strings(response.Permissions)

	// Convert the result to JSON and write to the response payload and we are done.
	w.Header().Set("Content-Type", defs.JSONMediaType)
	// The status is not sent here: util.WriteJSON below issues it, because it may
	// first need to add a Content-Encoding header, and headers set after
	// WriteHeader() are silently discarded.

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// ReadTablePermissions lists the permissions every user has been granted on a single
// table. Unlike ReadPermissions (which reports one user's grants -- the caller's own,
// or one named via ?user=), this has no user filter: it is the table-scoped analog of
// dsns.ListDSNPermHandler, and requires the same DSN/table admin standing
// authorizedForTablePermissions checks -- much narrower than the RootPermission that
// ReadAllPermissions demands for its DSN-wide dump.
func ReadTablePermissions(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	tableName := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])

	// DATA-SECURITY-2.md finding #4: see the doc comment on
	// authorizedForTablePermissions (above ReadPermissions, earlier in this
	// file) for what this check accepts and why the route no longer gates
	// this by itself.
	if !authorizedForTablePermissions(session, dsnName, tableName) {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.perm.privilege", ui.A{"permission": defs.TableAdminPermission}), http.StatusForbidden)
	}

	if !initPermissions() {
		err := errors.ErrPermissionsUnavailable.Clone().Context(dsnName + "." + tableName)

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	list, err := pHandle.Read(
		pHandle.Equals("dsn", dsnName),
		pHandle.Equals("table", tableName))

	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	response := defs.AllPermissionResponse{
		ServerInfo:  util.MakeServerInfo(session.ID),
		Permissions: []defs.PermissionObject{},
	}

	for _, item := range list {
		perm := item.(*PermissionsObject)

		perms := []string{}

		if perm.Admin {
			perms = append(perms, defs.TableAdminPermission)
		}

		if perm.Read {
			perms = append(perms, defs.TableReadPermission)
		}

		if perm.Write {
			perms = append(perms, defs.TableWritePermission)
		}

		if perm.Update {
			perms = append(perms, defs.TableUpdatePermission)
		}

		if perm.Delete {
			perms = append(perms, defs.TableDeletePermission)
		}

		if len(perms) == 0 {
			continue
		}

		sort.Strings(perms)

		response.Permissions = append(response.Permissions, defs.PermissionObject{
			User:        perm.User,
			DSNName:     perm.DSN,
			Table:       perm.Table,
			Permissions: perms,
		})
	}

	sort.Slice(response.Permissions, func(i, j int) bool {
		return response.Permissions[i].User < response.Permissions[j].User
	})

	response.Count = len(response.Permissions)
	response.Status = http.StatusOK

	ui.Log(ui.TableLogger, "table.permissions", ui.A{
		"session": session.ID,
		"dsn":     parsing.StripQuotes(dsnName),
		"table":   parsing.StripQuotes(tableName),
		"count":   response.Count})

	w.Header().Set("Content-Type", defs.JSONMediaType)

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

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
func ReadAllPermissions(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	dsnName := data.String(session.URLParts["dsn"])
	if dsnName == "@all" {
		dsnName = ""
	}

	if !initPermissions() {
		err := errors.ErrPermissionsUnavailable.Clone()

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	response := defs.AllPermissionResponse{
		Permissions: []defs.PermissionObject{},
	}

	var nameFilter, dsnFilter *resources.Filter

	if f := parsing.RequestForUser("", r.URL); f != "" {
		text, err := parsing.SQLEscape(f)
		if err != nil {
			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.filter.invalid"), http.StatusBadRequest)
		}

		nameFilter = pHandle.Equals("user", text)
	}

	if dsnName != "" {
		text, err := parsing.SQLEscape(dsnName)
		if err != nil {
			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.filter.invalid"), http.StatusBadRequest)
		}

		dsnFilter = pHandle.Equals("dsn", text)
	}

	list, err := pHandle.Read(dsnFilter, nameFilter)

	if err != nil {
		ui.Log(ui.TableLogger, "table.read.error", ui.A{
			"session": session.ID,
			"error":   err.Error()})

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
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
			permissions = append(permissions, defs.TableAdminPermission)
		}

		if p.Read {
			permissions = append(permissions, defs.TableReadPermission)
		}

		if p.Write {
			permissions = append(permissions, defs.TableWritePermission)
		}

		if p.Update {
			permissions = append(permissions, defs.TableUpdatePermission)
		}

		if p.Delete {
			permissions = append(permissions, defs.TableDeletePermission)
		}

		permObject.Permissions = permissions

		sort.Strings(permObject.Permissions)

		permObject.User = p.User
		permObject.DSNName = p.DSN
		permObject.Table = p.Table

		response.Permissions = append(response.Permissions, permObject)
		count = count + 1
	}

	response.Count = count
	response.Status = http.StatusOK

	// The status is not sent here: util.WriteJSON below issues it, because it may
	// first need to add a Content-Encoding header, and headers set after
	// WriteHeader() are silently discarded.

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

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
func GrantPermissions(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if !initPermissions() {
		err := errors.ErrPermissionsUnavailable.Clone()

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	tableName := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])

	// DATA-SECURITY-2.md finding #4: see the doc comment on
	// authorizedForTablePermissions (earlier in this file, just above
	// ReadPermissions) for what this check accepts and why the route no
	// longer gates this by itself.
	if !authorizedForTablePermissions(session, dsnName, tableName) {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.perm.privilege", ui.A{"permission": defs.TableAdminPermission}), http.StatusForbidden)
	}

	user := session.User

	if users := session.Parameters["user"]; len(users) == 1 {
		user = users[0]
	}

	items, err := pHandle.Read(
		pHandle.Equals("user", user),
		pHandle.Equals("table", tableName),
		pHandle.Equals("dsn", dsnName))

	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
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
				return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
			}
		} else {
			err = errors.ErrPermissionsUnavailable.Clone().Context(dsnName + "." + tableName)

			return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusNotFound)
		}
	}

	item := items[0].(*PermissionsObject)

	permissionsList := []string{}

	if err = json.NewDecoder(r.Body).Decode(&permissionsList); err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	sort.Strings(permissionsList)

	if !validPermissions(permissionsList) {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.permissions.invalid", ui.A{"list": permissionsList}), http.StatusBadRequest)
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
		case defs.TableReadPermission:
			item.Read = setting
		case defs.TableUpdatePermission:
			item.Update = setting
		case defs.TableDeletePermission:
			item.Delete = setting
		case defs.TableWritePermission:
			item.Write = setting
		case defs.TableAdminPermission:
			item.Admin = setting
		default:
			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.permission.invalid", ui.A{"name": key}), http.StatusBadRequest)
		}
	}

	err = pHandle.Update(item, pHandle.Equals("id", item.ID))
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	return ReadPermissions(session, w, r)
}

// DeletePermissions deletes one or more permissions records for a given username, dsn, and table.
func DeletePermissions(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	dsnName := data.String(session.URLParts["dsn"])
	if dsnName == "@all" {
		dsnName = ""
	}

	tableName := data.String(session.URLParts["table"])

	// DATA-SECURITY-2.md finding #4: see the doc comment on
	// authorizedForTablePermissions (earlier in this file, just above
	// ReadPermissions) for what this check accepts and why the route no
	// longer gates this by itself.
	//
	// Note dsnName may be "" here (the "@all" pseudo-DSN name normalized a
	// few lines up, meaning "sweep this table name across every DSN").
	// AuthDSN and Authorized() both fail an empty DSN name, so only
	// session.Admin or an identity-wide ego.dsn.admin grant -- never a
	// DSN-specific or table-specific one -- can satisfy this check for that
	// wide a sweep, which is the correct outcome: a caller who only
	// administers one specific DSN or table has no standing to touch grants
	// on tables of the same name in every *other* DSN too.
	if !authorizedForTablePermissions(session, dsnName, tableName) {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.perm.privilege", ui.A{"permission": defs.TableAdminPermission}), http.StatusForbidden)
	}

	if !initPermissions() {
		err := errors.ErrPermissionsUnavailable.Clone()

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	var nameFilter, dsnFilter, tableFilter *resources.Filter

	// Found while testing DATA-SECURITY-2.md finding #4: this used to
	// filter on the column name "name", but PermissionsObject (this file,
	// near the top) has no "name" field -- its field is "User" (stored as
	// SQL column "user_name", looked up here by the Go field name "User",
	// which ResHandle.Equals matches case-insensitively as "user"; see
	// ReadAllPermissions' identical pHandle.Equals("user", text) call
	// above, and Authorized()'s own doc comment further down in this file
	// for a fuller explanation of this exact failure mode elsewhere in the
	// package).
	//
	// Go note for readers new to the language: ResHandle.Equals
	// (internal/resources/filters.go) has a *value* receiver ("func (r
	// ResHandle) Equals(...)"), so each call works on its own private copy
	// of the handle, not the shared pHandle variable -- when the column
	// name doesn't match anything, that copy silently records the mismatch
	// on itself (discarded the moment the call returns) and hands back a
	// plain nil *Filter. No panic, no error returned to this function, and
	// no lasting effect on pHandle for the next call.
	//
	// The practical effect here: pHandle.Read(dsnFilter, tableFilter,
	// nameFilter) below received nameFilter == nil whenever a ?user= query
	// parameter was given, which is the same as passing no user filter at
	// all -- so a request meant to revoke exactly one user's grant on one
	// table instead matched and deleted *every* user's table_perms row for
	// that dsn+table. A DELETE meant to be narrow was silently deleting far
	// more than the caller asked for.
	if f := parsing.RequestForUser("", r.URL); f != "" {
		text, err := parsing.SQLEscape(f)
		if err != nil {
			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.filter.invalid"), http.StatusBadRequest)
		}

		nameFilter = pHandle.Equals("user", text)
	}

	if tableName != "" {
		tableFilter = pHandle.Equals("table", tableName)
	}

	if dsnName != "" {
		text, err := parsing.SQLEscape(dsnName)
		if err != nil {
			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.filter.invalid"), http.StatusBadRequest)
		}

		dsnFilter = pHandle.Equals("dsn", text)
	}

	list, err := pHandle.Read(dsnFilter, tableFilter, nameFilter)

	if err != nil {
		ui.Log(ui.TableLogger, "table.read.error", ui.A{
			"session": session.ID,
			"error":   err.Error()})

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
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

			return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
		}

		count = count + 1
	}

	ui.Log(ui.TableLogger, "table.perms.deleted", ui.A{
		"session": session.ID,
		"dsn":     dsnName,
		"count":   count})

	return http.StatusOK
}

// DeletePermissionsByDSN removes every table_perms entry associated with the given
// DSN name. This is called when a DSN itself is deleted, so that stale per-table
// grants don't linger in the system database (and potentially reapply) if the DSN
// name is ever reused.
func DeletePermissionsByDSN(session int, dsnName string) (int, error) {
	if !initPermissions() {
		return 0, nil
	}

	list, err := pHandle.Read(pHandle.Equals("dsn", dsnName))
	if err != nil {
		return 0, err
	}

	count := 0

	for _, item := range list {
		p := item.(*PermissionsObject)
		if p == nil {
			continue
		}

		if _, err := pHandle.Delete(pHandle.Equals("id", p.ID)); err != nil {
			return count, err
		}

		count++
	}

	ui.Log(ui.TableLogger, "table.perms.deleted", ui.A{
		"session": session,
		"dsn":     dsnName,
		"count":   count})

	return count, nil
}

// Authorized uses the system database to determine if the proposed operation is permitted
// for the given table. This only applies for tables in a DSN that is marked as "restricted".
// By default, DSNs are not restricted and depend on the underlying provider to handle all role
// and permissions checks. If a DSN is restricted, then before the provider is even
// contacted, we verify if the user/dsn/table and operation are authorized.
func Authorized(session *router.Session, user string, table string, operations ...string) bool {
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

	// If this DSN isn't Restricted (i.e. subject to Ego authorization rules)
	// we have nothing to do.
	if !dsnName.Restricted {
		return true
	}

	// If we the permissions subsystem is not initialized, then allow any operation.
	if !initPermissions() {
		return true
	}

	// NOTE: this filtered on the nonexistent column "name" until fixed here.
	// PermissionsObject has no "name" field (see its json tags above), so
	// resources.Equals set its (function-local, discarded) error and
	// returned a nil filter, which generateReadSQL then silently drops --
	// meaning this read matched every user's grant for (dsn, table), not
	// just user's. With exactly one grantee it authorized every caller as
	// that grantee; with zero or more than one it denied everyone,
	// including the correct owner. "user", matching the field name Go
	// resolves the SQL column from (see ReadPermissions/GrantPermissions
	// above, which already filter this same table on "user"), is correct.
	items, err := pHandle.Read(
		pHandle.Equals("dsn", dsn),
		pHandle.Equals("table", table),
		pHandle.Equals("user", user))
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
		case defs.TableReadPermission:
			if !perm.Read && !perm.Admin {
				auth = false
			}

		case defs.TableWritePermission:
			if !perm.Write && !perm.Admin {
				auth = false
			}

		case defs.TableAdminPermission:
			if !perm.Admin {
				auth = false
			}

		case defs.TableDeletePermission:
			if !perm.Delete && !perm.Admin {
				auth = false
			}

		case defs.TableUpdatePermission:
			if !perm.Update && !perm.Admin {
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
func createTablePermissions(session *router.Session, user, dsn, table string) bool {
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
func removeTablePermissions(session *router.Session, table string) bool {
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
