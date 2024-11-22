package users

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// GetUserHandler is the handler for the GET method on the users endpoint with a username
// provided in the path.
func GetUserHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	name := data.String(session.URLParts["name"])
	if u, err := auth.AuthService.ReadUser(name, false); err != nil {
		return util.ErrorResponse(w, session.ID, "No such user: "+name, http.StatusNotFound)
	} else {
		w.Header().Add(defs.ContentTypeHeader, defs.UserMediaType)

		u.Password = ""
		b, _ := json.Marshal(u)
		_, _ = w.Write(b)
		session.ResponseLength += len(b)

		return http.StatusOK
	}
}

// getUserFromBody is a helper function that retrieves a User object from
// the request body payload.
func getUserFromBody(r *http.Request, session *server.Session) (*defs.User, error) {
	userInfo := defs.User{Permissions: []string{}}

	// Get the payload which must be a user spec in JSON
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(r.Body); err == nil {
		if err = json.Unmarshal(buf.Bytes(), &userInfo); err != nil {
			ui.Log(ui.RestLogger, "[%d] Bad payload: %v", session.ID, err)

			return nil, err
		}
	}

	return &userInfo, nil
}
