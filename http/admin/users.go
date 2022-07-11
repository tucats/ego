package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	auth "github.com/tucats/ego/http/auth"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func userAction(sessionID int32, w http.ResponseWriter, r *http.Request) int {
	var err error

	var name string

	var u = defs.User{Permissions: []string{}}

	user, hasAdminPrivileges := isAdminRequestor(r)
	if !hasAdminPrivileges {
		util.ErrorResponse(w, sessionID, fmt.Sprintf("User %s not authorized to access credentials", user), http.StatusForbidden)

		return http.StatusForbidden
	}

	if !util.InList(r.Method, http.MethodPost, http.MethodDelete, http.MethodGet) {
		msg := fmt.Sprintf("Unsupported method %s", r.Method)

		util.ErrorResponse(w, sessionID, msg, http.StatusTeapot)

		return http.StatusTeapot
	}

	logHeaders(r, sessionID)

	if r.Method == http.MethodPost {
		// Get the payload which must be a user spec in JSON
		buf := new(bytes.Buffer)

		_, _ = buf.ReadFrom(r.Body)
		err = json.Unmarshal(buf.Bytes(), &u)

		name = u.Name
	} else {
		name = strings.TrimPrefix(r.URL.Path, defs.AdminUsersPath)
		if name != "" {
			if ud, ok := auth.AuthService.ReadUser(name); errors.Nil(ok) {
				u = ud
			}

			u.Name = name
		}
	}

	if errors.Nil(err) {
		s := symbols.NewSymbolTable(r.URL.Path)

		_ = s.SetAlways("_superuser", true)

		switch strings.ToUpper(r.Method) {
		// UPDATE OR CREATE A USER
		case http.MethodPost:
			args := datatypes.NewMap(datatypes.StringType, datatypes.InterfaceType)
			_, _ = args.Set("name", u.Name)
			_, _ = args.Set("password", u.Password)

			// Only replace permissions if the list is non-empty
			if len(u.Permissions) > 0 {
				// Have to convert this from string array to interface array.
				perms := []interface{}{}

				for _, p := range u.Permissions {
					perms = append(perms, p)
				}

				_, _ = args.Set("permissions", perms)
			}

			var response defs.User

			_, err = auth.SetUser(s, []interface{}{args})
			if errors.Nil(err) {
				u, err = auth.AuthService.ReadUser(name)
				if errors.Nil(err) {
					u.Name = name
					response = u
				} else {
					util.ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

					return http.StatusInternalServerError
				}
			}

			if errors.Nil(err) {
				w.Header().Add(contentTypeHeader, defs.UserMediaType)
				w.WriteHeader(http.StatusOK)

				msg, _ := json.Marshal(response)
				_, _ = w.Write(msg)

				ui.Debug(ui.ServerLogger, "[%d] 200 Success", sessionID)

				return http.StatusOK
			}

		// DELETE A USER
		case http.MethodDelete:
			u, exists := auth.AuthService.ReadUser(name)
			if !errors.Nil(exists) {
				msg := fmt.Sprintf("No username entry for '%s'", name)

				util.ErrorResponse(w, sessionID, msg, http.StatusNotFound)

				return http.StatusNotFound
			}

			// Clear the password for the return response object
			u.Password = ""
			response := u

			v, err := auth.DeleteUser(s, []interface{}{u.Name})
			if !errors.Nil(err) || !datatypes.GetBool(v) {
				msg := fmt.Sprintf("No username entry for '%s'", u.Name)

				util.ErrorResponse(w, sessionID, msg, http.StatusNotFound)

				return http.StatusNotFound
			}

			if errors.Nil(err) {
				b, _ := json.Marshal(response)

				w.Header().Add(contentTypeHeader, defs.UserMediaType)
				_, _ = w.Write(b)

				ui.Debug(ui.ServerLogger, "[%d] 200 Success", sessionID)

				return http.StatusOK
			}

		// GET A COLLECTION OR A SPECIFIC USER
		case http.MethodGet:
			// If it's a single user, do that.
			if name != "" {
				status := http.StatusOK
				msg := successMessage
				u.Password = ""

				if u.ID == uuid.Nil {
					util.ErrorResponse(w, sessionID, fmt.Sprintf("User %s not found", name), http.StatusNotFound)

					return http.StatusNotFound
				}

				ui.Debug(ui.ServerLogger, fmt.Sprintf("[%d] %d %s", sessionID, status, msg))
				w.Header().Add(contentTypeHeader, defs.UserMediaType)

				result := u
				b, _ := json.Marshal(result)
				_, _ = w.Write(b)

				return status
			}

			result := defs.UserCollection{
				BaseCollection: util.MakeBaseCollection(sessionID),
				Items:          []defs.User{},
			}

			userDatabase := auth.AuthService.ListUsers()
			for k, u := range userDatabase {
				ud := defs.User{}
				ud.Name = k
				ud.ID = u.ID
				ud.Permissions = u.Permissions
				result.Items = append(result.Items, ud)
			}

			result.Count = len(result.Items)
			result.Start = 0

			b, _ := json.Marshal(result)

			w.Header().Add(contentTypeHeader, defs.UsersMediaType)
			_, _ = w.Write(b)

			ui.Debug(ui.ServerLogger, "[%d] 200 returned info on %d users", sessionID, len(result.Items))

			return http.StatusOK
		}
	}

	// We had some kind of error, so report that.
	util.ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)
	ui.Debug(ui.ServerLogger, "[%d] 500 Internal server error %v", sessionID, err)

	return http.StatusInternalServerError
}