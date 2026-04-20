package users

import (
	"encoding/json"
	"net/http"
	"sort"

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

	for k, u := range userDatabase {
		ud := defs.User{
			Name:        k,
			ID:          u.ID,
			Permissions: u.Permissions,
			Passkeys:    json.RawMessage(passkeyCount(u)),
		}
		result.Items = append(result.Items, ud)
	}

	// Maps in Go have no guaranteed iteration order, so we sort the Items
	// slice alphabetically by Name to give callers a stable, predictable list.
	// sort.Slice sorts in-place using a caller-supplied comparison function.
	sort.Slice(result.Items, func(i, j int) bool {
		return result.Items[i].Name < result.Items[j].Name
	})

	// Record the total number of users and the starting index (always 0 here
	// because this endpoint returns all users in one response).
	result.Count = len(result.Items)
	result.Start = 0

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
