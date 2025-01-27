package users

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// ListUsersHandler is the handler for the GET method on the users endpoint. If a name was
// specified in the URL, this calls the individual user "GET" function, else it returns a
// list of all users.
func ListUsersHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	result := defs.UserCollection{
		BaseCollection: util.MakeBaseCollection(session.ID),
		Items:          []defs.User{},
	}

	userDatabase := auth.AuthService.ListUsers()

	for k, u := range userDatabase {
		ud := defs.User{
			Name:        k,
			ID:          u.ID,
			Permissions: u.Permissions,
		}
		result.Items = append(result.Items, ud)
	}

	// sort result.Items by Name
	sort.Slice(result.Items, func(i, j int) bool {
		return result.Items[i].Name < result.Items[j].Name
	})

	result.Count = len(result.Items)
	result.Start = 0

	w.Header().Add(defs.ContentTypeHeader, defs.UsersMediaType)

	// convert result to json and write to response
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
