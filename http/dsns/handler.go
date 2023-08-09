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
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/util"
)

// ListDSNPermHandler lists the permissions for a given DSN

func ListDSNPermHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK

	// Get the named DSN.
	name := data.String(session.URLParts["dsn"])

	_, err := DSNService.ReadDSN(session.User, name, false)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusNotFound)
	}

	perms, err := DSNService.Permissions(session.User, name)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	resp := defs.DSNPermissionResponse{}
	resp.ServerInfo = util.MakeServerInfo(session.ID)
	resp.DSN = name

	if len(perms) > 0 {
		resp.Items = map[string][]string{}
	}

	for user, actions := range perms {
		actionList := []string{}
		if actions&DSNAdminAction != 0 {
			actionList = append(actionList, "admin")
		}

		if actions&DSNReadAction != 0 {
			actionList = append(actionList, "read")
		}

		if actions&DSNWriteAction != 0 {
			actionList = append(actionList, "write")
		}

		resp.Items[user] = actionList
	}

	b, _ := json.Marshal(resp)
	_, _ = w.Write(b)

	return status
}

// ListDSNHandler reads all DSNs from a GET operation to the /dsns/endpoint.
func ListDSNHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK

	// Get the map of all the DSN names.
	names, err := DSNService.ListDSNS(session.User)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Make a sorted list of the DSN names.
	keys := []string{}
	for key := range names {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	// Build an array of DSNs from the map of DSN data, using the sorted list of keys
	items := make([]defs.DSN, len(keys))

	for idx, key := range keys {
		items[idx] = names[key]
	}

	// Craft a response object to send back.
	resp := defs.DSNListResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Items:      items,
		Count:      len(items),
	}

	b, _ := json.Marshal(resp)
	_, _ = w.Write(b)

	return status
}

// GetDSNHandler reads a DSN from a GET operation to the /dsns/{{name}} endpoint.
func GetDSNHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK
	name := strings.TrimSpace(data.String(session.URLParts["dsn"]))

	dsname, err := DSNService.ReadDSN(session.User, name, false)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Craft a response object to send back.
	resp := defs.DSNResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Name:       dsname.Name,
		Provider:   dsname.Provider,
		Host:       dsname.Host,
		Port:       dsname.Port,
		User:       dsname.Username,
		Schema:     dsname.Schema,
		Secured:    dsname.Secured,
		Native:     dsname.Native,
		Restricted: dsname.Restricted,
		Password:   "*******",
	}

	b, _ := json.Marshal(resp)
	_, _ = w.Write(b)

	return status
}

// DeleteDSNHandler deletes a DSN from a DEL operation to the /dsns/{{name}} endpoint.
func DeleteDSNHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK
	name := strings.TrimSpace(data.String(session.URLParts["dsn"]))

	dsname, err := DSNService.ReadDSN(session.User, name, false)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	if err := DSNService.DeleteDSN(session.User, name); err != nil {
		msg := fmt.Sprintf("unable to delete DSN, %s", err)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	// Craft a response object to send back  that contains the DSN info
	// we just deleted.
	resp := defs.DSNResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Name:       dsname.Name,
		Provider:   dsname.Provider,
		Host:       dsname.Host,
		Port:       dsname.Port,
		User:       dsname.Username,
		Secured:    dsname.Secured,
		Native:     dsname.Native,
		Schema:     dsname.Schema,
		Restricted: dsname.Restricted,
		Password:   "*******",
	}

	b, _ := json.Marshal(resp)
	_, _ = w.Write(b)

	return status
}

// CreateDSNHandler creates a DSN from a POST operation to the /dsns endpoint. The
// body must contain the representation of the DSN to be created.
func CreateDSNHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK
	dsname := defs.DSN{}

	// Retrieve content from the request body
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	ui.Log(ui.RestLogger, "[%d] Request payload:%s", session.ID, util.SessionLog(session.ID, buf.String()))

	if err := json.Unmarshal(buf.Bytes(), &dsname); err != nil {
		ui.Log(ui.TableLogger, "[%d] Unable to process inbound DSN payload, %s",
			session.ID,
			err)

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Minor cleanup/sanity checks to ensure a validly formed name, provider,
	// port, etc.
	if dsname.Name != strings.ToLower(strings.TrimSpace(dsname.Name)) {
		msg := fmt.Sprintf("invalid dsn name: %s", dsname.Name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	if !util.InList(strings.ToLower(dsname.Provider), "sqlite3", "postgres") {
		msg := fmt.Sprintf("unsupported or invalid provider name: %s", dsname.Provider)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	if dsname.Provider != "sqlite3" {
		if dsname.Host == "" {
			dsname.Host = "localhost"
		}

		if dsname.Port < 80 {
			msg := fmt.Sprintf("invalid port number: %d", dsname.Port)

			return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
		}

		if encoded, err := encrypt(dsname.Password); err == nil {
			dsname.Password = encoded
		} else {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	}

	// Does this DSN already exist?
	if _, err := DSNService.ReadDSN(session.User, dsname.Name, true); err == nil {
		msg := fmt.Sprintf("dsn already exists: %s", dsname.Name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	// Create a new DSN from the payload given.
	if err := DSNService.WriteDSN(session.User, dsname); err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// Craft a response object to send back.
	resp := defs.DSNResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Name:       dsname.Name,
		Provider:   dsname.Provider,
		Host:       dsname.Host,
		Port:       dsname.Port,
		User:       dsname.Username,
		Schema:     dsname.Schema,
		Secured:    dsname.Secured,
		Native:     dsname.Native,
		Restricted: dsname.Restricted,
		Password:   "*******",
	}

	b, _ := json.Marshal(resp)
	_, _ = w.Write(b)

	return status
}

// DSNPermissionsHandler grants or revokes DSN permissions for a given user.
func DSNPermissionsHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Retrieve content from the request body
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	if ui.IsActive(ui.RestLogger) {
		ui.Log(ui.RestLogger, "REST Request:\n%s", util.SessionLog(session.ID, buf.String()))
	}

	items := defs.DSNPermissionsRequest{}

	// Is it a request with a list, or a single item?
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil || len(items.Items) == 0 {
		item := defs.DSNPermissionItem{}
		if err := json.Unmarshal(buf.Bytes(), &item); err != nil {
			util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		} else {
			items.Items = []defs.DSNPermissionItem{item}
			ui.Log(ui.RestLogger, "[%d] Upgraded single permissions item to permissions list", session.ID)
		}
	}

	ui.Log(ui.RestLogger, "[%d] There are %d permission items", session.ID, len(items.Items))

	// Validate the items in the list
	for _, item := range items.Items {
		var err error

		if item.DSN == "" {
			err = errors.ErrNoSuchDSN
		} else if item.User == "" {
			err = errors.ErrNoSuchUser
		} else {
			_, err = DSNService.ReadDSN(item.User, item.DSN, true)
		}

		if err != nil {
			err = errors.NewError(err).Context(item.DSN + ", " + item.User)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}

		for _, action := range item.Actions {
			if action[0:1] == "+" {
				action = action[1:]
			} else if action[0:1] == "-" {
				action = action[1:]
			}

			if !util.InList(strings.ToLower(action), "admin", "read", "write") {
				err = errors.ErrInvalidPermission.Context(action)

				return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
			}
		}
	}

	// If all the items are vaid, let's try to set the relevant actions.
	for _, item := range items.Items {
		for _, actionName := range item.Actions {
			var (
				action DSNAction
				grant  = true
			)

			if actionName[0:1] == "+" {
				actionName = actionName[1:]
			} else if actionName[0:1] == "-" {
				actionName = actionName[1:]
				grant = false
			}

			switch strings.ToLower(actionName) {
			case "admin":
				action = DSNAdminAction

			case "read":
				action = DSNReadAction

			case "write":
				action = DSNWriteAction
			}

			if err := DSNService.GrantDSN(item.User, item.DSN, action, grant); err != nil {
				return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
			}
		}
	}

	resp := defs.DBRowCount{
		ServerInfo: util.MakeServerInfo(session.ID),
		Count:      len(items.Items),
	}

	b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)

	return http.StatusOK
}
