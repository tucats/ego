package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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

const (
	successMessage     = "Success"
	forwardedForHeader = "X-Forwarded-For"
	contentTypeHeader  = "Content-Type"
)

// UserHandler is the rest handler for /admin/user endpoint
// operations.
func UserHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := atomic.AddInt32(&nextSessionID, 1)
	requestor := r.RemoteAddr

	logRequest(r, sessionID)

	CountRequest(AdminRequestCounter)

	if forward := r.Header.Get(forwardedForHeader); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	// If INFO logging, put out the prologue message for the operation.
	ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s", sessionID, r.Method, r.URL.Path, requestor)
	ui.Debug(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	// Do the actual work.
	status := userHandler(sessionID, w, r)

	// If not doing INFO logging, no intermediate messages have been generated so we can generate a single summary here.
	if !ui.LoggerIsActive(ui.ServerLogger) {
		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; status %d; content: json", sessionID, r.Method, r.URL.Path, requestor, status)
	}
}

func CachesHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := atomic.AddInt32(&nextSessionID, 1)
	requestor := r.RemoteAddr

	logRequest(r, sessionID)

	CountRequest(AdminRequestCounter)

	if forward := r.Header.Get(forwardedForHeader); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s", sessionID, r.Method, r.URL.Path, requestor)
	ui.Debug(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	status := cachesHandler(sessionID, w, r)

	if !ui.LoggerIsActive(ui.ServerLogger) {
		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; status %d; content: json", sessionID, r.Method, r.URL.Path, requestor, status)
	}
}

func LoggingHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := atomic.AddInt32(&nextSessionID, 1)
	requestor := r.RemoteAddr

	logRequest(r, sessionID)

	CountRequest(AdminRequestCounter)

	if forward := r.Header.Get(forwardedForHeader); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s", sessionID, r.Method, r.URL.Path, requestor)
	ui.Debug(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	status := loggingHandler(sessionID, w, r)

	if !ui.LoggerIsActive(ui.ServerLogger) {
		ui.Debug(ui.ServerLogger, "[%d] %s %s; from %s; status %d; content: json", sessionID, r.Method, r.URL.Path, requestor, status)
	}
}

func userHandler(sessionID int32, w http.ResponseWriter, r *http.Request) int {
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

			_, err = SetUser(s, []interface{}{args})
			if errors.Nil(err) {
				u, err = service.ReadUser(name)
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
			u, exists := service.ReadUser(name)
			if !errors.Nil(exists) {
				msg := fmt.Sprintf("No username entry for '%s'", name)

				util.ErrorResponse(w, sessionID, msg, http.StatusNotFound)

				return http.StatusNotFound
			}

			// Clear the password for the return response object
			u.Password = ""
			response := u

			v, err := DeleteUser(s, []interface{}{u.Name})
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

// FlushCacheHandler is the rest handler for /admin/caches endpoint.
func cachesHandler(sessionID int32, w http.ResponseWriter, r *http.Request) int {
	user, hasAdminPrivileges := isAdminRequestor(r)
	if !hasAdminPrivileges {
		ui.Debug(ui.AuthLogger, "[%d] User %s not authorized", sessionID, user)
		util.ErrorResponse(w, sessionID, "Not authorized", http.StatusForbidden)

		return http.StatusForbidden
	}

	logHeaders(r, sessionID)

	switch r.Method {
	case http.MethodPost:
		var result defs.CacheResponse

		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)

		err := json.Unmarshal(buf.Bytes(), &result)
		if errors.Nil(err) {
			MaxCachedEntries = result.Limit
		}

		if !errors.Nil(err) {
			util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)

			return http.StatusBadRequest
		} else {
			result = defs.CacheResponse{
				ServerInfo: util.MakeServerInfo(sessionID),
				Count:      len(serviceCache),
				Limit:      MaxCachedEntries,
				Items:      []defs.CachedItem{},
			}

			for k, v := range serviceCache {
				result.Items = append(result.Items, defs.CachedItem{Name: k, LastUsed: v.age})
			}
		}

		w.Header().Add(contentTypeHeader, defs.CacheMediaType)

		b, _ := json.Marshal(result)
		_, _ = w.Write(b)

		ui.Debug(ui.ServerLogger, fmt.Sprintf("[%d] %d, sending JSON response", sessionID, http.StatusOK))

		return http.StatusOK

	// Get the list of cached items.
	case http.MethodGet:
		result := defs.CacheResponse{
			ServerInfo: util.MakeServerInfo(sessionID),
			Count:      len(serviceCache),
			Limit:      MaxCachedEntries,
			Items:      []defs.CachedItem{},
			AssetSize:  GetAssetCacheSize(),
			AssetCount: GetAssetCacheCount(),
		}

		for k, v := range serviceCache {
			result.Items = append(result.Items, defs.CachedItem{Name: k, LastUsed: v.age, Count: v.count})
		}

		for k, v := range assetCache {
			result.Items = append(result.Items, defs.CachedItem{Name: k, LastUsed: v.lastUsed, Count: v.count})
		}

		w.Header().Add(contentTypeHeader, defs.CacheMediaType)

		b, _ := json.Marshal(result)
		_, _ = w.Write(b)

		ui.Debug(ui.ServerLogger, "[%d] 200, sending JSON response", sessionID)

		return http.StatusOK

	// DELETE the cached service compilation units. In-flight services
	// are unaffected.
	case http.MethodDelete:
		FlushAssetCache()

		serviceCache = map[string]cachedCompilationUnit{}
		result := defs.CacheResponse{
			ServerInfo: util.MakeServerInfo(sessionID),
			Count:      0,
			Limit:      MaxCachedEntries,
			Items:      []defs.CachedItem{},
			AssetSize:  GetAssetCacheSize(),
			AssetCount: GetAssetCacheCount(),
		}

		w.Header().Add(contentTypeHeader, defs.CacheMediaType)

		b, _ := json.Marshal(result)
		_, _ = w.Write(b)

		ui.Debug(ui.ServerLogger, "[%d] 200, sending JSON response", sessionID)

		return http.StatusOK

	default:
		util.ErrorResponse(w, sessionID, "Unsupported method: "+r.Method, http.StatusTeapot)

		return http.StatusTeapot
	}
}

// For a given userid, indicate if this user exists and has admin privileges.
func isAdminRequestor(r *http.Request) (string, bool) {
	var user string

	hasAdminPrivileges := false

	auth := r.Header.Get("Authorization")
	if auth == "" {
		ui.Debug(ui.AuthLogger, "No authentication credentials given")

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

		ui.Debug(ui.AuthLogger, "Auth using token %s...", tokenString)

		if validateToken(token) {
			user := tokenUser(token)
			if user == "" {
				ui.Debug(ui.AuthLogger, "No username associated with token")
			}

			hasAdminPrivileges = getPermission(user, "root")
		} else {
			ui.Debug(ui.AuthLogger, "No valid token presented")
		}
	} else {
		// Not a token, so assume BasicAuth
		user, pass, ok := r.BasicAuth()
		if ok {
			ui.Debug(ui.AuthLogger, "Auth using user %s", user)

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
	loggers := defs.LoggingItem{}
	response := defs.LoggingResponse{
		ServerInfo: util.MakeServerInfo(sessionID),
	}

	user, hasAdminPrivileges := isAdminRequestor(r)
	if !hasAdminPrivileges {
		ui.Debug(ui.AuthLogger, "[%d] User %s not authorized", sessionID, user)
		util.ErrorResponse(w, sessionID, "Not authorized", http.StatusForbidden)

		return http.StatusForbidden
	}

	logHeaders(r, sessionID)

	switch r.Method {
	case http.MethodPost:
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)

		err := json.Unmarshal(buf.Bytes(), &loggers)
		if err != nil {
			util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
			ui.Debug(ui.ServerLogger, "[%d] Bad payload: %v", sessionID, err)

			return http.StatusBadRequest
		}

		for loggerName, mode := range loggers.Loggers {
			logger := ui.Logger(loggerName)
			if logger < 0 || (logger == ui.ServerLogger && !mode) {
				util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
				ui.Debug(ui.ServerLogger, "[%d] Bad logger name: %s", sessionID, loggerName)

				return http.StatusBadRequest
			}

			modeString := "enable"
			if !mode {
				modeString = "disable"
			}

			ui.Debug(ui.ServerLogger, "[%d] %s %s(%d) logger", sessionID, modeString, loggerName, logger)
			ui.SetLogger(logger, mode)
		}

		fallthrough

	case http.MethodGet:
		response.Filename = ui.CurrentLogFile()
		response.Loggers = map[string]bool{}
		response.RetainCount = ui.LogRetainCount
		response.ServerInfo = util.MakeServerInfo(sessionID)

		for _, k := range ui.LoggerNames() {
			response.Loggers[k] = ui.LoggerIsActive(ui.Logger(k))
		}

		w.Header().Add(contentTypeHeader, defs.LogStatusMediaType)

		b, _ := json.Marshal(response)
		_, _ = w.Write(b)

		return http.StatusOK

	case http.MethodDelete:
		if err := util.ValidateParameters(r.URL, map[string]string{"keep": "int"}); !errors.Nil(err) {
			util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)

			return http.StatusBadRequest
		}

		keep := ui.LogRetainCount
		q := r.URL.Query()

		if v, found := q["keep"]; found {
			if len(v) == 1 {
				keep, _ = strconv.Atoi(v[0])
			}
		}

		if keep < 1 {
			keep = 1
		}

		ui.LogRetainCount = keep
		count := ui.PurgeLogs()

		reply := defs.DBRowCount{
			ServerInfo: util.MakeServerInfo(sessionID),
			Count:      count}

		w.Header().Add(contentTypeHeader, defs.RowCountMediaType)

		b, _ := json.Marshal(reply)
		_, _ = w.Write(b)

		return http.StatusOK

	default:
		ui.Debug(ui.ServerLogger, "[%d] 405 Unsupported method %s", sessionID, r.Method)
		util.ErrorResponse(w, sessionID, "Method not allowed", http.StatusMethodNotAllowed)

		return http.StatusMethodNotAllowed
	}
}

func logHeaders(r *http.Request, sessionID int32) {
	if ui.LoggerIsActive(ui.InfoLogger) {
		for headerName, headerValues := range r.Header {
			if strings.EqualFold(headerName, "Authorization") {
				continue
			}

			ui.Debug(ui.InfoLogger, "[%d] header: %s %v", sessionID, headerName, headerValues)
		}
	}
}

// Debugging tool that dumps interesting things about a request. Only outputs
// when DEBUG logging is enabled.
func logRequest(r *http.Request, sessionID int32) {
	if ui.LoggerIsActive(ui.RestLogger) {
		ui.Debug(ui.RestLogger, "[%d] *** START NEW REQUEST ***", sessionID)
		ui.Debug(ui.RestLogger, "[%d] %s %s from %s (%d bytes of request content)", sessionID, r.Method, r.URL.Path, r.RemoteAddr, r.ContentLength)

		queryParameters := r.URL.Query()
		parmMsg := strings.Builder{}

		for k, v := range queryParameters {
			parmMsg.WriteString("  ")
			parmMsg.WriteString(k)
			parmMsg.WriteString(" is ")

			valueMsg := ""

			for n, value := range v {
				if n == 1 {
					valueMsg = "[" + valueMsg + ", "
				} else if n > 1 {
					valueMsg = valueMsg + ", "
				}

				valueMsg = valueMsg + value
			}

			if len(v) > 1 {
				valueMsg = valueMsg + "]"
			}

			parmMsg.WriteString(valueMsg)
		}

		if parmMsg.Len() > 0 {
			ui.Debug(ui.RestLogger, "[%d] Query parameters:\n%s", sessionID,
				util.SessionLog(sessionID, strings.TrimSuffix(parmMsg.String(), "\n")))
		}

		headerMsg := strings.Builder{}

		for k, v := range r.Header {
			for _, i := range v {
				// A bit of a hack, but if this is the Authorization header, only show
				// the first token in the value (Bearer, Basic, etc).
				if strings.EqualFold(k, "Authorization") {
					f := strings.Fields(i)
					if len(f) > 0 {
						i = f[0] + " <hidden value>"
					}
				}

				headerMsg.WriteString("   ")
				headerMsg.WriteString(k)
				headerMsg.WriteString(": ")
				headerMsg.WriteString(i)
				headerMsg.WriteString("\n")
			}
		}

		ui.Debug(ui.RestLogger, "[%d] Received headers:\n%s",
			sessionID,
			util.SessionLog(sessionID,
				strings.TrimSuffix(headerMsg.String(), "\n"),
			))
	}
}
