package dsns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	egodsns "github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/dberrors"
	"github.com/tucats/ego/internal/server/tables"
	"github.com/tucats/ego/internal/util"
)

// ListDSNPermHandler lists the permissions for a given DSN from a GET operation to the
// /dsns/{{name}}/permissions endpoint.
func ListDSNPermHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK

	// Get the named DSN.
	name := data.String(session.URLParts["dsn"])

	_, err := egodsns.DSNService.ReadDSN(session.ID, session.User, name, false)
	if err != nil {
		// Was hardcoded to 404. Routed through the shared classifier so this
		// agrees with GetDSNHandler/DeleteDSNHandler on the identical
		// underlying error, rather than being a fourth place that could drift
		// away from them (REST-2 fixed the first three; this was missed).
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), dberrors.PayloadStatus(err))
	}

	perms, err := egodsns.DSNService.Permissions(session.ID, session.User, name)
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	response := defs.DSNPermissionResponse{}
	response.ServerInfo = util.MakeServerInfo(session.ID)
	response.DSN = name
	response.Status = http.StatusOK

	if len(perms) > 0 {
		response.Items = map[string][]string{}
	}

	for user, actions := range perms {
		actionList := []string{}
		if actions&egodsns.DSNAdminAction != 0 {
			actionList = append(actionList, defs.AdminPriv)
		}

		if actions&egodsns.DSNReadAction != 0 {
			actionList = append(actionList, defs.ReadPriv)
		}

		if actions&egodsns.DSNWriteAction != 0 {
			actionList = append(actionList, defs.WritePriv)
		}

		response.Items[user] = actionList
	}

	w.Header().Add(defs.ContentTypeHeader, defs.DSNListPermsMediaType)

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return status
}

// ListDSNHandler reads all DSNs from a GET operation to the /dsns/ endpoint.
//
// The route itself (commands/routes.go) only requires authentication, not
// any specific permission, because the two permissions that unlock this
// endpoint -- ego.server.admin and ego.sql -- are alternatives (either one
// is enough), and Route.Permissions() only expresses "all of these are
// required". So the check is done here instead: an admin caller (or an
// ego.server.admin holder) can list every DSN on the server; an ego.sql
// holder (the dashboard's SQL tab, or a non-admin API caller with @sql
// access -- see internal/server/tables/sql_permissions.go) can too, purely
// so they have DSN names to choose from, not because ego.sql implies
// server-admin standing more generally.
func ListDSNHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	if !session.Admin &&
		!util.InListInsensitive(defs.ServerAdminPermission, session.Permissions...) &&
		!util.InListInsensitive(defs.SQLPermission, session.Permissions...) {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.perm.privilege", ui.A{"permission": defs.ServerAdminPermission}), http.StatusForbidden)
	}

	// Get the map of all the DSN names.
	names, err := egodsns.DSNService.ListDSNS(session.ID, session.User)
	if err != nil {
		// This endpoint takes no client-supplied payload beyond paging, so a
		// failure here is never something a different request body would
		// have avoided -- ExecStatus's 500 default fits, not PayloadStatus's
		// 400.
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), dberrors.ExecStatus(err))
	}

	// Non-admin callers (e.g. an ego.sql holder without server-admin
	// standing) only see DSNs that are unrestricted or that they have
	// access to -- either an identity-wide ego.dsn.read/ego.dsn.admin
	// grant, which (per DATA-SECURITY.md §3.4/§1a) unlocks every DSN, or a
	// DSN-specific dsns_auth record (AuthDSN). This mirrors the same
	// identity-then-per-DSN check tables/database.Open() applies before it
	// lets a non-admin touch a restricted DSN's data.
	if !session.Admin &&
		!util.InListInsensitive(defs.ServerAdminPermission, session.Permissions...) &&
		!egodsns.IdentityAuthorizesAction(session.Permissions, egodsns.DSNReadAction) {
		for key, dsn := range names {
			if dsn.Restricted && !egodsns.DSNService.AuthDSN(session.ID, session.User, dsn.Name, egodsns.DSNReadAction) {
				delete(names, key)
			}
		}
	}

	// Build a sorted list of DSN names for stable output.
	keys := make([]string, 0, len(names))
	for key := range names {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	// Apply paging. session.Start and session.Limit were already validated and
	// populated by the server framework before this handler was called.
	start := session.Start
	limit := session.Limit

	if limit == 0 {
		maxLimit := settings.GetInt(defs.ServerMaxItemLimitSetting)
		if maxLimit > 0 {
			limit = maxLimit
		}
	}

	if start > len(keys) {
		start = len(keys)
	}

	keys = keys[start:]

	if limit > 0 && limit < len(keys) {
		keys = keys[:limit]
	}

	// Build an array of DSNs from the map of DSN data using the paged key list.
	items := make([]defs.DSN, len(keys))

	for idx, key := range keys {
		items[idx] = names[key]
	}

	// Craft a response object to send back.
	response := defs.DSNListResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Items:      items,
		Count:      len(items),
		Start:      start,
		Limit:      limit,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.DSNListMediaType)
	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// GetDSNHandler reads a DSN from a GET operation to the /dsns/{{name}} endpoint.
func GetDSNHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK
	name := strings.TrimSpace(data.String(session.URLParts["dsn"]))

	dataSourceName, err := egodsns.DSNService.ReadDSN(session.ID, session.User, name, false)
	if err != nil {
		// This answered 400 while DeleteDSNHandler below answered 404 for the
		// identical error, so adjacent routes on the same path disagreed about
		// what a missing DSN means (REST-2).
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), dberrors.PayloadStatus(err))
	}

	// Craft a response object to send back.
	response := defs.DSNResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Name:       dataSourceName.Name,
		Provider:   dataSourceName.Provider,
		Host:       dataSourceName.Host,
		Port:       dataSourceName.Port,
		User:       dataSourceName.Username,
		Schema:     dataSourceName.Schema,
		Secured:    dataSourceName.Secured,
		Restricted: dataSourceName.Restricted,
		Password:   defs.ElidedPassword,
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.DSNMediaType)
	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return status
}

// DeleteDSNHandler deletes a DSN from a DEL operation to the /dsns/{{name}} endpoint.
func DeleteDSNHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK
	name := strings.TrimSpace(data.String(session.URLParts["dsn"]))

	dataSourceName, err := egodsns.DSNService.ReadDSN(session.ID, session.User, name, false)
	if err != nil {
		// This route already answered 404 via its own inline check. It now uses
		// the shared classifier so there is one place that decides, and so it
		// cannot drift away from GetDSNHandler again (REST-2).
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), dberrors.PayloadStatus(err))
	}

	if err := egodsns.DSNService.DeleteDSN(session.ID, session.User, name); err != nil {
		// The existence check just above already confirmed the DSN is
		// there, so a failure here is a storage fault at exec time, not a
		// malformed request -- ExecStatus's 500 default fits, not a flat
		// 400.
		msg := fmt.Sprintf("unable to delete DSN, %s", err)

		return util.ErrorResponse(w, session.ID, msg, dberrors.ExecStatus(err))
	}

	// Best-effort cleanup of any per-table permission grants for this DSN, so
	// stale table_perms rows don't linger (and potentially reapply) if the DSN
	// name is ever reused. This does not fail the DSN deletion if it errors.
	if _, err := tables.DeletePermissionsByDSN(session.ID, name); err != nil {
		ui.Log(ui.TableLogger, "table.delete.error", ui.A{
			"session": session.ID,
			"error":   err.Error()})
	}

	// Craft a response object to send back  that contains the DSN info
	// we just deleted.
	response := defs.DSNResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Name:       dataSourceName.Name,
		Provider:   dataSourceName.Provider,
		Host:       dataSourceName.Host,
		Port:       dataSourceName.Port,
		User:       dataSourceName.Username,
		Secured:    dataSourceName.Secured,
		Schema:     dataSourceName.Schema,
		Restricted: dataSourceName.Restricted,
		Password:   defs.ElidedPassword,
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.DSNMediaType)

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return status
}

// CreateDSNHandler creates a DSN from a POST operation to the /dsns endpoint. The
// body must contain the representation of the DSN to be created.
func CreateDSNHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	var status int 
	
	dataSourceName := defs.DSN{}

	// Retrieve content from the request body
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
		"session": session.ID,
		"body":    buf.String()})

	if err := json.Unmarshal(buf.Bytes(), &dataSourceName); err != nil {
		ui.Log(ui.RestLogger, "rest.bad.payload", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
	}

	// Minor cleanup/sanity checks to ensure a validly formed name, provider,
	// port, etc.
	if dataSourceName.Name != strings.TrimSpace(dataSourceName.Name) {
		msg := fmt.Sprintf("invalid dsn name: %s", dataSourceName.Name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	if dataSourceName.Provider == defs.DeprecatedSqliteProvider {
		dataSourceName.Provider = defs.SqliteProvider
	}

	if dataSourceName.Provider != defs.SqliteProvider {
		if dataSourceName.Host == "" {
			dataSourceName.Host = defs.LocalHost
		}

		if dataSourceName.Port < 80 {
			msg := fmt.Sprintf("invalid port number: %d", dataSourceName.Port)

			return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
		}

		if encoded, err := encrypt(dataSourceName.Password); err == nil {
			dataSourceName.Password = encoded
		} else {
			return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
		}
	}

	// Does this DSN already exist? A duplicate name is the textbook 409
	// case -- the request is well-formed, it just conflicts with a DSN
	// already stored under that name -- not a 400.
	if _, err := egodsns.DSNService.ReadDSN(session.ID, session.User, dataSourceName.Name, true); err == nil {
		msg := errors.ErrDSNAlreadyExists.Clone().Context(dataSourceName.Name).Localize(session.Language)

		return util.ErrorResponse(w, session.ID, msg, http.StatusConflict)
	}

	// Create a new DSN from the payload given. The existence check just
	// above means a failure here is a storage fault at exec time; route it
	// through the same classifier as every other exec-stage failure so an
	// unclassified error still defaults to 500 rather than being hardcoded
	// separately from its siblings.
	if err := egodsns.DSNService.WriteDSN(session.ID, session.User, dataSourceName); err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), dberrors.ExecStatus(err))
	}

	// A successful create reports 201, not 200, with a Location header
	// naming the new DSN's own URL -- RFC 9110 §10.2.2. This is a genuine
	// creation (the existence check above already ruled out an overwrite),
	// and GET on this exact path returns the DSN just created
	// (GetDSNHandler), so there is a real resource for the header to name.
	status = http.StatusCreated

	// Craft a response object to send back.
	response := defs.DSNResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Name:       dataSourceName.Name,
		Provider:   dataSourceName.Provider,
		Host:       dataSourceName.Host,
		Port:       dataSourceName.Port,
		User:       dataSourceName.Username,
		Schema:     dataSourceName.Schema,
		Secured:    dataSourceName.Secured,
		Restricted: dataSourceName.Restricted,
		Password:   defs.ElidedPassword,
		Status:     status,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.DSNMediaType)
	w.Header().Set(defs.LocationHeader, defs.DSNPath+url.PathEscape(dataSourceName.Name))

	b := util.WriteJSON(w, session.Response(), status, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return status
}

// DSNPermissionsHandler grants or revokes DSN permissions for a given user from a POST
// operation to the /dsns/{{name}}/permissions endpoint. The body must contain the
// representation of the permissions to be granted or revoked.
func DSNPermissionsHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// Retrieve content from the request body
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
		"session": session.ID,
		"body":    buf.String()})

	items := defs.DSNPermissionsRequest{}

	// Is it a request with a list, or a single item?
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil || len(items.Items) == 0 {
		item := defs.DSNPermissionItem{}
		if err := json.Unmarshal(buf.Bytes(), &item); err != nil {
			ui.Log(ui.RestLogger, "rest.bad.payload", ui.A{
				"session": session.ID,
				"error":   err.Error()})

			// Was missing this return: execution fell through to the
			// success path below with an empty items.Items, writing a
			// second, misleading "200 OK, 0 items" response on top of the
			// 400 already sent for genuinely malformed JSON.
			return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
		} else {
			items.Items = []defs.DSNPermissionItem{item}
		}
	}

	// Validate the items in the list
	for _, item := range items.Items {
		var err error

		if item.DSN == "" {
			err = errors.ErrNoSuchDSN
		} else if item.User == "" {
			err = errors.ErrNoSuchUser
		} else {
			_, err = egodsns.DSNService.ReadDSN(session.ID, item.User, item.DSN, true)
		}

		if err != nil {
			err = errors.New(err).Context(item.DSN + ", " + item.User)

			// A missing DSN (the item.DSN == "" case, or a ReadDSN miss) is
			// 404 via the shared classifier, same as every other DSN-not-
			// found site in this file. errors.ErrNoSuchUser isn't something
			// dberrors classifies -- it's a user, not a DSN or database
			// error -- so that case still falls through to PayloadStatus's
			// 400 default, unchanged from before.
			return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), dberrors.PayloadStatus(err))
		}

		for _, action := range item.Actions {
			// Strip off the '+' or '-' from the action name, if any, that defines if this
			// is a grant or revoke. We just want the action name itself to validate.
			if action[0:1] == "+" || action[0:1] == "-" {
				action = action[1:]
			}

			if !util.InList(strings.ToLower(action), defs.AdminPriv, defs.ReadPriv, defs.WritePriv) {
				err = errors.ErrInvalidPermission.Context(action)

				return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
			}
		}
	}

	// If all the items are valid, let's try to set the relevant actions.
	for _, item := range items.Items {
		for _, actionName := range item.Actions {
			var (
				action egodsns.DSNAction
				grant  = true
			)

			// Strip off the grant or revoke flag (if present) and determine if this is a revoke (not a grant).
			switch actionName[0:1] {
			case "+":
				actionName = actionName[1:]
			case "-":
				actionName = actionName[1:]
				grant = false
			}

			switch strings.ToLower(actionName) {
			case defs.AdminPriv:
				action = egodsns.DSNAdminAction

			case defs.ReadPriv:
				action = egodsns.DSNReadAction

			case defs.WritePriv:
				action = egodsns.DSNWriteAction
			}

			if err := egodsns.DSNService.GrantDSN(session.ID, item.User, item.DSN, action, grant); err != nil {
				return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
			}
		}
	}

	response := defs.DBRowCount{
		ServerInfo: util.MakeServerInfo(session.ID),
		Count:      len(items.Items),
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
