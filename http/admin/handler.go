// Package admin handles the server side administrative functions
// of an Ego server.
package admin

import (
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	auth "github.com/tucats/ego/http/auth"
	"github.com/tucats/ego/http/server"
)

const (
	successMessage     = "Success"
	forwardedForHeader = "X-Forwarded-For"
	contentTypeHeader  = "Content-Type"
)

// UserHandler is the rest handler for /admin/user endpoint
// operations.
func UserHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := atomic.AddInt32(&server.NextSessionID, 1)
	requestor := r.RemoteAddr

	w.Header().Add("X-Ego-Server", defs.ServerInstanceID)
	server.LogRequest(r, sessionID)
	server.CountRequest(server.AdminRequestCounter)

	if forward := r.Header.Get(forwardedForHeader); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	// If INFO logging, put out the prologue message for the operation.
	ui.Log(ui.RestLogger, "[%d] %s %s; from %s", sessionID, r.Method, r.URL.Path, requestor)
	ui.Log(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	// Do the actual work.
	status := userAction(sessionID, w, r)

	ui.Log(ui.ServerLogger, "[%d] %s %s; from %s; status %d; content: json", sessionID, r.Method, r.URL.Path, requestor, status)
}

func CachesHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := atomic.AddInt32(&server.NextSessionID, 1)
	requestor := r.RemoteAddr

	w.Header().Add("X-Ego-Server", defs.ServerInstanceID)
	server.LogRequest(r, sessionID)
	server.CountRequest(server.AdminRequestCounter)

	if forward := r.Header.Get(forwardedForHeader); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	ui.Log(ui.RestLogger, "[%d] %s %s; from %s", sessionID, r.Method, r.URL.Path, requestor)
	ui.Log(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	status := cachesAction(sessionID, w, r)

	ui.Log(ui.ServerLogger, "[%d] %s %s; from %s; status %d; content: json", sessionID, r.Method, r.URL.Path, requestor, status)
}

func LoggingHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := atomic.AddInt32(&server.NextSessionID, 1)
	requestor := r.RemoteAddr

	w.Header().Add("X-Ego-Server", defs.ServerInstanceID)
	server.LogRequest(r, sessionID)
	server.CountRequest(server.AdminRequestCounter)

	if forward := r.Header.Get(forwardedForHeader); forward != "" {
		addrs := strings.Split(forward, ",")
		requestor = addrs[0]
	}

	ui.Log(ui.RestLogger, "[%d] %s %s; from %s", sessionID, r.Method, r.URL.Path, requestor)
	ui.Log(ui.RestLogger, "[%d] User agent: %s", sessionID, r.Header.Get("User-Agent"))

	status := loggingAction(sessionID, w, r)

	ui.Log(ui.ServerLogger, "[%d] %s %s; from %s; status %d; content: json", sessionID, r.Method, r.URL.Path, requestor, status)
}

// For a given userid, indicate if this user exists and has admin privileges.
func isAdminRequestor(r *http.Request) (string, bool) {
	var user string

	hasAdminPrivileges := false

	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		ui.Log(ui.AuthLogger, "No authentication credentials given")

		return "<invalid>", false
	}

	// IF the authorization header has the auth scheme prefix, extract and
	// validate the token
	if strings.HasPrefix(strings.ToLower(authorization), defs.AuthScheme) {
		token := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(authorization), defs.AuthScheme))

		tokenString := token
		if len(tokenString) > 10 {
			tokenString = tokenString[:10] + "..."
		}

		ui.Log(ui.AuthLogger, "Auth using token %s...", tokenString)

		if auth.ValidateToken(token) {
			user := auth.TokenUser(token)
			if user == "" {
				ui.Log(ui.AuthLogger, "No username associated with token")
			}

			hasAdminPrivileges = auth.GetPermission(user, "root")
		} else {
			ui.Log(ui.AuthLogger, "No valid token presented")
		}
	} else {
		// Not a token, so assume BasicAuth
		user, pass, ok := r.BasicAuth()
		if ok {
			ui.Log(ui.AuthLogger, "Auth using user %s", user)

			if ok := auth.ValidatePassword(user, pass); ok {
				hasAdminPrivileges = auth.GetPermission(user, "root")
			}
		}
	}

	if !hasAdminPrivileges && user == "" {
		user = "<invalid>"
	}

	return user, hasAdminPrivileges
}
