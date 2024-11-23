package users

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
)

// UpdateUserHandler is the handler for the PATH method on the users endpoint with a username
// provided in the path. This will update the password or permissions data for a user.
func UpdateUserHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	name := data.String(session.URLParts["name"])
	if u, err := auth.AuthService.ReadUser(name, false); err != nil {
		return util.ErrorResponse(w, session.ID, "no such user: "+name, http.StatusNotFound)
	} else {
		// Let's see if we can read the payload with update(s) to the user to apply.
		newUser, err := getUserFromBody(r, session)
		if err != nil {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}

		changed := false

		if newUser.Name != u.Name {
			return util.ErrorResponse(w, session.ID, "cannot change user name", http.StatusBadRequest)
		}

		if newUser.Password != "" {
			u.Password = newUser.Password
			changed = true
		}

		if len(newUser.Permissions) > 0 {
			// Make a set of the current permmissions
			set := map[string]bool{}
			for _, perm := range u.Permissions {
				set[perm] = true
			}

			// Scan over the supplied permissions, and add or delete
			// them based on the first character (add is assumed if
			// the marker is not present).
			for _, perm := range newUser.Permissions {
				add := true
				if strings.HasPrefix(perm, "-") {
					add = false
					perm = strings.TrimPrefix(perm, "-")
				} else {
					perm = strings.TrimPrefix(perm, "+")
				}

				if add {
					set[perm] = true
				} else {
					delete(set, perm)
				}
			}

			// Now re-apply the resulting set to the user
			u.Permissions = []string{}
			for perm := range set {
				u.Permissions = append(u.Permissions, perm)
			}

			changed = true
		}

		// Update the user record if we made a change to it.
		if changed {
			if err := auth.AuthService.WriteUser(u); err != nil {
				return util.ErrorResponse(w, session.ID, "error updating "+name+", "+err.Error(), http.StatusNotFound)
			}
		}

		// Write the updated user info back to the caller. We do not return the
		// password hash string.
		w.Header().Add(defs.ContentTypeHeader, defs.UserMediaType)

		// if the password is not empty, set it to "Enabled" so the caller knows
		// there is a password set.
		if u.Password != "" {
			u.Password = "Enabled"
		}

		r := defs.UserResponse{
			ServerInfo: util.MakeServerInfo(session.ID),
			User:       u,
		}

		b, _ := json.Marshal(r)
		_, _ = w.Write(b)
		session.ResponseLength += len(b)

		return http.StatusOK
	}
}
