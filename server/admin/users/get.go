package users

import (
	"encoding/json"
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// GetUserHandler is the HTTP handler for GET /admin/users/{name}. It looks up
// a single user by the name extracted from the URL path and returns their
// record — with the password replaced by a placeholder string.
func GetUserHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// session.URLParts["name"] holds the username captured from the URL path
	// by the router (e.g. "/admin/users/alice" → "alice").
	name := data.String(session.URLParts["name"])

	// ReadUser returns the user record and an error. In Go, the short variable
	// declaration "if u, err := …; err != nil" declares u and err, evaluates
	// the right-hand side, and immediately tests err — all in one statement.
	if u, err := auth.AuthService.ReadUser(session.ID, name, false); err != nil {
		// The user was not found — return 404 Not Found.
		return util.ErrorResponse(w, session.ID, i18n.T("error.user.name.not.found", ui.A{"name": name}), http.StatusNotFound)
	} else {
		w.Header().Add(defs.ContentTypeHeader, defs.UserMediaType)

		// Replace the stored password hash with the elided placeholder so it
		// is never transmitted to the caller.
		u.Password = defs.ElidedPassword

		response := defs.UserResponse{
			ServerInfo: util.MakeServerInfo(session.ID),
			User:       u,
			Status:     http.StatusOK,
		}

		b := util.WriteJSON(w, response, &session.ResponseLength)

		if ui.IsActive(ui.RestLogger) {
			ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
				"session": session.ID,
				"body":    string(b)})
		}

		return http.StatusOK
	}
}

// getUserFromBody is a helper used by CreateUserHandler and UpdateUserHandler
// to read and decode a JSON user object from the HTTP request body.
//
// It returns a pointer to a defs.User and nil on success, or nil and a
// non-nil error if the body could not be read or was not valid JSON.
func getUserFromBody(r *http.Request, session *server.Session) (*defs.User, error) {
	// Initialise with an empty (non-nil) Permissions slice so that callers can
	// safely call len() without a nil-pointer check.
	userInfo := defs.User{Permissions: []string{}}

	// Decode the JSON request body directly into userInfo without an
	// intermediate buffer. Only fields present in the JSON are updated;
	// the rest keep their zero values (empty string, nil slice, etc.).
	if err := json.NewDecoder(r.Body).Decode(&userInfo); err != nil {
		ui.Log(ui.RestLogger, "rest.bad.payload", ui.A{
			"session": session.ID,
			"error":   err})

		return nil, err
	}

	// If REST logging is enabled, write a pretty-printed copy of the decoded
	// struct so the log shows exactly what the server received.
	if ui.IsActive(ui.RestLogger) {
		b, _ := json.MarshalIndent(userInfo, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.WriteLog(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return &userInfo, nil
}
