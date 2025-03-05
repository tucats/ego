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

// GetConfigHandler is the server endpoint handler for retrieving config values from the server.
func GetConfigHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// The body should be a list of config name strings.
	items := []string{}

	buf := new(bytes.Buffer)

	if _, err := buf.ReadFrom(r.Body); err != nil {
		ui.Log(ui.RestLogger, "rest.bad.payload", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

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

	// Scan over the config items and put their values in the result map
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

	// Prepare the response
	result := defs.ConfigResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Count:      len(config),
		Items:      config,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.ConfigMediaType)

	b, _ := json.MarshalIndent(result, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// GetAllConfigHandler is the server endpoint handler for retrieving all
// config values from the server.
func GetAllConfigHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	items := settings.Keys()

	// Scan over the config items and put their values in the result map
	config := map[string]string{}

	for _, item := range items {
		var value string

		if util.InList(item, "ego.server.token", "ego.server.token.key", "ego.logon.token") {
			value = defs.ElidedPassword
		} else if strings.Contains(item, "password") || strings.Contains(item, "credentials") {
			value = defs.ElidedPassword
		} else {
			value = settings.Get(item)
		}

		config[item] = value
	}

	// Prepare the response
	result := defs.ConfigResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Status:     http.StatusOK,
		Count:      len(config),
		Items:      config,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.ConfigMediaType)

	b, _ := json.MarshalIndent(result, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
