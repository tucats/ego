package users

import (
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// CreateUserHandler is the handler for the POST method on the users endpoint. It creates a
// new user using the JSON payload in the request.
func CreateUserHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	userInfo, err := getUserFromBody(r, session)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Create a symbol table for the use fo the SetUser function.
	s := symbols.NewSymbolTable(r.URL.Path)
	s.SetAlways(defs.SessionVariable, session.ID)

	// Construct an Ego map with two values for the "user" and "password" data from the
	// original payload.
	args := data.NewMap(data.StringType, data.InterfaceType).
		SetAlways("name", userInfo.Name).
		SetAlways("password", userInfo.Password)

	// Only replace permissions if the payload permissions list is non-empty
	if len(userInfo.Permissions) > 0 {
		// Have to convert this from string array to interface array.
		perms := []any{}

		for _, p := range userInfo.Permissions {
			perms = append(perms, p)
		}

		args.SetAlways("permissions", perms)
	}

	// Call the SetUser function, passing in the structure that contains the User information.
	if _, err := auth.SetUser(s, data.NewList(args)); err == nil {
		if u, err := auth.AuthService.ReadUser(session.ID, userInfo.Name, false); err == nil {
			w.Header().Add(defs.ContentTypeHeader, defs.UserMediaType)
			w.WriteHeader(http.StatusOK)

			r := defs.UserResponse{
				ServerInfo: util.MakeServerInfo(session.ID),
				User:       u,
				Status:     http.StatusOK,
			}

			b, _ := json.MarshalIndent(r, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
			_, _ = w.Write(b)
			session.ResponseLength += len(b)

			if ui.IsActive(ui.RestLogger) {
				ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
					"session": session.ID,
					"body":    string(b)})
			}

			return http.StatusOK
		} else {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
		}
	} else {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}
}
