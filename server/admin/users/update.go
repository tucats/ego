package users

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// UpdateUserHandler is the HTTP handler for PATCH /admin/users/{name}. It
// applies partial updates — password and/or permissions — to an existing user.
// The username itself cannot be changed via this endpoint.
//
// Permissions in the request body may be prefixed with "+" (add) or "-"
// (remove). An unprefixed permission is treated as an add. Permissions not
// present in the request body are left unchanged.
func UpdateUserHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Extract the target username from the URL path captured by the router.
	name := data.String(session.URLParts["name"])

	// Look up the existing user record. If it does not exist, return 404.
	// The "if u, err := …; err != nil { … } else { … }" pattern is idiomatic
	// Go: u and err are scoped to this if/else block.
	if u, err := auth.AuthService.ReadUser(session.ID, name, false); err != nil {
		return util.ErrorResponse(w, session.ID, "no such user: "+name, http.StatusNotFound)
	} else {
		// Decode the JSON request body into a new defs.User struct that carries
		// the fields the caller wants to change.
		newUser, err := getUserFromBody(r, session)
		if err != nil {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}

		// Track whether any actual change was made so we can skip the write
		// operation when the request body is a no-op.
		changed := false

		// Renaming users is not supported — reject the request if the body
		// specifies a different name than the URL.
		if newUser.Name != u.Name {
			return util.ErrorResponse(w, session.ID, "cannot change user name", http.StatusBadRequest)
		}

		// If a new password was provided, hash it and replace the stored hash.
		// An empty password string means "leave the password unchanged".
		if newUser.Password != "" {
			u.Password = egostrings.HashString(newUser.Password)
			changed = true
		}

		if len(newUser.Permissions) > 0 {
			// Validate every permission name before applying any changes.
			for _, perm := range newUser.Permissions {
				// Skip blank entries that might result from trailing commas
				// in a comma-separated input.
				if strings.TrimSpace(perm) == "" {
					continue
				}

				// Strip the leading +/- modifier (if any) before validation
				// so we check the bare permission name against the known list.
				if perm[0] == '+' || perm[0] == '-' {
					perm = perm[1:]
				}

				if strings.HasPrefix(perm, "ego.") {
					// Full "ego.something" form — must match the known list.
					if !util.InListInsensitive(perm, defs.AllPermissions...) {
						msg := errors.ErrInvalidPermission.Clone().Context(perm).Error()

						return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
					}
				} else {
					// No "ego." prefix — check whether adding it would match a
					// known permission, and guide the caller to the right spelling.
					testPerm := "ego." + strings.ToLower(perm)
					if util.InListInsensitive(testPerm, defs.AllPermissions...) {
						msg := errors.ErrAmbiguousPermission.Clone().Context(perm).Chain(errors.ErrDidYouMean.Clone().Context(testPerm)).Error()

						return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
					}
				}
			}

			// Build a set (map[string]bool) from the user's current permissions
			// so we can add and remove entries efficiently.
			set := map[string]bool{}
			for _, perm := range u.Permissions {
				set[perm] = true
			}

			// Apply each permission in the request body:
			//   "-foo" → remove "foo" from the set
			//   "+foo" or "foo" → add "foo" to the set
			for _, perm := range newUser.Permissions {
				add := true
				if strings.HasPrefix(perm, "-") {
					add = false
					perm = strings.TrimPrefix(perm, "-")
				} else {
					// Strip leading "+" if present; the flag stays true.
					perm = strings.TrimPrefix(perm, "+")
				}

				if add {
					set[perm] = true
				} else {
					delete(set, perm)
				}
			}

			// Convert the set back to a plain []string and store it on the
			// user record. Iteration order over a map is not guaranteed in Go,
			// but permissions are not order-sensitive.
			u.Permissions = []string{}
			for perm := range set {
				u.Permissions = append(u.Permissions, perm)
			}

			changed = true
		}

		// Only write to the auth store if something actually changed. auth
		// operations flush the cache, so skipping an unnecessary write avoids
		// evicting cached credentials without cause.
		if changed {
			if err := auth.AuthService.WriteUser(session.ID, u); err != nil {
				return util.ErrorResponse(w, session.ID, "error updating "+name+", "+err.Error(), http.StatusNotFound)
			}
		}

		// Build the response. Replace the stored password hash with the elided
		// placeholder before sending the record back to the caller.
		w.Header().Add(defs.ContentTypeHeader, defs.UserMediaType)

		u.Password = defs.ElidedPassword

		response := defs.UserResponse{
			ServerInfo: util.MakeServerInfo(session.ID),
			User:       u,
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
