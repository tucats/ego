package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	cliconfig "github.com/tucats/ego/internal/cli/config"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/util"
)

// GetConfigHandler is the HTTP handler for POST /admin/config. The caller
// supplies a JSON array of configuration key names in the request body; the
// handler reads each named setting and returns a map of name → value.
//
// Any setting whose name is a well-known token key is replaced with the elided
// placeholder string so that bearer tokens are never transmitted in responses.
func GetConfigHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// items will hold the list of setting names decoded from the request body.
	items := []string{}

	// Decode the JSON request body directly into items without an intermediate
	// buffer. A JSON array of strings (["key1","key2"]) maps directly to []string.
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		ui.Log(ui.RestLogger, "rest.bad.payload", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
	}

	if ui.IsActive(ui.RestLogger) {
		b, _ := json.MarshalIndent(items, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.WriteLog(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	// Build a result map: for each requested key, look up its value in the
	// settings store. Token-related keys are replaced with the elided placeholder
	// so they cannot be extracted via this endpoint. Each item also carries its
	// localized description (in the caller's own language) so the dashboard can
	// show it as a tooltip without a second round-trip.
	config := map[string]defs.ConfigItem{}

	for _, item := range items {
		var value string

		if util.InList(item, "ego.server.token", "ego.server.token.key", "ego.logon.token") {
			value = defs.ElidedPassword
		} else {
			value = settings.Get(item)
		}

		config[item] = defs.ConfigItem{
			Value:       value,
			Readonly:    defs.ReadonlySetting[item],
			Description: cliconfig.Description(session.Language, item),
		}
	}

	// Wrap the map in the standard response envelope and write it as JSON.
	response := defs.ConfigResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Count:      len(config),
		Items:      config,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.ConfigMediaType)

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

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
func GetAllConfigHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// settings.Keys() returns every key that has been set in the in-memory
	// configuration store (persisted profile + command-line overrides).
	items := settings.Keys()

	config := map[string]defs.ConfigItem{}

	for _, item := range items {
		var value string

		if util.InList(item, "ego.server.token", "ego.server.token.key", "ego.logon.token", "ego.logon.refresh.token") {
			// Hard-coded token keys are always elided.
			value = defs.ElidedPassword
		} else if strings.Contains(item, "password") || strings.Contains(item, "credentials") {
			// Any setting whose name suggests it contains a secret is also elided.
			value = defs.ElidedPassword
		} else {
			value = settings.Get(item)
		}

		config[item] = defs.ConfigItem{
			Value:       value,
			Readonly:    defs.ReadonlySetting[item],
			Description: cliconfig.Description(session.Language, item),
		}
	}

	response := defs.ConfigResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Count:      len(config),
		Items:      config,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.ConfigMediaType)
	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// PatchConfigHandler is the HTTP handler for POST /admin/config. The caller
// supplies a JSON object with configuration key names and their string value
// to be applied to the current server instance. Some keys require special
// handling other that being placed in the ephemeral settings table.
// Not all key values can be modified.
func PatchConfigHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// items will hold the list of setting names decoded from the request body.
	items := map[string]any{}

	// Decode the JSON request body directly into items without an intermediate
	// buffer. The JSON object maps directly to the items map.
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		ui.Log(ui.RestLogger, "rest.bad.payload", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
	}

	if ui.IsActive(ui.RestLogger) {
		b, _ := json.MarshalIndent(items, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.WriteLog(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	// First thing that must be done; determine if the request contains items that cannot be set.
	// If so, we don't do the set operation at all.
	invalid := []string{}
	status := http.StatusOK

	for key := range items {
		key = strings.ToLower(key)
		if defs.ReadonlySetting[key] {
			invalid = append(invalid, key)
			status = http.StatusBadRequest
		}
	}

	// If we got a readonly setting then bail out now and make no changes.
	if status != http.StatusOK {
		err := errors.ErrReadOnly.Clone().Context(strings.Join(invalid, ", "))

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), status)
	}

	// Settings okay, spin through them and make the changes.
	for key, item := range items {
		key = strings.ToLower(key)
		text := data.String(item)

		settings.Set(key, text)

		// Special handling for some key values, which also require storage
		// into package variables which act as a faster access to the items.
		// We will count on the payload validation phase detecting bad types
		// so we don't worry about checking conversion return codes here.
		switch key {
		case defs.LogRetainCountSetting:
			ui.LogRetainCount, _ = data.Int(text)

		case defs.LogArchiveSetting:
			ui.SetArchive(text)
		}
	}

	return http.StatusOK
}
