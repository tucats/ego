package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"

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
	sessionID := atomic.AddInt32(&nextSessionID, 1)
	requestor := r.RemoteAddr

	if forward := r.Header.Get("X-Forwarded-For"); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	// If INFO logging, put out the prologue message for the operation.
	ui.Debug(ui.InfoLogger, "[%d] %s %s; from %s", sessionID, r.Method, r.URL.Path, requestor)

	// Do the actual work.
	status := userHandler(sessionID, w, r)

	// If not doing INFO logging, no intermediate messages have been generated so we can generate a single summary here.
	if !ui.LoggerIsActive(ui.InfoLogger) {
		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; status %d; content: json", sessionID, r.Method, r.URL.Path, requestor, status)
	}
}

func CachesHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := atomic.AddInt32(&nextSessionID, 1)
	requestor := r.RemoteAddr

	if forward := r.Header.Get("X-Forwarded-For"); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	ui.Debug(ui.InfoLogger, "[%d] %s %s; from %s", sessionID, r.Method, r.URL.Path, requestor)
	status := cachesHandler(sessionID, w, r)

	if !ui.LoggerIsActive(ui.InfoLogger) {
		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; status %d; content: json", sessionID, r.Method, r.URL.Path, requestor, status)
	}
}

func LoggingHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := atomic.AddInt32(&nextSessionID, 1)
	requestor := r.RemoteAddr

	if forward := r.Header.Get("X-Forwarded-For"); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	ui.Debug(ui.InfoLogger, "[%d] %s %s; from %s", sessionID, r.Method, r.URL.Path, requestor)
	status := loggingHandler(sessionID, w, r)

	if !ui.LoggerIsActive(ui.InfoLogger) {
		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; status %d; content: json", sessionID, r.Method, r.URL.Path, requestor, status)
	}
}

func userHandler(sessionID int32, w http.ResponseWriter, r *http.Request) int {
	var err error

	var name string

	var u = defs.User{Permissions: []string{}}

	w.Header().Add("Content-Type", defs.JSONMediaType)

	user, hasAdminPrivileges := isAdminRequestor(r)
	if !hasAdminPrivileges {
		ui.Debug(ui.InfoLogger, "[%d] User %s not authorized", sessionID, user)
		w.WriteHeader(http.StatusForbidden)

		msg := `{ "status" : 403, "msg" : "Not authorized" }`

		_, _ = io.WriteString(w, msg)

		return 403
	}

	if !util.InList(r.Method, "POST", "DELETE", "GET") {
		w.WriteHeader(http.StatusTeapot)

		msg := `{ "status" : 418, "msg" : "Unsupported method %s" }`

		_, _ = io.WriteString(w, fmt.Sprintf(msg, r.Method))

		return 418
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

				ui.Debug(ui.InfoLogger, "[%d] 200 Success", sessionID)

				return 200
			}

		// DELETE A USER
		case "DELETE":
			u, exists := service.ReadUser(name)
			if !errors.Nil(exists) {
				w.WriteHeader(http.StatusNotFound)

				msg := `{ "status" : 404, "msg" : "No username entry for '%s'" }`

				_, _ = io.WriteString(w, fmt.Sprintf(msg, name))

				ui.Debug(ui.InfoLogger, "[%d] 404 No such user", sessionID)

				return 404
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

				ui.Debug(ui.InfoLogger, "[%d] 404 No such user", sessionID)

				return 404
			}

			if errors.Nil(err) {
				b, _ := json.Marshal(response)

				w.WriteHeader(http.StatusOK)

				_, _ = w.Write(b)

				ui.Debug(ui.InfoLogger, "[%d] 200 Success", sessionID)

				return 200
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

				ui.Debug(ui.InfoLogger, fmt.Sprintf("[%d] %d %s", sessionID, status, msg))

				return status
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

			ui.Debug(ui.InfoLogger, "[%d] 200 returned info on %d users", sessionID, len(result.Items))

			return 200
		}
	}

	// We had some kind of error, so report that.
	w.WriteHeader(http.StatusInternalServerError)

	msg := `{ "status" : HTTP.STATUSINTERNALSERVERERROR, "msg" : "%s"`

	_, _ = io.WriteString(w, fmt.Sprintf(msg, err.Error()))

	ui.Debug(ui.InfoLogger, "[%d] 500 ]Internal server error %v", sessionID, err)

	return 500
}

// FlushCacheHandler is the rest handler for /admin/caches endpoint.
func cachesHandler(sessionID int32, w http.ResponseWriter, r *http.Request) int {
	w.Header().Add("Content-Type", defs.JSONMediaType)

	user, hasAdminPrivileges := isAdminRequestor(r)
	if !hasAdminPrivileges {
		ui.Debug(ui.InfoLogger, "[%d] User %s not authorized", sessionID, user)
		w.WriteHeader(http.StatusForbidden)

		msg := `{ "status" : 403, "msg" : "Not authorized" }`

		_, _ = io.WriteString(w, msg)

		return 403
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

		ui.Debug(ui.InfoLogger, fmt.Sprintf("[%d] %d, sending JSON response", sessionID, result.Status))

		return result.Status

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

		ui.Debug(ui.InfoLogger, "[%d] 200, sending JSON response", sessionID)

		return 200

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

		ui.Debug(ui.InfoLogger, "[%d] 200, sending JSON response", sessionID)

		return 200

	default:
		w.WriteHeader(http.StatusTeapot)

		msg := `{ "status" : 418, "msg" : "Unsupported method %s" }`

		_, _ = io.WriteString(w, fmt.Sprintf(msg, r.Method))

		ui.Debug(ui.InfoLogger, "[%d] 418, sending JSON response: unsupported method %s", sessionID, r.Method)

		return 418
	}
}

// For a given userid, indicate if this user exists and has admin privileges.
func isAdminRequestor(r *http.Request) (string, bool) {
	var user string

	hasAdminPrivileges := false

	auth := r.Header.Get("Authorization")
	if auth == "" {
		ui.Debug(ui.InfoLogger, "No authentication credentials given")

		return "<invalid>", false
	}

	// IF the authorization header has the auth scheme prefix, extract and
	// validate the token
	if strings.HasPrefix(strings.ToLower(auth), defs.AuthScheme) {
		token := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(auth), defs.AuthScheme))

		tokenString := token
		if len(tokenString) > 10 {
			tokenString = tokenString[:10] + "..."
		}

		ui.Debug(ui.InfoLogger, "Auth using token %s...", tokenString)

		if validateToken(token) {
			user := tokenUser(token)
			if user == "" {
				ui.Debug(ui.InfoLogger, "No username associated with token")
			}

			hasAdminPrivileges = getPermission(user, "root")
		} else {
			ui.Debug(ui.InfoLogger, "No valid token presented")
		}
	} else {
		// Not a token, so assume BasicAuth
		user, pass, ok := r.BasicAuth()
		if ok {
			ui.Debug(ui.InfoLogger, "Auth using user %s", user)

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

// loggingHandler is the rest handler for /admin/logging endpoint.
func loggingHandler(sessionID int32, w http.ResponseWriter, r *http.Request) int {
	w.Header().Add("Content-Type", defs.JSONMediaType)

	user, hasAdminPrivileges := isAdminRequestor(r)
	if !hasAdminPrivileges {
		ui.Debug(ui.InfoLogger, "[%d] User %s not authorized", sessionID, user)
		w.WriteHeader(http.StatusForbidden)

		msg := `{ "status" : 403, "msg" : "Not authorized" }`

		_, _ = io.WriteString(w, msg)

		return 403
	}

	if r.Method != "POST" {
		ui.Debug(ui.InfoLogger, "[%d] 405 Unsupported method %s", sessionID, r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)

		msg := `{ "status" : 405, "msg" : "Method not allowed" }`
		_, _ = io.WriteString(w, msg)

		return http.StatusMethodNotAllowed
	}

	name := strings.ToUpper(strings.TrimPrefix(r.URL.Path, "/admin/loggers/"))
	if name == "" {
		ui.Debug(ui.InfoLogger, "[%d] 400 Missing logger name", sessionID)
		w.WriteHeader(http.StatusBadRequest)

		msg := `{ "status" : 400, "msg" : "Missing logger name" }`
		_, _ = io.WriteString(w, msg)

		return http.StatusBadRequest
	}

	logger := ui.Logger(name)
	if logger < 0 || logger == ui.ServerLogger {
		ui.Debug(ui.InfoLogger, "[%d] 400 Invalid logger name %s", sessionID, name)
		w.WriteHeader(http.StatusBadRequest)

		msg := `{ "status" : 400, "msg" : "Invalid logger name" }`
		_, _ = io.WriteString(w, msg)

		return http.StatusBadRequest
	}

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)

	mode := strings.ToLower(buf.String())

	switch mode {
	case "true", "on", "enable", "1", "yes":
		ui.Debug(ui.ServerLogger, "[%d] Enable logger \"%s\" (%d)", sessionID, name, logger)
		ui.SetLogger(logger, true)

	case "false", "off", "disable", "0", "no":
		ui.Debug(ui.ServerLogger, "[%d] Disable logger \"%s\" (%d)", sessionID, name, logger)
		ui.SetLogger(logger, false)

	default:
		ui.Debug(ui.InfoLogger, "[%d] 400 Invalid logger state %s", sessionID, mode)
		w.WriteHeader(http.StatusBadRequest)

		msg := fmt.Sprintf(`{ "status" : 400, "msg" : "Invalid logger state %s}`, mode)
		_, _ = io.WriteString(w, msg)

		return http.StatusBadRequest
	}

	ui.Debug(ui.InfoLogger, "[%d] 200 OK", sessionID)
	w.WriteHeader(http.StatusOK)

	return http.StatusOK
}
