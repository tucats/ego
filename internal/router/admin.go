package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/tucats/ego/internal/builtins"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
	"github.com/tucats/ego/internal/language/tokens"
	"github.com/tucats/ego/internal/runtime/cipher"
	egoRuntimeUtility "github.com/tucats/ego/internal/runtime/util"
	auth "github.com/tucats/ego/internal/server/auth"
	"github.com/tucats/ego/internal/util"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// LogonHandler fields incoming logon requests to the /services/admin/logon endpoint.
// This endpoint is only used if the server library does not include an Ego service
// that performs this operation. The idea is that you can use this default, or you can
// add a service endpoint that overrides this to extend its functionality.
func LogonHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	ui.Log(ui.RouteLogger, "route.native.token", ui.A{
		"session": session.ID,
		"name":    session.User})

	// If the caller gave us a login source string, add it to the log. Currently, this
	// is only done by the /ui dashboard endpoint.
	if session.Source != "" {
		ui.Log(ui.ServerLogger, "server.login.source", ui.A{
			"session": session.ID,
			"source":  session.Source,
			"user":    session.User})
	}

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
	s.SetAlways(defs.SessionVariable, session.ID)

	// Call the builtin function cipher.New in the cipher package, using the symbol table
	// we just constructed. The function is passed the user name, and empty string for the
	// extra data, and an expiration time request. If it fails, bail out with an error.
	v, err := builtins.CallBuiltin(s, "cipher.New", session.User, "", session.Expiration)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusForbidden)
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

	// Unwrap the freshly-minted token to read its authoritative expiry and ID.
	// Using the token's own Expires field ensures the advisory value returned to
	// the client is always consistent with what the server will actually enforce,
	// regardless of any server-side duration setting changes (L3).
	t, err := tokens.Unwrap(response.Token, session.ID)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	response.ID = t.TokenID.String()
	response.Expiration = t.Expires.Format(time.UnixDate)
	response.Status = http.StatusOK

	// Stamp the user record with the time this token was issued. This is
	// best-effort — failure here does not affect the token already generated.
	if u, readErr := auth.AuthService.ReadUser(session.ID, session.User, true); readErr == nil {
		u.LastTokenAt = t.Created.Format(time.RFC3339)

		if writeErr := auth.AuthService.WriteUser(session.ID, u); writeErr == nil {
			_ = auth.AuthService.Flush()
		}
	}

	// Set the capability flags for this user.
	response.CanAdmin = session.Admin
	response.CanCode = session.Admin || util.InList(defs.CodeRunPermission, session.Permissions...)

	// Tell the dashboard how long it should wait for user activity before
	// signing out, so it doesn't need its own hard-coded default.
	response.InactivityTimeout = dashboardInactivityTimeout()

	// Convert the response to JSON and write it to the response and we're done.
	_ = util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		// Need to obscure the token value for logging purposes.
		response.Token = egostrings.TruncateMiddle(response.Token, 10)

		b, _ := json.MarshalIndent(response, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// dashboardInactivityTimeout returns the configured ego.server.dashboard.inactivity
// duration string, defaulting to "15m" (and persisting that default) when unset.
// Shared by LogonHandler here and issueToken in webauthn.go so both the
// password and passkey dashboard login paths report the same value.
func dashboardInactivityTimeout() string {
	timeout := settings.Get(defs.DashboardInactivityTimeoutSetting)
	if timeout == "" {
		timeout = "15m"
		settings.SetDefault(defs.DashboardInactivityTimeoutSetting, timeout)
	}

	return timeout
}

// DownHandler fields incoming requests to the /services/admin/down endpoint.
// This endpoint is only used if the runtime library does not include an Ego service
// that performs this operation. The idea is that you can use this default, or you can
// add a service endpoint that overrides this to extend its functionality.
//
// This function requests an orderly shutdown of the server via RequestShutdown,
// and then reports to the caller that the server is going down. The response
// status is purely informational to the caller; it is not used by the router to
// decide whether to shut down (see RequestShutdown for why).
func DownHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	gracePeriod := 1 * time.Second

	// See if there is a valid grace period on the request.
	if len(session.Parameters["grace"]) > 0 {
		text := session.Parameters["grace"][0]

		g, err := time.ParseDuration(text)
		if err != nil || g < 0 {
			m := errors.ErrInvalidDuration.Context(text).Error()

			return util.ErrorResponse(w, session.ID, m, http.StatusBadRequest)
		}

		gracePeriod = g
	}

	RequestShutdown(gracePeriod)

	return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.admin.server.stopped"), http.StatusServiceUnavailable)
}

// LogHandler is the native handler of the endpoint that retrieves log lines
// from a server. This handler will be invoked if no handler for this endpoint
// is found in the Ego services library.
func LogHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	var (
		err      error
		filter   int
		count    int
		classes  string
		message  string
		archive  bool
		since    time.Time
		until    time.Time
		serverID string
		status   = http.StatusOK
		lines    = []string{}
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

			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.admin.tail.invalid", ui.A{"value": v[0]}), http.StatusBadRequest)
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

			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.admin.session.invalid", ui.A{"value": v[0]}), http.StatusBadRequest)
		}
	}

	// Optional "class" filter: a comma-separated list of logger class names,
	// such as "REST,AUTH". Names are not case sensitive. Validation of the
	// individual names happens inside util.Log, which reports an unknown name
	// as an error rather than quietly matching nothing.
	if v, found := session.Parameters["class"]; found && len(v) > 0 {
		classes = strings.Join(v, ",")
	}

	// Optional "msg" filter: a glob pattern matched against the message
	// identifier, such as "rest.*". This matches the identifier recorded in the
	// log file, not the localized text the client eventually sees, so the same
	// pattern selects the same lines whatever language was requested.
	if v, found := session.Parameters["msg"]; found && len(v) > 0 {
		message = strings.TrimSpace(v[0])
	}

	// Optional "archive" flag: when true, the search extends past the active
	// log file into this instance's older rolled-over log files on disk, and
	// then into the configured zip archive if one exists, until the request
	// has enough lines or every available log has been consulted.
	if v, found := session.Parameters["archive"]; found && len(v) > 0 {
		archive, err = data.Bool(v[0])
		if err != nil {
			ui.Log(ui.RestLogger, "rest.error", ui.A{
				"session": session.ID,
				"status":  http.StatusBadRequest,
				"error":   err})

			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.admin.archive.invalid", ui.A{"value": v[0]}), http.StatusBadRequest)
		}
	}

	// Optional "since" and "until" bounds restrict results to entries whose
	// timestamp falls in that range. Each accepts RFC 3339 (with or without a
	// time-of-day component) or a plain "YYYY-MM-DD" date.
	if v, found := session.Parameters["since"]; found && len(v) > 0 {
		since, err = parseLogQueryTime(v[0])
		if err != nil {
			ui.Log(ui.RestLogger, "rest.error", ui.A{
				"session": session.ID,
				"status":  http.StatusBadRequest,
				"error":   err})

			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.admin.since.invalid", ui.A{"value": v[0]}), http.StatusBadRequest)
		}
	}

	if v, found := session.Parameters["until"]; found && len(v) > 0 {
		until, err = parseLogQueryTime(v[0])
		if err != nil {
			ui.Log(ui.RestLogger, "rest.error", ui.A{
				"session": session.ID,
				"status":  http.StatusBadRequest,
				"error":   err})

			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.admin.until.invalid", ui.A{"value": v[0]}), http.StatusBadRequest)
		}
	}

	// Optional "serverid" filter: a glob pattern matched against the writing
	// server's instance UUID, such as "da35*" or "*34fd". This only means
	// anything together with "archive" -- the active log file is written by a
	// single running process, so every one of its entries already shares the
	// same ID -- and util.Log's underlying LogFilter.Validate rejects the
	// combination of a serverid filter without archive, so no local check is
	// duplicated here.
	if v, found := session.Parameters["serverid"]; found && len(v) > 0 {
		serverID = strings.TrimSpace(v[0])
	}

	// If no count was given, assume we want the last 50 lines.
	if count <= 0 {
		count = 50
	}

	// since and until cross into the builtin call as Unix seconds (0 meaning
	// "unbounded") rather than as time.Time values, since builtin arguments
	// are the same primitive types an Ego script could pass to util.Log.
	var sinceUnix, untilUnix int64

	if !since.IsZero() {
		sinceUnix = since.Unix()
	}

	if !until.IsZero() {
		untilUnix = until.Unix()
	}

	// This service requires using the util.Log runtime function. Create a symbol
	// table and initialize the util package in that symbol table. Then call the
	// function, passing it the number of lines and the filter values.
	// If the function returns an error, formulate an error response to the caller.
	v, err := builtins.CallBuiltin(
		symbols.NewRootSymbolTable("log service").
			SetAlways("util", egoRuntimeUtility.UtilPackage).
			SetAlways(defs.SessionVariable, session.ID),
		"util.Log",
		count,
		filter,
		classes,
		message,
		archive,
		sinceUnix,
		untilUnix,
		serverID)

	if err != nil {
		// A rejected filter is the caller's mistake, not the server's: an unknown
		// logger class, a malformed pattern, or a structured filter asked of a
		// text-format log. Those get a 400 so the client can correct and retry,
		// while anything else (an unreadable log file, say) stays a 500.
		status := http.StatusInternalServerError
		if isFilterError(err) {
			status = http.StatusBadRequest
		}

		ui.Log(ui.RestLogger, "rest.error", ui.A{
			"session": session.ID,
			"status":  status,
			"error":   err})

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), status)
	}

	// The response should be an array of strings. Because util.Log follows the (value,error)
	// convention, first extract the array from the returned data.List. Convert this to a
	// native array of strings by appending each line to a []string array.
	if list, ok := v.(data.List); ok {
		v = list.Get(0)
	}

	if array, ok := v.(*data.Array); ok {
		for i := 0; i < array.Len(); i++ {
			v, _ := array.Get(i)
			lines = append(lines, data.String(v))
		}
	}

	// If the caller wants a JSON payload, form a JSON package that contains the
	// representation of the log lines along with the server information.
	if session.AcceptsJSON {
		// Note this is named "payload" rather than the shorter "r" used elsewhere,
		// because "r" is already the incoming *http.Request in this function and that
		// request is needed below to find out whether the client accepts compression.
		payload := defs.LogTextResponse{
			ServerInfo: util.MakeServerInfo(session.ID),
			Lines:      lines,
		}

		if b, err := json.MarshalIndent(payload, ui.JSONIndentPrefix, ui.JSONIndentSpacer); err == nil {
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

			minifiedBytes := []byte(egostrings.JSONMinify(string(b)))

			// Log payloads can be very large, so hand the body to the shared writer that
			// gzips it when the client advertised support for gzip and the payload is big
			// enough to be worth compressing. It also sets the Content-Type, the status,
			// and (when it compresses) the Content-Encoding and Vary headers for us.
			writeLogPayload(session, w, defs.LogLinesJSONMediaType, minifiedBytes)
		} else {
			ui.Log(ui.RestLogger, "rest.error", ui.A{
				"session": session.ID,
				"error":   err})

			return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
		}
	} else if session.AcceptsText {
		// The caller wants text, so the response payload is just raw text from the log.
		// The lines are assembled into a single buffer rather than written one at a time,
		// because the decision to compress depends on knowing the total payload size
		// before any of it is sent. This is safe for memory: the number of lines is
		// bounded by the "tail" parameter, which defaults to 50.
		var buffer bytes.Buffer

		for _, line := range lines {
			buffer.WriteString(ui.FormatJSONLogEntryAsText(line))
			buffer.WriteString("\n")
		}

		writeLogPayload(session, w, "text/plain", buffer.Bytes())
	} else {
		// Something other than JSON or TEXT requested; we don't know how to handle it.
		ui.Log(ui.RestLogger, "auth.bad.media", ui.A{
			"session": session.ID})

		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.media.unsupported"), http.StatusBadRequest)
	}

	return status
}

// isFilterError reports whether an error from the log retrieval call was caused
// by the filter the caller supplied, as opposed to something going wrong on the
// server. The former is a 400 -- the request itself was bad and no amount of
// retrying it unchanged will help -- while the latter stays a 500.
//
// Comparison is by the underlying error rather than by string, so wrapping the
// error with call context on the way up does not defeat the check.
func isFilterError(err error) bool {
	for _, filterError := range []error{
		errors.ErrInvalidLoggerName,
		errors.ErrInvalidLogPattern,
		errors.ErrLogFilterNeedsJSON,
		errors.ErrInvalidLogDateRange,
		errors.ErrInvalidLogServerIDPattern,
		errors.ErrLogServerIDNeedsArchive,
	} {
		if errors.Equals(err, filterError) {
			return true
		}
	}

	return false
}

// parseLogQueryTime parses the value of a "since" or "until" log query
// parameter. It accepts RFC 3339 (with or without a time-of-day component,
// e.g. "2026-08-12T10:15:00Z" or "2026-08-12T10:15:00") or a plain
// "2026-08-12" date, which is taken to mean local midnight at the start of
// that day.
//
// Anything else falls to dateparse.ParseIn, the same flexible parser behind
// the "ego" command line's --since/--until normalization (see
// normalizeLogQueryTime in internal/commands/logging.go). The Dashboard
// sends an RFC 3339 value whenever its own browser-side Date parsing can
// make sense of what the user typed, but falls back to sending the raw text
// otherwise rather than refusing it outright -- this is what makes that raw
// text usable server-side too, instead of just erroring.
func parseLogQueryTime(value string) (time.Time, error) {
	for _, format := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(format, value, time.Local); err == nil {
			return t, nil
		}
	}

	if t, err := dateparse.ParseIn(value, time.Local); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid time value %q", value)
}

// writeLogPayload sends a completed log response body to the client, compressing it
// with gzip when the client said it can accept gzip and the payload is large enough
// for compression to be worthwhile. Both the JSON and the plain-text forms of the log
// response go through here so the two share identical compression behavior.
//
// Server log responses are the largest payloads this server routinely produces -- a
// few hundred log lines is easily hundreds of kilobytes of highly repetitive text --
// which is why this endpoint in particular is worth compressing.
//
// The session's ResponseLength is updated with the number of bytes actually sent, so
// the request line written to the server log reports the true size on the network --
// the compressed size when compression was applied. WriteMaybeCompressed reports the
// before-and-after sizes to the REST logger itself, so both numbers stay available when
// debugging without this function having to log anything of its own.
func writeLogPayload(session *Session, w http.ResponseWriter, contentType string, body []byte) {
	_, err := util.WriteMaybeCompressed(w, session.Response(), http.StatusOK, contentType, body)
	if err != nil {
		// A write error means the client went away mid-response. There is nothing left
		// to send it -- the status line has already gone out -- so just record the fact.
		ui.Log(ui.RestLogger, "rest.error", ui.A{
			"session": session.ID,
			"error":   err})
	}
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
	s.SetAlways(defs.SessionVariable, session.ID)

	// Call the function to extract the value. This returns a structure item if it
	// succeeds. However, if the token is damaged or not able to be decrypted, an error is
	// returned.
	v, err := builtins.CallBuiltin(s, "cipher.Extract", session.Token)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
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
	user, err := auth.AuthService.ReadUser(session.ID, reply.Name, false)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
	}

	// Add the user permissions array to the response object.
	reply.Permissions = user.Permissions

	// Convert the response object to JSON, and write it to the response object and we're done.
	b, err := json.MarshalIndent(reply, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	if err != nil {
		ui.Log(ui.AuthLogger, "auth.error", ui.A{
			"session": session.ID,
			"error":   err})

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
	}

	minifiedBytes := []byte(egostrings.JSONMinify(string(b)))
	_, _ = w.Write(minifiedBytes)
	session.ResponseLength += len(minifiedBytes)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return status
}
