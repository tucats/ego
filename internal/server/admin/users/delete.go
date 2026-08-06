package users

import (
	"fmt"
	"net/http"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/auth"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/util"
)

// DeleteUserHandler is the HTTP handler for DELETE /admin/users/{name}. It
// looks up the named user, removes them from the auth store, and returns the
// deleted user record so the caller can confirm what was removed.
func DeleteUserHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// session.URLParts is a map populated by the router when it matched the
	// request URL. The "name" key holds the username extracted from the path,
	// e.g. "/admin/users/alice" → name == "alice".
	// data.String() safely converts any value to a string (handles nil, etc.).
	name := data.String(session.URLParts["name"])

	// Verify the user exists before attempting deletion. ReadUser returns an
	// error if the name is not found.
	u, userErr := auth.AuthService.ReadUser(session.ID, name, false)
	if userErr != nil {
		msg := fmt.Sprintf("No username entry for '%s'", name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusNotFound)
	}

	// Blank out the password hash so it is never included in log output or
	// returned to the caller in the response body.
	u.Password = defs.ElidedPassword

	// Create a symbol table for use by the auth.DeleteUser function.
	// auth.DeleteUser is an Ego built-in that reads the session ID from the
	// symbol table, so we must set it before calling.
	s := symbols.NewSymbolTable("delete user")
	s.SetAlways(defs.SessionVariable, session.ID)

	// Attempt the deletion. auth.DeleteUser returns (bool, error): true means
	// the user was found and deleted, false with a nil error means it was not
	// found (a race against another deletion of the same user between the
	// ReadUser check above and this call), and a non-nil error means the
	// underlying delete (or the flush that follows it) failed -- a server
	// fault, not a "not found" condition, since ReadUser already confirmed
	// the user existed a moment ago.
	// data.BoolOrFalse() converts the returned any value to a plain bool.
	v, err := auth.DeleteUser(s, data.NewList(u.Name))
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	if !data.BoolOrFalse(v) {
		msg := fmt.Sprintf("No username entry for '%s'", u.Name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusNotFound)
	}

	// Deletion succeeded — write the deleted user's record back to the caller
	// as confirmation. The password field is already blanked out above.
	w.Header().Add(defs.ContentTypeHeader, defs.UserMediaType)

	response := defs.UserResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		User:       u,
	}
	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}
