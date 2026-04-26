package users

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// ListUsersHandler is the HTTP handler for GET /admin/users. It returns a
// sorted JSON collection of all user records, with passwords omitted.
func ListUsersHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Build the response envelope. util.MakeBaseCollection fills in the
	// standard ServerInfo header and an HTTP 200 status. Items starts as an
	// empty slice (not nil) so the JSON output is "[]" rather than "null"
	// when no users exist.
	result := defs.UserCollection{
		BaseCollection: util.MakeBaseCollection(session.ID),
		Items:          []defs.User{},
	}

	// ListUsers returns a map[string]defs.User — the map key is the username,
	// the value is the full user record. We iterate over it and build the
	// Items slice, deliberately omitting the Password field (it stays as its
	// zero value, an empty string) so password hashes are never returned.
	userDatabase := auth.AuthService.ListUsers(true)

	allItems := make([]defs.User, 0, len(userDatabase))

	for k, u := range userDatabase {
		ud := defs.User{
			Name:        k,
			ID:          u.ID,
			Permissions: u.Permissions,
			Passkeys:    json.RawMessage(passkeyCount(u)),
			LastTokenAt: u.LastTokenAt,
		}
		allItems = append(allItems, ud)
	}

	// Maps in Go have no guaranteed iteration order, so we sort the Items
	// slice alphabetically by Name to give callers a stable, predictable list.
	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].Name < allItems[j].Name
	})

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

	if start > len(allItems) {
		start = len(allItems)
	}

	pagedItems := allItems[start:]

	if limit > 0 && limit < len(pagedItems) {
		pagedItems = pagedItems[:limit]
	}

	result.Items = pagedItems
	result.Count = len(pagedItems)
	result.Start = start
	result.Limit = limit

	w.Header().Add(defs.ContentTypeHeader, defs.UsersMediaType)

	// json.MarshalIndent produces human-readable JSON with consistent
	// indentation. egostrings.JSONMinify then removes the extra whitespace
	// before writing to the wire, saving bandwidth while keeping the log
	// version readable.
	b, _ := json.MarshalIndent(result, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	minifiedBytes := []byte(egostrings.JSONMinify(string(b)))
	_, _ = w.Write(minifiedBytes)
	session.ResponseLength += len(minifiedBytes)

	if ui.IsActive(ui.RestLogger) {
		// Log the pretty-printed (non-minified) version for readability.
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
