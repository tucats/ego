package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// UserHandler is the rest handler for /admin/user endpoint
// operations.
func UserHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	var name string

	var u = defs.User{Permissions: []string{}}

	ui.Debug(ui.ServerLogger, "%s %s", r.Method, r.URL.Path)
	w.Header().Add("Content_Type", defs.JSONMediaType)

	user, hasAdminPrivileges := isAdminRequestor(r)
	if !hasAdminPrivileges {
		ui.Debug(ui.ServerLogger, "User %s not authorized", user)
		w.WriteHeader(http.StatusForbidden)

		msg := `{ "status" : 403, "msg" : "Not authorized" }`

		_, _ = io.WriteString(w, msg)

		return
	}

	if !util.InList(r.Method, "POST", "DELETE", "GET") {
		w.WriteHeader(http.StatusTeapot)

		msg := `{ "status" : 418, "msg" : "Unsupported method %s" }`

		_, _ = io.WriteString(w, fmt.Sprintf(msg, r.Method))

		return
	}

	if r.Method == "POST" {
		// Get the payload which must be a user spec in JSON
		buf := new(bytes.Buffer)

		_, _ = buf.ReadFrom(r.Body)
		err = json.Unmarshal(buf.Bytes(), &u)

		name = u.Name
	} else {
		name = strings.TrimPrefix(r.URL.Path, "/admin/users/")
		if name != "" {
			if ud, ok := service.ReadUser(name); errors.Nil(ok) {
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
		case "POST":
			args := datatypes.NewMap(datatypes.StringTypeDef, datatypes.InterfaceTypeDef)
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

			var response defs.UserResponse

			_, err = SetUser(s, []interface{}{args})
			if errors.Nil(err) {
				u, err = service.ReadUser(name)
				if errors.Nil(err) {
					u.Name = name
					response = defs.UserResponse{
						User: u,
						RestResponse: defs.RestResponse{
							Status:  http.StatusOK,
							Message: fmt.Sprintf("successfully updated user '%s'", u.Name),
						},
					}
				} else {
					response = defs.UserResponse{
						User: u,
						RestResponse: defs.RestResponse{
							Status:  http.StatusInternalServerError,
							Message: err.Error(),
						},
					}
				}
			}

			if errors.Nil(err) {
				w.WriteHeader(http.StatusOK)

				msg, _ := json.Marshal(response)

				_, _ = io.WriteString(w, string(msg))

				ui.Debug(ui.ServerLogger, "200 Success")

				return
			}

		// DELETE A USER
		case "DELETE":
			u, exists := service.ReadUser(name)
			if !errors.Nil(exists) {
				w.WriteHeader(http.StatusNotFound)

				msg := `{ "status" : 404, "msg" : "No username entry for '%s'" }`

				_, _ = io.WriteString(w, fmt.Sprintf(msg, name))

				ui.Debug(ui.ServerLogger, "404 No such user")

				return
			}

			// Clear the password for the return response object
			u.Password = ""
			response := defs.UserResponse{
				User: u,
				RestResponse: defs.RestResponse{
					Status:  http.StatusOK,
					Message: fmt.Sprintf("successfully deleted user '%s'", name),
				},
			}

			v, err := DeleteUser(s, []interface{}{u.Name})
			if errors.Nil(err) && !util.GetBool(v) {
				w.WriteHeader(http.StatusNotFound)

				msg := `{ "status" : 404, "msg" : "No username entry for '%s'" }`

				_, _ = io.WriteString(w, fmt.Sprintf(msg, name))

				ui.Debug(ui.ServerLogger, "404 No such user")

				return
			}

			if errors.Nil(err) {
				b, _ := json.Marshal(response)

				w.WriteHeader(http.StatusOK)

				_, _ = w.Write(b)

				ui.Debug(ui.ServerLogger, "200 Success")

				return
			}

		// GET A COLLECTION OR A SPECIFIC USER
		case "GET":
			// If it's a single user, do that.
			if name != "" {
				status := http.StatusOK
				msg := "Success"
				u.Password = ""

				if u.ID == uuid.Nil {
					status = http.StatusNotFound
					msg = "User not found"
				}

				result := defs.UserResponse{
					User: u,
					RestResponse: defs.RestResponse{
						Status:  status,
						Message: msg,
					},
				}
				b, _ := json.Marshal(result)

				w.WriteHeader(status)

				_, _ = w.Write(b)

				ui.Debug(ui.ServerLogger, fmt.Sprintf("%d %s", status, msg))

				return
			}

			result := defs.UserCollection{
				Items: []defs.User{},
			}
			result.Status = http.StatusOK

			userDatabase := service.ListUsers()
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

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(b)

			ui.Debug(ui.ServerLogger, "200 returned info on %d users", len(result.Items))

			return
		}
	}

	// We had some kind of error, so report that.
	w.WriteHeader(http.StatusInternalServerError)

	msg := `{ "status" : HTTP.STATUSINTERNALSERVERERROR, "msg" : "%s"`

	_, _ = io.WriteString(w, fmt.Sprintf(msg, err.Error()))

	ui.Debug(ui.ServerLogger, "HTTP.STATUSINTERNALSERVERERROR Internal server error %v", err)
}

// FlushCacheHandler is the rest handler for /admin/caches endpoint.
func CachesHandler(w http.ResponseWriter, r *http.Request) {
	ui.Debug(ui.ServerLogger, "%s %s", r.Method, r.URL.Path)
	w.Header().Add("Content_Type", defs.JSONMediaType)

	user, hasAdminPrivileges := isAdminRequestor(r)
	if !hasAdminPrivileges {
		ui.Debug(ui.ServerLogger, "User %s not authorized", user)
		w.WriteHeader(http.StatusForbidden)

		msg := `{ "status" : 403, "msg" : "Not authorized" }`

		_, _ = io.WriteString(w, msg)

		return
	}

	switch r.Method {
	case "POST":
		var result defs.CacheResponse

		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)

		err := json.Unmarshal(buf.Bytes(), &result)
		if errors.Nil(err) {
			MaxCachedEntries = result.Limit
		}

		if !errors.Nil(err) {
			result.Status = http.StatusBadRequest
			result.Message = err.Error()
		} else {
			result = defs.CacheResponse{
				Count: len(serviceCache),
				Limit: MaxCachedEntries,
				Items: []defs.CachedItem{},
			}
			result.Status = http.StatusOK
			result.Message = "Success"

			for k, v := range serviceCache {
				result.Items = append(result.Items, defs.CachedItem{Name: k, LastUsed: v.age})
			}
		}

		b, _ := json.Marshal(result)
		_, _ = w.Write(b)

		ui.Debug(ui.ServerLogger, fmt.Sprintf("%d %s", result.Status, result.Message))

		return

	// Get the list of cached items.
	case "GET":
		result := defs.CacheResponse{
			Count: len(serviceCache),
			Limit: MaxCachedEntries,
			Items: []defs.CachedItem{},
		}
		result.Status = http.StatusOK
		result.Message = "Success"

		for k, v := range serviceCache {
			result.Items = append(result.Items, defs.CachedItem{Name: k, LastUsed: v.age, Count: v.count})
		}

		b, _ := json.Marshal(result)
		_, _ = w.Write(b)

		ui.Debug(ui.ServerLogger, "200 Success")

		return

	// DELETE the cached service compilation units. In-flight services
	// are unaffected.
	case "DELETE":
		w.WriteHeader(http.StatusOK)

		serviceCache = map[string]cachedCompilationUnit{}
		result := defs.CacheResponse{
			Count: 0,
			Limit: MaxCachedEntries,
			Items: []defs.CachedItem{},
		}
		result.Status = http.StatusOK
		result.Message = "Success"

		b, _ := json.Marshal(result)
		_, _ = w.Write(b)

		ui.Debug(ui.ServerLogger, "200 Success")

		return

	default:
		w.WriteHeader(http.StatusTeapot)

		msg := `{ "status" : 418, "msg" : "Unsupported method %s" }`

		_, _ = io.WriteString(w, fmt.Sprintf(msg, r.Method))

		return
	}
}

// For a given userid, indicate if this user exists and has admin privileges.
func isAdminRequestor(r *http.Request) (string, bool) {
	var user string

	hasAdminPrivileges := false

	auth := r.Header.Get("Authorization")
	if auth == "" {
		ui.Debug(ui.ServerLogger, "No authentication credentials given")

		return "<invalid>", false
	}

	// IF the authorization header has the auth scheme prefix, extract and
	// validate the token
	if strings.HasPrefix(strings.ToLower(auth), defs.AuthScheme) {
		token := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(auth), defs.AuthScheme))

		tokenString := token
		if len(tokenString) > 20 {
			tokenString = tokenString[:20] + "..."
		}

		ui.Debug(ui.ServerLogger, "Auth using token %s...", tokenString)

		if validateToken(token) {
			user := tokenUser(token)
			if user == "" {
				ui.Debug(ui.ServerLogger, "No username associated with token")
			}

			hasAdminPrivileges = getPermission(user, "root")
		} else {
			ui.Debug(ui.ServerLogger, "No valid token presented")
		}
	} else {
		// Not a token, so assume BasicAuth
		user, pass, ok := r.BasicAuth()
		if ok {
			ui.Debug(ui.ServerLogger, "Auth using user %s", user)

			if ok := validatePassword(user, pass); ok {
				hasAdminPrivileges = getPermission(user, "root")
			}
		}
	}

	if !hasAdminPrivileges && user == "" {
		user = "<invalid>"
	}

	return user, hasAdminPrivileges
}
