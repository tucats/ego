package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// GetConfigHandler is the HTTP handler for POST /admin/config. The caller
// supplies a JSON array of configuration key names in the request body; the
// handler reads each named setting and returns a map of name → value.
//
// Any setting whose name is a well-known token key is replaced with the elided
// placeholder string so that bearer tokens are never transmitted in responses.
func GetConfigHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// items will hold the list of setting names decoded from the request body.
	items := []string{}

	// bytes.Buffer is an in-memory byte buffer. We read the entire request body
	// into it so we can pass the raw bytes to json.Unmarshal.
	buf := new(bytes.Buffer)

	if _, err := buf.ReadFrom(r.Body); err != nil {
		ui.Log(ui.RestLogger, "rest.bad.payload", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// json.Unmarshal decodes the JSON bytes into the items slice.
	// A JSON array of strings (["key1","key2"]) maps directly to []string.
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		ui.Log(ui.RestLogger, "rest.bad.payload", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	if ui.IsActive(ui.RestLogger) {
		b, _ := json.MarshalIndent(items, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.WriteLog(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	// Build a result map: for each requested key, look up its value in the
	// settings store. Token-related keys are replaced with the elided placeholder
	// so they cannot be extracted via this endpoint.
	config := map[string]string{}

	for _, item := range items {
		var value string

		if util.InList(item, "ego.server.token", "ego.server.token.key", "ego.logon.token") {
			value = defs.ElidedPassword
		} else {
			value = settings.Get(item)
		}

		config[item] = value
	}

	// Wrap the map in the standard response envelope and write it as JSON.
	response := defs.ConfigResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Count:      len(config),
		Items:      config,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.ConfigMediaType)

	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// GetAllConfigHandler is the HTTP handler for GET /admin/config. It returns
// every configuration setting known to the server in a single map.
//
// Sensitive settings (token keys, any setting whose name contains "password"
// or "credentials") are replaced with the elided placeholder so secrets are
// never transmitted in the response.
func GetAllConfigHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// settings.Keys() returns every key that has been set in the in-memory
	// configuration store (persisted profile + command-line overrides).
	items := settings.Keys()

	config := map[string]string{}

	for _, item := range items {
		var value string

		if util.InList(item, "ego.server.token", "ego.server.token.key", "ego.logon.token") {
			// Hard-coded token keys are always elided.
			value = defs.ElidedPassword
		} else if strings.Contains(item, "password") || strings.Contains(item, "credentials") {
			// Any setting whose name suggests it contains a secret is also elided.
			value = defs.ElidedPassword
		} else {
			value = settings.Get(item)
		}

		config[item] = value
	}

	response := defs.ConfigResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Count:      len(config),
		Items:      config,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.ConfigMediaType)
	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
