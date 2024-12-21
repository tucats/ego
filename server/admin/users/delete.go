package users

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// DeleteUserHandler is the handler for the DELETE method on the users endpoint.
func DeleteUserHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	name := data.String(session.URLParts["name"])

	u, userErr := auth.AuthService.ReadUser(name, false)
	if userErr != nil {
		msg := fmt.Sprintf("No username entry for '%s'", name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusNotFound)
	}

	// Empty out the hashed password, we don't need it.
	u.Password = ""

	// Create a symbol table for use by the DeleteUser function, with the flag that
	// sais this is being done under the auspices of an administrator.
	s := symbols.NewSymbolTable("delete user").SetAlways(defs.SuperUserVariable, true)

	// Delete the user from the data store. If there was an error, report it.
	v, err := auth.DeleteUser(s, data.NewList(u.Name))
	if err != nil || !data.BoolOrFalse(v) {
		msg := fmt.Sprintf("No username entry for '%s'", u.Name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusNotFound)
	}

	// Write the deleted user record back to the caller.
	w.Header().Add(defs.ContentTypeHeader, defs.UserMediaType)

	// Make a reply that contains the user info and the server info
	// for the just-deleted user.
	reply := defs.UserResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		User:       u,
	}

	b, _ := json.Marshal(reply)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	return http.StatusOK
}
