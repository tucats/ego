package users

import (
	"encoding/json"
	"net/http"

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

	// Create a symbol table for the use fo the SetUser function. Also, set the flag
	// that says we are in admin mode, so the function won't complain.
	s := symbols.NewSymbolTable(r.URL.Path).SetAlways(defs.SuperUserVariable, true)

	// Construct an Ego map with two values for the "user" and "password" data from the
	// original payload.
	args := data.NewMap(data.StringType, data.InterfaceType).
		SetAlways("name", userInfo.Name).
		SetAlways("password", userInfo.Password)

	// Only replace permissions if the payload permissions list is non-empty
	if len(userInfo.Permissions) > 0 {
		// Have to convert this from string array to interface array.
		perms := []interface{}{}

		for _, p := range userInfo.Permissions {
			perms = append(perms, p)
		}

		args.SetAlways("permissions", perms)
	}

	// Call the SetUser function, passing in the structure that contains the User information.
	if _, err := auth.SetUser(s, data.NewList(args)); err == nil {
		if u, err := auth.AuthService.ReadUser(userInfo.Name, false); err == nil {
			w.Header().Add(defs.ContentTypeHeader, defs.UserMediaType)
			w.WriteHeader(http.StatusOK)

			r := defs.UserResponse{
				ServerInfo: util.MakeServerInfo(session.ID),
				User:       u,
				Status:     http.StatusOK,
			}

			msg, _ := json.Marshal(r)
			_, _ = w.Write(msg)
			session.ResponseLength += len(msg)

			return http.StatusOK
		} else {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
		}
	} else {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}
}
