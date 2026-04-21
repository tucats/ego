package dsns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// ListDSNPermHandler lists the permissions for a given DSN from a GET operation to the
// /dsns/{{name}}/permissions endpoint.
func ListDSNPermHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK

	// Get the named DSN.
	name := data.String(session.URLParts["dsn"])

	_, err := DSNService.ReadDSN(session.ID, session.User, name, false)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusNotFound)
	}

	perms, err := DSNService.Permissions(session.ID, session.User, name)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
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
		if actions&DSNAdminAction != 0 {
			actionList = append(actionList, defs.AdminPriv)
		}

		if actions&DSNReadAction != 0 {
			actionList = append(actionList, defs.ReadPriv)
		}

		if actions&DSNWriteAction != 0 {
			actionList = append(actionList, defs.WritePriv)
		}

		response.Items[user] = actionList
	}

	w.Header().Add(defs.ContentTypeHeader, defs.DSNListPermsMediaType)

	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return status
}

// ListDSNHandler reads all DSNs from a GET operation to the /dsns/ endpoint.
func ListDSNHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Get the map of all the DSN names.
	names, err := DSNService.ListDSNS(session.ID, session.User)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
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
	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// GetDSNHandler reads a DSN from a GET operation to the /dsns/{{name}} endpoint.
func GetDSNHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK
	name := strings.TrimSpace(data.String(session.URLParts["dsn"]))

	dataSourceName, err := DSNService.ReadDSN(session.ID, session.User, name, false)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
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
	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return status
}

// DeleteDSNHandler deletes a DSN from a DEL operation to the /dsns/{{name}} endpoint.
func DeleteDSNHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK
	name := strings.TrimSpace(data.String(session.URLParts["dsn"]))

	dataSourceName, err := DSNService.ReadDSN(session.ID, session.User, name, false)
	if err != nil {
		status = http.StatusBadRequest
		if errors.Equal(err, errors.ErrNoSuchDSN) {
			status = http.StatusNotFound
		}

		return util.ErrorResponse(w, session.ID, err.Error(), status)
	}

	if err := DSNService.DeleteDSN(session.ID, session.User, name); err != nil {
		msg := fmt.Sprintf("unable to delete DSN, %s", err)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
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

	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return status
}

// CreateDSNHandler creates a DSN from a POST operation to the /dsns endpoint. The
// body must contain the representation of the DSN to be created.
func CreateDSNHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK
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

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Minor cleanup/sanity checks to ensure a validly formed name, provider,
	// port, etc.
	if dataSourceName.Name != strings.TrimSpace(dataSourceName.Name) {
		msg := fmt.Sprintf("invalid dsn name: %s", dataSourceName.Name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	if dataSourceName.Provider != "sqlite3" {
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
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	}

	// Does this DSN already exist?
	if _, err := DSNService.ReadDSN(session.ID, session.User, dataSourceName.Name, true); err == nil {
		msg := fmt.Sprintf("dsn already exists: %s", dataSourceName.Name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	// Create a new DSN from the payload given.
	if err := DSNService.WriteDSN(session.ID, session.User, dataSourceName); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
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

	b := util.WriteJSON(w, response, &session.ResponseLength)

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
func DSNPermissionsHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
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

			util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
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
			_, err = DSNService.ReadDSN(session.ID, item.User, item.DSN, true)
		}

		if err != nil {
			err = errors.New(err).Context(item.DSN + ", " + item.User)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}

		for _, action := range item.Actions {
			// Strip off the '+' or '-' from the action name, if any, that defines if this
			// is a grant or revoke. We just want the action name itself to validate.
			if action[0:1] == "+" || action[0:1] == "-" {
				action = action[1:]
			}

			if !util.InList(strings.ToLower(action), defs.AdminPriv, defs.ReadPriv, defs.WritePriv) {
				err = errors.ErrInvalidPermission.Context(action)

				return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
			}
		}
	}

	// If all the items are valid, let's try to set the relevant actions.
	for _, item := range items.Items {
		for _, actionName := range item.Actions {
			var (
				action DSNAction
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
				action = DSNAdminAction

			case defs.ReadPriv:
				action = DSNReadAction

			case defs.WritePriv:
				action = DSNWriteAction
			}

			if err := DSNService.GrantDSN(session.ID, item.User, item.DSN, action, grant); err != nil {
				return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
			}
		}
	}

	response := defs.DBRowCount{
		ServerInfo: util.MakeServerInfo(session.ID),
		Count:      len(items.Items),
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
