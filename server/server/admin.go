package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/runtime/cipher"
	egoRuntimeUtility "github.com/tucats/ego/runtime/util"
	auth "github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// LogonHandler fields incoming logon requests to the /services/admin/logon endpoint.
// This endpoint is only used if the server library does not include an Ego service
// that performs this operation. The idea is that you can use this default, or you can
// add a service endpoint that overrides this to extend its functionality.
func LogonHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	ui.Log(ui.RouteLogger, "route.native.token", ui.A{
		"session": session.ID,
		"name":    session.User})

	// Is there another auth server we should refer this to? If so, redirect.
	if auth := settings.Get(defs.ServerAuthoritySetting); auth != "" {
		http.Redirect(w, r, auth+"/services/admin/logon", http.StatusMovedPermanently)

		return http.StatusMovedPermanently
	}

	// No redirect, so we'll be generating a token here. This involves calling an Ego
	// function, so we need a new symbol table to support that function call. Then,
	// initialize the cipher package in that symbol table, so the package functionality
	// is available.
	s := symbols.NewRootSymbolTable("logon service")
	s.SetAlways("cipher", cipher.CipherPackage)

	// Call the builtin function cipher.New in the cipher package, using the symbol table
	// we just constructed. The function is passed the user name, and empty string for the
	// extra data, and an expiration time request. If it fails, bail out with an error.
	v, err := builtins.CallBuiltin(s, "cipher.New", session.User, "", session.Expiration)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusForbidden)
	}

	// Construct a response object to hold the token and server info.
	response := defs.LogonResponse{
		Identity: session.User,
		RestStatusResponse: defs.RestStatusResponse{
			ServerInfo: util.MakeServerInfo(session.ID),
		},
	}

	w.Header().Add(defs.ContentTypeHeader, defs.LogonMediaType)

	// If the function result was a string value, then it contains the token. if not,
	// something went wrong with the function call and we should report that as an
	// internal error.
	if t, ok := v.(string); ok {
		response.Token = data.String(t)
	} else {
		msg := fmt.Sprintf("invalid internal token data type: %s", data.TypeOf(v).String())
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   msg})

		return util.ErrorResponse(w, session.ID, msg, http.StatusInternalServerError)
	}

	// A little clunky, but we want to return the expiration time in the response.
	// However, the underlying function was smart enough to ensure the duration
	// in the request (if any) didn't exceed the defined server duration. So we have
	// to replicate that logic again here, so we can return the actual expiration
	// associated with the token that was generated. Note that this expiration is
	// returned to the caller as a courtesy; it doesn't effect the token in any way
	// but lets the client (usually Ego running in CLI mode) to store the expiration
	// if it wishes so that later it can warn the suer that a token won't work due
	// to expiration before calling the authentication service.
	serverDurationString := settings.Get(defs.ServerTokenExpirationSetting)
	if serverDurationString == "" {
		serverDurationString = "15m"

		settings.SetDefault(defs.ServerTokenExpirationSetting, serverDurationString)
	}

	maxServerDuration, _ := util.ParseDuration(serverDurationString)
	duration := maxServerDuration

	if session.Expiration != "" {
		if requestedDuration, err := util.ParseDuration(session.Expiration); err == nil {
			if requestedDuration > maxServerDuration {
				requestedDuration = maxServerDuration

				ui.Log(ui.AuthLogger, "auth.token.max.expire", ui.A{
					"session":  session.ID,
					"duration": maxServerDuration})
			}

			duration = requestedDuration
		}
	}

	// Store the resulting expiration string and status in the response.
	response.Expiration = time.Now().Add(duration).Format(time.UnixDate)
	response.Status = http.StatusOK

	// Convert the response to JSON and write it to the response and we're done.
	b, _ := json.MarshalIndent(response, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// DownHandler fields incoming requests to the /services/admin/down endpoint.
// This endpoint is only used if the runtime library does not include an Ego service
// that performs this operation. The idea is that you can use this default, or you can
// add a service endpoint that overrides this to extend its functionality.
//
// This function does not actually stop the server, but by returning an HTTP status
// indicating the server is down, the router that called this handler will know that
// the server is to be stopped.
func DownHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	text := "Server stopped"
	session.ResponseLength = len(text)

	ui.Log(ui.RouteLogger, "route.native.down", ui.A{
		"session": session.ID})
	w.WriteHeader(http.StatusServiceUnavailable)

	_, _ = w.Write([]byte(text))
	session.ResponseLength += len(text)

	return http.StatusServiceUnavailable
}

// LogHandler is the native handler of the endpoint that retrieves log lines
// from a server. This handler will be invoked in no handler for this endpoint
// is found in the Ego services library.
func LogHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	var (
		err    error
		filter int
		count  int
		status = http.StatusOK
		lines  = []string{}
	)

	ui.Log(ui.RouteLogger, "route.native.log", ui.A{
		"session": session.ID})

	// If present, get the "tail" value that says how many lines of output we are
	// asked to retrieve. If not present, default to 50 lines. If the string value
	// is invalid, return an error response to the caller.
	if v, found := session.Parameters["tail"]; found && len(v) > 0 {
		count, err = egostrings.Atoi(v[0])
		if err != nil {
			ui.Log(ui.RestLogger, "rest.error", ui.A{
				"session": session.ID,
				"status":  http.StatusBadRequest,
				"error":   err})

			return util.ErrorResponse(w, session.ID, "Invalid tail integer value: "+v[0], http.StatusBadRequest)
		}
	}

	// See if we are filtering by a specific session ID. If not present, no filtering
	// occurs. If the session number is invalid, an error response is returned to the caller.
	if v, found := session.Parameters["session"]; found && len(v) > 0 {
		filter, err = egostrings.Atoi(v[0])
		if err != nil {
			ui.Log(ui.RestLogger, "rest.error", ui.A{
				"session": session.ID,
				"status":  http.StatusBadRequest,
				"error":   err})

			return util.ErrorResponse(w, session.ID, "Invalid session id value: "+v[0], http.StatusBadRequest)
		}
	}

	// If no count was given, assume we want the last 50 lines.
	if count <= 0 {
		count = 50
	}

	// This service requires using the util.Log runtime function. Create a symbol
	// table and initialize the util package in that symbol table.
	s := symbols.NewRootSymbolTable("log service")
	s.SetAlways("util", egoRuntimeUtility.UtilPackage)

	// Call the function, passing it the number of lines and the session filter
	// values. If the function returns an error, formulate an error response to
	// the caller.
	v, err := builtins.CallBuiltin(s, "util.Log", count, filter)
	if err != nil {
		ui.Log(ui.RestLogger, "rest.error", ui.A{
			"session": session.ID,
			"status":  http.StatusInternalServerError,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	// The response should be an array of strings. Convert this to a native array
	// of strings by appending each line to a []string array.
	if array, ok := v.(*data.Array); ok {
		for i := 0; i < array.Len(); i++ {
			v, _ := array.Get(i)
			lines = append(lines, data.String(v))
		}
	}

	// If the caller wants a JSON payload, form a JSON package that contains the
	// representation of the log lines along with the server information.
	if session.AcceptsJSON {
		r := defs.LogTextResponse{
			ServerInfo: util.MakeServerInfo(session.ID),
			Lines:      lines,
		}

		if b, err := json.MarshalIndent(r, ui.JSONIndentPrefix, ui.JSONIndentSpacer); err == nil {
			if ui.IsActive(ui.RestLogger) {
				if settings.GetBool(defs.ServerLogResponseSetting) {
					ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
						"session": session.ID,
						"body":    string(b)})
				} else {
					ui.Log(ui.RestLogger, "rest.server.log", ui.A{
						"session": session.ID,
						"type":    ui.LogFormat,
						"lines":   len(lines),
						"size":    len(b)})
				}
			}

			w.Header().Set("Content-Type", defs.LogLinesMediaType)
			w.WriteHeader(http.StatusOK)

			_, _ = w.Write(b)
			session.ResponseLength += len(b)
		} else {
			ui.Log(ui.RestLogger, "rest.error", ui.A{
				"session": session.ID,
				"error":   err})

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	} else if session.AcceptsText {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		// The caller wants text, so the response payload is just raw text from the log.
		for _, line := range lines {
			_, _ = w.Write([]byte(line + "\n"))
			session.ResponseLength += len(line) + 1
		}
	} else {
		// Something other than JSON or TEXT requested; we don't know how to handle it.
		ui.Log(ui.RestLogger, "auth.bad.media", ui.A{
			"session": session.ID})

		return util.ErrorResponse(w, session.ID, "unsupported media type", http.StatusBadRequest)
	}

	return status
}

// AuthenticateHandler is the native endpoint for the /services/admin/authenticate
// endpoint, which returns information about the token used to access it.
func AuthenticateHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	status := http.StatusOK

	if session.Token == "" {
		msg := "unable to use endpoint without token authentication"

		ui.Log(ui.RestLogger, "rest.auth.token", ui.A{
			"session": session.ID,
			"path":    session.Path})

		return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
	}

	// This is done by calling the internal runtime cipher.Extract() function, so create
	// a symbol table for the call and initialize the cipher package in that symbol table.
	s := symbols.NewRootSymbolTable("authenticate service")
	s.SetAlways("cipher", cipher.CipherPackage)

	// Call the function to extract the value. This returns a structure item if it
	// succeeds. However, if the token is damaged or not able to be decrypted, an error is
	// returned.
	v, err := builtins.CallBuiltin(s, "cipher.Extract", session.Token)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Create an instance of the response object and fill the server info.
	reply := defs.AuthenticateResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
	}

	// Assuming the response was a struct value, retrieve each field from the
	// associated structure and store the value in the response object.
	if m, ok := v.(*data.Struct); ok {
		if v, found := m.Get("AuthID"); found {
			reply.AuthID = data.String(v)
		}

		if v, found := m.Get("Data"); found {
			reply.Data = data.String(v)
		}

		if v, found := m.Get("Expires"); found {
			reply.Expires = data.String(v)
		}

		if v, found := m.Get("Name"); found {
			reply.Name = data.String(v)
		}

		if v, found := m.Get("TokenID"); found {
			reply.TokenID = data.String(v)
		}
	}

	// Access the user information for the associated user name. We will use this to add
	// additional permissions information for the requested user to the response object.
	// If this operation fails, return an error response to the caller.
	user, err := auth.AuthService.ReadUser(reply.Name, false)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// Add the user permissions array to the response object.
	reply.Permissions = user.Permissions

	// Convert the response object to JSON, and write it to the response object and we're done.
	b, err := json.MarshalIndent(reply, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return status
}
