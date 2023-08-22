package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	auth "github.com/tucats/ego/http/auth"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// CreateUserHandler is the handler for the POST method on the users endpoint. It creates a
// new user using the JSON payload in the request.
func CreateUserHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	userInfo, err := getUserFromBody(r)
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
			}

			msg, _ := json.Marshal(r)
			_, _ = w.Write(msg)

			return http.StatusOK
		} else {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
		}
	} else {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}
}

// ListUsersHandler is the handler for the GET method on the users endpoint. If a name was
// specified in the URL, this calls the individual user "GET" function, else it returns a
// list of all users.
func ListUsersHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	result := defs.UserCollection{
		BaseCollection: util.MakeBaseCollection(session.ID),
		Items:          []defs.User{},
	}

	userDatabase := auth.AuthService.ListUsers()

	for k, u := range userDatabase {
		ud := defs.User{
			Name:        k,
			ID:          u.ID,
			Permissions: u.Permissions,
		}
		result.Items = append(result.Items, ud)
	}

	// sort result.Items by Name
	sort.Slice(result.Items, func(i, j int) bool {
		return result.Items[i].Name < result.Items[j].Name
	})

	result.Count = len(result.Items)
	result.Start = 0

	w.Header().Add(defs.ContentTypeHeader, defs.UsersMediaType)

	// convert result to json and write to response
	b, _ := json.Marshal(result)
	_, _ = w.Write(b)

	return http.StatusOK
}

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

		return http.StatusOK
	}
}

// UpdateUserHandler is the handler for the PATH method on the users endpoint with a username
// provided in the path. This will update the password or permissions data for a user.
func UpdateUserHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	name := data.String(session.URLParts["name"])
	if u, err := auth.AuthService.ReadUser(name, false); err != nil {
		return util.ErrorResponse(w, session.ID, "no such user: "+name, http.StatusNotFound)
	} else {
		// Let's see if we can read the payload with update(s) to the user to apply.
		newUser, err := getUserFromBody(r)
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

		return http.StatusOK
	}
}

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
	if err != nil || !data.Bool(v) {
		msg := fmt.Sprintf("No username entry for '%s'", u.Name)

		return util.ErrorResponse(w, session.ID, msg, http.StatusNotFound)
	}

	// Write the deleted user record back to the caller.
	w.Header().Add(defs.ContentTypeHeader, defs.UserMediaType)

	b, _ := json.Marshal(u)
	_, _ = w.Write(b)

	return http.StatusOK
}

// getUserFromBody is a helper function that retrieves a User object from
// the request body payload.
func getUserFromBody(r *http.Request) (*defs.User, error) {
	userInfo := defs.User{Permissions: []string{}}

	// Get the payload which must be a user spec in JSON
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(r.Body); err == nil {
		if err = json.Unmarshal(buf.Bytes(), &userInfo); err != nil {
			return nil, err
		}
	}

	return &userInfo, nil
}
