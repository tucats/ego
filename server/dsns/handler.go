package dsns

import (
	"bytes"
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

	resp := defs.DSNPermissionResponse{}
	resp.ServerInfo = util.MakeServerInfo(session.ID)
	resp.DSN = name
	resp.Status = http.StatusOK

	if len(perms) > 0 {
		resp.Items = map[string][]string{}
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

		resp.Items[user] = actionList
	}

	w.Header().Add(defs.ContentTypeHeader, defs.DSNListPermsMediaType)

	b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return status
}

// ListDSNHandler reads all DSNs from a GET operation to the /dsns/endpoint.
func ListDSNHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK

	// Get the map of all the DSN names.
	names, err := DSNService.ListDSNS(session.ID, session.User)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Make a sorted list of the DSN names.
	keys := []string{}
	for key := range names {
		keys = append(keys, key)
	}

	limit := 1000
	start := 0

	if len(session.Parameters["start"]) > 0 {
		start, err = data.Int(session.Parameters["start"][0])
		if err != nil || start < 0 {
			return util.ErrorResponse(w, session.ID, "Invalid start parameter", http.StatusBadRequest)
		}

		if start > len(keys) {
			start = len(keys)
		}
	}

	if len(session.Parameters["limit"]) > 0 {
		limit, err = data.Int(session.Parameters["limit"][0])
		if err != nil {
			return util.ErrorResponse(w, session.ID, "Invalid limit parameter", http.StatusBadRequest)
		}

		if limit > len(keys)-start {
			limit = len(keys)
		}
	}

	sort.Strings(keys)

	// If there was a start or limit parameter, apply them now to the list of key
	// values, so we appropriately limit the result.
	if start > 0 {
		if start < len(keys) {
			keys = keys[start:]
		} else {
			keys = []string{}
		}
	}

	if limit > 0 && limit < len(keys) {
		keys = keys[:limit]
	}

	// Build an array of DSNs from the map of DSN data, using the sorted list of keys
	items := make([]defs.DSN, len(keys))

	for idx, key := range keys {
		items[idx] = names[key]
	}

	// Craft a response object to send back.
	resp := defs.DSNListResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Items:      items,
		Count:      len(items),
	}

	w.Header().Add(defs.ContentTypeHeader, defs.DSNListMediaType)

	b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return status
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
	resp := defs.DSNResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Name:       dataSourceName.Name,
		Provider:   dataSourceName.Provider,
		Host:       dataSourceName.Host,
		Port:       dataSourceName.Port,
		User:       dataSourceName.Username,
		Schema:     dataSourceName.Schema,
		Secured:    dataSourceName.Secured,
		Native:     dataSourceName.Native,
		Restricted: dataSourceName.Restricted,
		Password:   defs.ElidedPassword,
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.DSNMediaType)

	b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

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
	resp := defs.DSNResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Name:       dataSourceName.Name,
		Provider:   dataSourceName.Provider,
		Host:       dataSourceName.Host,
		Port:       dataSourceName.Port,
		User:       dataSourceName.Username,
		Secured:    dataSourceName.Secured,
		Native:     dataSourceName.Native,
		Schema:     dataSourceName.Schema,
		Restricted: dataSourceName.Restricted,
		Password:   defs.ElidedPassword,
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.DSNMediaType)

	b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

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
	resp := defs.DSNResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Name:       dataSourceName.Name,
		Provider:   dataSourceName.Provider,
		Host:       dataSourceName.Host,
		Port:       dataSourceName.Port,
		User:       dataSourceName.Username,
		Schema:     dataSourceName.Schema,
		Secured:    dataSourceName.Secured,
		Native:     dataSourceName.Native,
		Restricted: dataSourceName.Restricted,
		Password:   defs.ElidedPassword,
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.DSNMediaType)

	b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

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

	resp := defs.DBRowCount{
		ServerInfo: util.MakeServerInfo(session.ID),
		Count:      len(items.Items),
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

	b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
