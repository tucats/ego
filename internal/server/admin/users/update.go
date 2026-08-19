package users

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/auth"
	"github.com/tucats/ego/internal/util"
)

// UpdateUserHandler is the HTTP handler for PATCH /admin/users/{name}. It
// applies partial updates — password and/or permissions — to an existing user.
// The username itself cannot be changed via this endpoint.
//
// Permissions in the request body may be prefixed with "+" (add) or "-"
// (remove). An unprefixed permission is treated as an add. Permissions not
// present in the request body are left unchanged.
func UpdateUserHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// Extract the target username from the URL path captured by the router.
	name := data.String(session.URLParts["name"])

	// Look up the existing user record. If it does not exist, return 404.
	// The "if u, err := …; err != nil { … } else { … }" pattern is idiomatic
	// Go: u and err are scoped to this if/else block.
	if u, err := auth.AuthService.ReadUser(session.ID, name, false); err != nil {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.user.name.not.found", ui.A{"name": name}), http.StatusNotFound)
	} else {
		// Decode the JSON request body into a new defs.User struct that carries
		// the fields the caller wants to change.
		newUser, err := getUserFromBody(r, session)
		if err != nil {
			return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
		}

		// Track whether any actual change was made so we can skip the write
		// operation when the request body is a no-op.
		changed := false

		// Renaming users is not supported — reject the request if the body
		// specifies a different name than the URL.
		if newUser.Name != u.Name {
			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.user.rename.denied"), http.StatusBadRequest)
		}

		// If a new password was provided, hash it and replace the stored hash.
		// An empty password string means "leave the password unchanged".
		if newUser.Password != "" {
			h, err := auth.HashPassword(newUser.Password)
			if err != nil {
				return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.user.hash.failed", ui.A{"err": err.Error()}), http.StatusInternalServerError)
			}

			u.Password = h
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
						msg := errors.ErrInvalidPermission.Clone().Context(perm).Localize(session.Language)

						return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
					}
				} else {
					// No "ego." prefix — check whether adding it would match a
					// known permission, and guide the caller to the right spelling.
					testPerm := "ego." + strings.ToLower(perm)
					if util.InListInsensitive(testPerm, defs.AllPermissions...) {
						msg := errors.ErrAmbiguousPermission.Clone().Context(perm).Chain(errors.ErrDidYouMean.Clone().Context(testPerm)).Localize(session.Language)

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
					// SECURITY RULE: "you cannot grant a permission you do
					// not hold yourself." This check only runs on the "add"
					// side of a +/- update, never on "-" (removing a
					// permission only takes privilege away, it can never be
					// used to escalate, so it is always allowed).
					//
					// It also only applies to Ego's own built-in permission
					// names -- the ones starting with "ego." that appear in
					// defs.AllPermissions, like "ego.root" or
					// "ego.dsn.admin". Ego also lets an operator invent
					// arbitrary "custom" permission names (e.g. "payroll")
					// for their own service code to check -- see the
					// "ego server users create" example in docs/SERVER.md.
					// Those custom names carry no special meaning to the
					// server itself, so handing one to another user is not
					// a privilege-escalation risk the way handing out a
					// built-in "ego." permission is, and must keep working
					// exactly as it always has.
					//
					// Go note for readers new to the language:
					// strings.HasPrefix(perm, "ego.") reports whether perm
					// starts with the literal text "ego." -- true for
					// "ego.root", false for "payroll". "!" is the boolean
					// NOT operator and "&&" is boolean AND, so the full
					// condition below reads as "this is a built-in ego.*
					// permission, AND the caller is not root, AND the
					// caller does not already hold this exact permission."
					// All three must be true for the request to be
					// rejected.
					//
					// Why this check exists: without it, a caller who holds
					// only "ego.server.admin" -- a deliberately lesser
					// permission meant for day-to-day user and server
					// administration, not full control of the server --
					// could PATCH any user (including themselves) to add
					// "ego.root", the one permission that bypasses every
					// other check in the server. That would make
					// "ego.server.admin" secretly equivalent to full
					// "ego.root", which defeats the entire point of having
					// two separate permission levels. See create.go's
					// identical check (and its longer comment) for the
					// create-a-new-user side of this same rule.
					//
					// session.Admin means "this caller holds ego.root" (set
					// once at login time -- see router/auth.go). Root can
					// always grant anything, including root itself, so
					// "!session.Admin" short-circuits the rest of the
					// condition and skips straight to allowing the grant
					// for a root caller. session.HasAllPermissions(perm)
					// answers "does the caller's own permission list
					// already contain perm?" -- if not, they are trying to
					// hand out something they don't have themselves.
					if strings.HasPrefix(perm, "ego.") && !session.Admin && !session.HasAllPermissions(perm) {
						msg := errors.ErrPermissionNotHeld.Clone().Context(perm).Localize(session.Language)

						return util.ErrorResponse(w, session.ID, msg, http.StatusForbidden)
					}

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
			// The ReadUser call at the top of this handler already confirmed
			// the user exists, so a WriteUser failure here is a storage
			// fault, not a "not found" condition.
			if err := auth.AuthService.WriteUser(session.ID, u); err != nil {
				return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.user.update.failed", ui.A{"name": name, "err": err.Error()}), http.StatusInternalServerError)
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

		b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

		if ui.IsActive(ui.RestLogger) {
			ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
				"session": session.ID,
				"body":    string(b)})
		}

		return http.StatusOK
	}
}
