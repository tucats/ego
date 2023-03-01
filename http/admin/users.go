package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	auth "github.com/tucats/ego/http/auth"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func userAction(sessionID int, w http.ResponseWriter, r *http.Request) int {
	var (
		err  error = nil
		name       = ""
		u          = defs.User{Permissions: []string{}}
	)

	if !util.InList(r.Method, http.MethodPost, http.MethodDelete, http.MethodGet) {
		msg := fmt.Sprintf("Unsupported method %s", r.Method)

		return util.ErrorResponse(w, sessionID, msg, http.StatusTeapot)
	}

	if r.Method == http.MethodPost {
		// Get the payload which must be a user spec in JSON
		buf := new(bytes.Buffer)

		_, _ = buf.ReadFrom(r.Body)
		err = json.Unmarshal(buf.Bytes(), &u)

		name = u.Name
	} else {
		name = strings.TrimPrefix(r.URL.Path, defs.AdminUsersPath)
		if name != "" {
			if ud, err := auth.AuthService.ReadUser(name, false); err == nil {
				u = ud
			}

			u.Name = name
		}
	}

	if err == nil {
		s := symbols.NewSymbolTable(r.URL.Path)

		s.SetAlways("_superuser", true)

		switch strings.ToUpper(r.Method) {
		// UPDATE OR CREATE A USER
		case http.MethodPost:
			args := data.NewMap(data.StringType, data.InterfaceType)
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

			_, err = auth.SetUser(s, data.NewList(args))
			if err == nil {
				u, err = auth.AuthService.ReadUser(name, false)
				if err == nil {
					u.Name = name
					response = u
				} else {
					return util.ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)
				}
			}

			if err == nil {
				w.Header().Add(contentTypeHeader, defs.UserMediaType)
				w.WriteHeader(http.StatusOK)

				msg, _ := json.Marshal(response)
				_, _ = w.Write(msg)

				return http.StatusOK
			}

		// DELETE A USER
		case http.MethodDelete:
			// Clear the password for the return response object
			shouldReturn, returnValue := deleteUserMethod(name, w, sessionID, s)
			if shouldReturn {
				return returnValue
			}

		// GET A COLLECTION OR A SPECIFIC USER
		case http.MethodGet:
			// If it's a single user, do that.
			if name != "" {
				status := http.StatusOK
				u.Password = ""

				if u.ID == uuid.Nil {
					return util.ErrorResponse(w, sessionID, fmt.Sprintf("User %s not found", name), http.StatusNotFound)
				}

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

			ui.Log(ui.RestLogger, "[%d] Returned info on %d users", sessionID, len(result.Items))

			return http.StatusOK
		}
	}

	// We had some kind of error, so report that.
	return util.ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)
}

func deleteUserMethod(name string, w http.ResponseWriter, sessionID int, s *symbols.SymbolTable) (bool, int) {
	u, userErr := auth.AuthService.ReadUser(name, false)
	if userErr != nil {
		msg := fmt.Sprintf("No username entry for '%s'", name)

		return true, util.ErrorResponse(w, sessionID, msg, http.StatusNotFound)
	}

	u.Password = ""
	response := u

	v, err := auth.DeleteUser(s, data.NewList(u.Name))
	if err != nil || !data.Bool(v) {
		msg := fmt.Sprintf("No username entry for '%s'", u.Name)

		return true, util.ErrorResponse(w, sessionID, msg, http.StatusNotFound)
	}

	if err == nil {
		b, _ := json.Marshal(response)

		w.Header().Add(contentTypeHeader, defs.UserMediaType)
		_, _ = w.Write(b)

		return true, http.StatusOK
	}

	return false, 0
}
