package users

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// CreateUserHandler is the HTTP handler for POST /admin/users. It reads a
// JSON body describing the new user (name, password, optional permissions),
// validates the permissions, creates the user in the auth store, and returns
// the new user record — with the password replaced by a placeholder string.
func CreateUserHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// Parse the JSON request body into a defs.User struct. If the body is
	// missing or malformed, getUserFromBody returns an error and we reply
	// with 400 Bad Request.
	userInfo, err := getUserFromBody(r, session)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Create a symbol table scoped to this request. auth.SetUser is an Ego
	// built-in function that expects to find the session ID in the symbol
	// table under the well-known defs.SessionVariable key.
	s := symbols.NewSymbolTable(r.URL.Path)
	s.SetAlways(defs.SessionVariable, session.ID)

	// Build a data.Map (Ego's generic key-value map type) carrying the fields
	// that SetUser needs: the username and the plain-text password. SetUser
	// will hash the password before storing it.
	args := data.NewMap(data.StringType, data.InterfaceType).
		SetAlways("name", userInfo.Name).
		SetAlways("password", userInfo.Password)

	// Only replace permissions if the payload included a non-empty list.
	// An absent or empty permissions list means "keep whatever defaults
	// the auth layer assigns".
	if len(userInfo.Permissions) > 0 {
		// Validate every permission name before storing anything. Ego's own
		// built-in permissions all start with "ego." — any that do not match
		// the known list are rejected immediately.
		for _, perm := range userInfo.Permissions {
			if strings.HasPrefix(perm, "ego.") {
				// The caller used the full "ego.something" form — check it
				// exists in defs.AllPermissions.
				if !util.InListInsensitive(perm, defs.AllPermissions...) {
					msg := errors.ErrInvalidPermission.Clone().Context(perm).Error()

					return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
				}
			} else {
				// The caller omitted the "ego." prefix. Check whether adding
				// it would match a known permission, and if so, tell the caller
				// which spelling to use (ambiguous permission error).
				testPerm := "ego." + strings.ToLower(perm)
				if util.InListInsensitive(testPerm, defs.AllPermissions...) {
					msg := errors.ErrAmbiguousPermission.Clone().Context(perm).Chain(errors.ErrDidYouMean.Clone().Context(testPerm)).Error()

					return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
				}
			}
		}

		// data.Map values must be []any, not []string, so convert the
		// permissions slice before adding it to the args map.
		perms := []any{}

		for _, p := range userInfo.Permissions {
			perms = append(perms, p)
		}

		args.SetAlways("permissions", perms)
	}

	// Call auth.SetUser to create (or replace) the user record. On success,
	// immediately read the stored record back so we can return it to the
	// caller with accurate field values.
	if _, err := auth.SetUser(s, data.NewList(args)); err == nil {
		if u, err := auth.AuthService.ReadUser(session.ID, userInfo.Name, false); err == nil {
			w.Header().Add(defs.ContentTypeHeader, defs.UserMediaType)
			w.WriteHeader(http.StatusOK)

			// Never return the password hash to the client — replace it with
			// the elided placeholder string defined in defs.
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
		} else {
			// SetUser succeeded but the subsequent ReadUser failed — unexpected
			// server-side error.
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
		}
	} else {
		// SetUser itself failed — report the auth layer's error message.
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}
}
