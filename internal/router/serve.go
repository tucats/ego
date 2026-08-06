package router

import (
	"bytes"
	nativeErrors "errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/server/auth"
	"github.com/tucats/ego/internal/util"
	"github.com/tucats/ego/internal/util/validate"
)

// defaultMaxBodyBytes is the upper bound on request body size when
// ego.server.max.body.size is not configured. 32 MiB is generous enough
// for all expected payloads while preventing memory-exhaustion via large bodies.
const defaultMaxBodyBytes = 32 << 20 // 32 MiB

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

var ServerShutdownLock sync.Mutex

// reportRequestPanic is the server's last-resort panic handler. It is called
// from a deferred function at the top of ServeHTTP while the stack of a
// panicking request is being unwound, and receives the value that recover()
// returned.
//
// IMPORTANT, and easy to get wrong: this function does NOT call recover()
// itself. Go only lets recover() stop a panic when it is called *directly* by
// the deferred function -- one extra layer of function call and it returns nil
// and the panic keeps going. So the recover() lives in the deferred closure in
// ServeHTTP, and the value it produced is passed down to here.
//
// NILPTR-1: Some background on why this exists, for anyone new to Go.
//
// A panic (from a nil-pointer dereference, an out-of-range index, a failed type
// assertion, ...) unwinds the current goroutine. Go's net/http package puts its
// own recover() at the connection level, so a panicking handler does NOT kill
// the server process -- that much was already safe. What was missing:
//
//   - The client got no response at all. net/http just closes the connection,
//     so the caller sees a broken connection rather than an HTTP 500, and has
//     no way to tell a crash apart from a network fault.
//   - Nothing was logged by us, so the failure left no trace in the server log
//     that an operator would find.
//
// Recovering here supplies both: a real 500 response, and a log entry with the
// panic value and its stack.
//
// A related but separate hazard is a lock that is never released. ServeHTTP
// takes ServerShutdownLock on entry and drops it right after routing, so a
// panic between those two points would strand the mutex and every later request
// would block on it forever -- the server would still be "running" but unable
// to answer anything. That window is narrow (it covers FindRoute, not the
// handler, because the lock is released before the handler is called), and the
// fix for it is not this recover() but the deferred Unlock in ServeHTTP: Go runs
// deferred calls even while a panic unwinds, so the mutex comes back either way.
// See TestFindRoutePanicDoesNotLeakShutdownLock_NILPTR1.
//
// Set ego.server.panic.recovery to false to disable this and let panics
// propagate unmodified, which is usually what you want while debugging. The
// deferred Unlock protection stays in force regardless of that setting.
func reportRequestPanic(w http.ResponseWriter, r *http.Request, sessionID int, panicValue any) {
	// Honor the configuration switch. Re-panicking here keeps the panic moving
	// up the stack exactly as if we had never intervened, so a developer sees
	// the original failure and its original stack.
	if !util.PanicRecoveryEnabled() {
		panic(panicValue)
	}

	// Capture the stack of the panicking goroutine. debug.Stack must be called
	// here, during unwinding, because by the time this function returns the
	// frames that panicked are gone.
	stack := string(debug.Stack())

	ui.Log(ui.ServerLogger, "server.panic.recovered", ui.A{
		"session": sessionID,
		"method":  r.Method,
		"path":    r.URL.Path,
		"error":   fmt.Sprintf("%v", panicValue),
	})

	// The stack trace is logged separately, and only to the internal log, to
	// keep the single-line server log readable while still preserving the
	// diagnostic detail somewhere.
	ui.Log(ui.InternalLogger, "server.panic.stack", ui.A{
		"session": sessionID,
		"stack":   stack,
	})

	// Send a generic 500. The panic text is deliberately NOT included in the
	// response: it routinely contains internal type and file names, which we do
	// not want to hand to a caller who may have caused the panic on purpose.
	//
	// If the handler already began writing a response, WriteHeader here will be
	// ignored by net/http (it logs "superfluous WriteHeader"); there is nothing
	// better we can do at that point, since the status line is already sent.
	util.ErrorResponse(w, sessionID, i18n.Text(negotiateLanguage(r), "error.server.error"), http.StatusInternalServerError)
}

// ServeHTTP satisfies the requirements of an HTTP multiplexer to
// the Go "http" package. This accepts a request and response writer,
// and determines which path to direct the request to.
//
// This function also handles creating the *Session object passed to
// the handler, and basic logging.
func (m *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var session *Session

	// sessionID is declared before the deferred panic handler so that the
	// handler can report it. It is filled in below once we know whether this
	// request counts as a session.
	sessionID := 0

	// Install the last-resort panic handler FIRST, before anything that could
	// panic or take a lock. Deferred calls run in last-in-first-out order, so
	// registering this first means it runs last -- after the deferred Unlock
	// calls below have already released their locks.
	//
	// The recover() call has to be right here, in the deferred function itself.
	// Go only honors recover() when it is invoked directly by the deferred
	// function; calling it from a helper that the deferred function invokes
	// returns nil and the panic continues unchecked. So this closure does the
	// recovering and reportRequestPanic does the reporting.
	//
	// sessionID is read when this closure RUNS, not when it is declared, so the
	// value assigned further down is the one reported.
	defer func() {
		if panicValue := recover(); panicValue != nil {
			reportRequestPanic(w, r, sessionID, panicValue)
		}
	}()

	// Make sure we aren't blocked on shutdown.
	//
	// NILPTR-1: the release below is a deferred Unlock rather than a plain
	// statement, so that a panic inside FindRoute -- the only code this lock is
	// held across -- cannot leave the mutex locked forever. Deferred calls run
	// during panic unwinding, a plain statement does not, and a stranded
	// ServerShutdownLock would block every subsequent request permanently.
	//
	// shutdownLockHeld makes the deferred release idempotent, because on the
	// normal path we still want to drop the lock as early as possible rather than
	// hold it for the life of the request.
	shutdownLockHeld := true

	ServerShutdownLock.Lock()

	defer func() {
		if shutdownLockHeld {
			ServerShutdownLock.Unlock()
		}
	}()

	// Record when this particular request began, and find the matching
	// route for this request.
	start := time.Now()
	route, status := m.FindRoute(r.Method, r.URL.Path, true)
	defer route.Unlock()

	// If we've gotten this far, not blocked for shutdown.
	shutdownLockHeld = false
	ServerShutdownLock.Unlock()

	// Now that we (potentially) have a route, increment the session count
	// if this is not a "lightweight" request type. Note that a failed route
	// connection always counts as a session attempt and increments the
	// sequence number. sessionID was declared at the top of the function so the
	// deferred panic handler can report it.
	if route == nil || !route.lightweight {
		sessionID = int(atomic.AddInt32(&SequenceNumber, 1))
	}

	// Set security headers on every response.
	addSecurityHeaders(w, r)

	// Problem with the path? Log it based on whether the method was not found or
	// unsupported.
	if status != http.StatusOK {
		msg := "invalid URL"

		// clientMsg is the generic message returned to the caller.
		// The raw path is kept off the wire to avoid reflecting attacker-controlled
		// strings and to limit reconnaissance; it is still captured in the log below.
		clientMsg := "invalid URL"
		servedHTML := false

		switch status {
		case http.StatusMethodNotAllowed:
			msg = "method " + r.Method + " not allowed"
			clientMsg = msg // method name is safe to echo; it's our own validated string

		case http.StatusForbidden:
			msg = "forbidden access to " + r.URL.Path
			clientMsg = i18n.Text(negotiateLanguage(r), "error.route.forbidden")

		case http.StatusNotFound:
			msg = "endpoint " + r.URL.Path + " not found"
			clientMsg = i18n.Text(negotiateLanguage(r), "error.route.not.found")

			// When the request originates from a web browser, serve a helpful HTML
			// page instead of the machine-readable JSON error body.
			if requestWantsBrowserHTML(r) {
				serveNotFoundPage(w, r)

				servedHTML = true
			}
		}

		if !servedHTML {
			util.ErrorResponse(w, sessionID, clientMsg, status)
		}

		ui.Log(ui.ServerLogger, "server.route.error", ui.A{
			"session": sessionID,
			"status":  status,
			"message": msg,
			"method":  r.Method,
			"path":    r.URL.Path,
			"remote":  r.RemoteAddr,
		})

		return
	}

	// If we found a route, make a session object. Set the media type
	// flags for Text or JSON data, the URL parts map, and the parameter
	// map in the session, so this info doesn't need to have complex parsing
	// in the individual handlers.
	if route != nil {
		text := false
		json := false

		if acceptTypes := r.Header["Accept"]; len(acceptTypes) > 0 {
			for _, acceptType := range acceptTypes {
				if strings.Contains(acceptType, "*/*") {
					text = true
					json = true

					break
				}

				if strings.Contains(strings.ToLower(acceptType), "text") {
					text = true
				}

				if strings.Contains(strings.ToLower(acceptType), "json") {
					json = true
				}
			}
		}

		session = &Session{
			Route:               route,
			URLParts:            route.partsMap(r.URL.Path),
			Parameters:          route.parmMap(r),
			Path:                route.endpoint,
			URL:                 r.URL,
			handler:             route.handler,
			ID:                  sessionID,
			Instance:            route.router.name,
			Filename:            route.filename,
			AcceptsJSON:         json,
			AcceptsText:         text,
			AcceptsGzip:         util.AcceptsGzip(r),
			Language:            negotiateLanguage(r),
			Redirect:            route.redirect,
			Validations:         route.Validations(),
			Router:              m,
			ValidateCredentials: route.checkCredentials,
		}
	}

	// If this route has a service class associated with it for auditing service
	// stats, then count it.
	if route != nil && route.auditClass > NotCounted {
		CountRequest(route.auditClass)
	}

	if route != nil && !route.lightweight {
		// Log the detailed information on the request, before any conditions that might
		// set the result status.
		LogRequest(r, session.ID)

		// Process any authentication info in the request, and add it to the session.
		session.Authenticate(r)

		// If the account is rate-limited due to too many failed login attempts,
		// return 429 with a Retry-After header immediately, before any other check.
		if session.LockedOut {
			w.Header().Set("Retry-After", strconv.Itoa(session.RetryAfter))
			util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.auth.rate.limited"), http.StatusTooManyRequests)

			return
		}

		if !session.Authenticated && route.mustAuthenticate {
			ui.Log(ui.ServerLogger, "server.auth.failed", ui.A{
				"session": sessionID,
				"remote":  r.RemoteAddr,
				"path":    r.URL.Path,
			})

			status = http.StatusForbidden

			if route.canAuthenticate {
				realmHeader := fmt.Sprintf(`Basic realm="%s"`, Realm)
				status = http.StatusForbidden

				w.Header().Set("WWW-Authenticate", realmHeader)
			}

			util.ErrorResponse(w, session.ID, errors.ErrInvalidCredentials.Localize(session.Language), status)

			return
		}
		// A route with no handler function cannot be dispatched. Report it as an
		// internal error instead of falling through to the call site below.
		//
		// NILPTR-2: this check used to live inside the "if ui.IsActive(ui.RestLogger)"
		// block that follows, so it only ran when REST logging happened to be
		// switched on. With logging off -- the normal production configuration --
		// a nil handler reached "session.handler(session, w, r)" further down and
		// panicked on a call through a nil function value. A safety check must
		// never be conditional on a diagnostic setting being enabled, so it is
		// hoisted out here where it always runs.
		//
		// The panic-recovery handler added for NILPTR-1 would now turn that into
		// a 500 rather than a dropped connection, but reporting the specific
		// problem here is much more useful than a generic recovered panic.
		if route.handler == nil {
			ui.Log(ui.InternalLogger, "route.handler.nil", ui.A{
				"route": fmt.Sprintf("%#v", route)})

			// The route dump is deliberately kept out of the client response; it
			// describes server-side routing internals.
			util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.server.error"), http.StatusInternalServerError)

			return
		}

		// Log which route we're using. This is helpful for debugging service route
		// declaration errors.
		if ui.IsActive(ui.RestLogger) {
			// Get the real name of the handler function, and clean it up by removing
			// noisy prefixes supplied by the reflection system.
			functionName := runtime.FuncForPC(reflect.ValueOf(route.handler).Pointer()).Name()
			functionName = data.StripGoPrefixes(functionName)

			if route.filename != "" {
				functionName = functionName + ", file " + strconv.Quote(route.filename)
			}

			ui.Log(ui.RestLogger, "route.handler", ui.A{
				"session":  sessionID,
				"endpoint": route.endpoint,
				"handler":  functionName})
		}
	}

	// Validate request media types required for this route, if any.
	if route != nil && route.acceptMediaTypes != nil {
		ui.Log(ui.RestLogger, "rest.media.check", ui.A{
			"session": sessionID,
			"media":   route.acceptMediaTypes})

		if err := util.AcceptedMediaType(r, route.acceptMediaTypes); err != nil {
			status = util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), http.StatusBadRequest)
		}
	}

	if route != nil && route.contentMediaTypes != nil {
		ui.Log(ui.RestLogger, "rest.media.check", ui.A{
			"session": sessionID,
			"media":   route.contentMediaTypes})

		if err := util.ContentMediaType(r, route.contentMediaTypes); err != nil {
			status = util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), http.StatusBadRequest)
		}
	}

	// Validate required permissions that must exist for this user. We skip this if the
	// user authenticated as an admin account. If any permissions are missing, we fail
	// with a Forbidden error.
	if status == http.StatusOK && (route.requiredPermissions != nil && !session.Admin) {
		allowed := true

		for _, permission := range route.requiredPermissions {
			if !auth.GetPermission(session.ID, session.User, permission) {
				allowed = false

				logger := ui.RouteLogger
				if !ui.IsActive(logger) {
					logger = ui.AuthLogger
				}

				ui.Log(logger, "route.perm.auth", ui.A{
					"session":    session.ID,
					"permission": permission,
					"user":       session.User,
				})

				sts := http.StatusForbidden
				if session.User == "" && route.canAuthenticate {
					sts = http.StatusUnauthorized

					w.Header().Add(defs.AuthenticateHeader,
						fmt.Sprintf(`Basic realm=%s, charset="UTF-8"`, strconv.Quote(Realm)))
				}

				// Stop at the first missing permission. Continuing to loop
				// would call util.ErrorResponse again for every subsequent
				// missing permission on a route that requires more than
				// one -- each call writes its own status line and JSON
				// body, and since a response can only be written to once,
				// that means a second (and third, ...) body gets appended
				// to the one already sent, corrupting it.
				status = util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.perm.privilege", ui.A{"permission": permission}), sts)

				break
			}
		}

		if allowed {
			ui.Log(ui.AuthLogger, "route.perm.authorized", ui.A{
				"session":     session.ID,
				"user":        session.User,
				"permissions": route.requiredPermissions,
			})
		}
	}

	// While we're here, copy the permissions list to the session for future use.
	// Don't do this if we already got the permissions during authentication.
	if session.User != "" && len(session.Permissions) == 0 {
		session.Permissions = auth.GetPermissions(session.ID, session.User)
	}

	// If the route has a redirect, redirect the user to the new location.
	if status == http.StatusOK && route.redirect != "" {
		ui.Log(ui.ServerLogger, "server.redirected", ui.A{
			"session": session.ID,
			"oldpath": route.endpoint,
			"newpath": route.redirect,
		})

		http.Redirect(w, r, route.redirect, http.StatusTemporaryRedirect)

		return
	}

	// Validate that the parameters provided are all permitted and of the correct form.
	if status == http.StatusOK {
		if err := route.Disallowed(session); err != nil {
			status = util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
		} else if err := util.ValidateParameters(r.URL, route.parameters); err != nil {
			status = util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
		}
	}

	// Validate and populate paging parameters (start, limit) for routes that declare them.
	if status == http.StatusOK {
		status = validatePaging(session, w)
	}

	// Validate that the user is authenticated if required by the route.
	if status == http.StatusOK {
		if route.mustAuthenticate && !session.Authenticated && route.canAuthenticate {
			w.Header().Set(defs.AuthenticateHeader, `Basic realm=`+strconv.Quote(Realm)+`, charset="UTF-8"`)
			ui.Log(ui.RouteLogger, "route.cred", ui.A{
				"session": session.ID,
			})

			util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.auth.unauthenticated"), http.StatusUnauthorized)

			return
		} else if route.mustBeAdmin && !session.Admin {
			ui.Log(ui.RouteLogger, "route.admin", ui.A{
				"session": session.ID,
			})

			util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.auth.forbidden"), http.StatusForbidden)

			return
		}
	}

	// Move the request body to the session object, enforcing a size cap to
	// prevent memory exhaustion from arbitrarily large payloads.
	if r.Body != nil {
		maxBytes := int64(defaultMaxBodyBytes)
		if v := settings.GetInt(defs.ServerMaxBodySizeSetting); v > 0 {
			maxBytes = int64(v)
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

		var readErr error

		session.Body, readErr = io.ReadAll(r.Body)
		r.Body.Close()

		if readErr != nil {
			var maxBytesErr *http.MaxBytesError
			if nativeErrors.As(readErr, &maxBytesErr) {
				util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.request.too.large"), http.StatusRequestEntityTooLarge)

				return
			}
		}

		// Reset the body reader so handlers that need to re-read it can do so.
		r.Body = nopCloser{bytes.NewReader(session.Body)}
	}

	// If we have validation objects for this route, let's check them out.
	if status == http.StatusOK && len(session.Validations) > 0 && session.Body != nil {
		var last error

		status = http.StatusBadRequest

		for _, validation := range route.validations {
			err := validate.Validate(session.Body, validation)
			if err == nil {
				status = http.StatusOK

				break
			} else {
				last = err
			}
		}

		// If we got an error from a validation, send it back to the client. If not,
		// it means all validations passed, so we can move on to the next step.

		if status != http.StatusOK {
			status = util.ErrorResponse(w, session.ID, errors.Localize(last, session.Language), status)
		}
	}

	// Call the designated route handler. This is where the actual work of the request will be done.
	//
	// NILPTR-2: the nil-handler check above runs only for non-lightweight routes,
	// because it lives inside the "!route.lightweight" block that does the
	// authentication and logging work. A lightweight route with no handler would
	// still reach this line, and calling a nil function value panics -- so the
	// call site itself is guarded here as well. In Go a func-typed field defaults
	// to nil, so an incompletely built Route has exactly this shape.
	if status == http.StatusOK {
		if session.handler == nil {
			ui.Log(ui.InternalLogger, "route.handler.nil", ui.A{
				"route": fmt.Sprintf("%#v", route)})

			status = util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.server.error"), http.StatusInternalServerError)
		} else {
			status = session.handler(session, w, r)
		}
	}

	// If it wasn't a lightweight call, log information about the request.
	if !route.lightweight {
		LogResponse(w, session.ID)

		// Prepare an end-of-request message for the SERVER logger.
		contentType := w.Header().Get(defs.ContentTypeHeader)
		if contentType == "" {
			w.Header().Set(defs.ContentTypeHeader, "text")

			contentType = "text"
		}

		size := strconv.Itoa(session.ResponseLength)
		elapsed := time.Since(start).String()

		ui.Log(ui.ServerLogger, "server.request", ui.A{
			"session": session.ID,
			"status":  status,
			"method":  r.Method,
			"path":    r.URL.Path,
			"host":    r.RemoteAddr,
			"user":    session.User,
			"type":    contentType,
			"length":  size,
			"elapsed": elapsed})

		// If the result status was indicating that the service is unavailable, let's start
		// a shutdown to make this a true statement. We always sleep for one second to allow
		// the response to clear back to the caller.
		if status == http.StatusServiceUnavailable && session.Admin {
			ServerShutdownLock.Lock()
			go func() {
				time.Sleep(1 * time.Second)
				ui.Log(ui.ServerLogger, "server.shutdown", nil)
				os.Exit(0)
			}()
		}
	}
}

// addSecurityHeaders sets defensive HTTP response headers on every reply.
// The transport-security header is only emitted on TLS connections to avoid breaking plain-HTTP deployments.
func addSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'self'")

	if r.TLS != nil {
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}

	// We do not allow any part of the dashboard to be in a frame, to prevent hijacking.
	if true || strings.Contains(r.URL.Path, "assets/dashboard/") {
		h.Set("X-Frame-Options", "DENY")
	}
}

// defaultMaxItemLimit is the fallback ceiling for paged collection responses when
// ego.server.max.item.limit is not configured.
const defaultMaxItemLimit = 1000

// validatePaging reads the "start" and "limit" query parameters from the session
// for routes that declare them, validates their values, and stores the results in
// session.Start and session.Limit. It returns a non-OK HTTP status and writes an
// error response if any value fails validation.
func validatePaging(session *Session, w http.ResponseWriter) int {
	if session.Route == nil || session.Route.parameters == nil {
		return http.StatusOK
	}

	_, hasStart := session.Route.parameters["start"]
	_, hasLimit := session.Route.parameters["limit"]

	if !hasStart && !hasLimit {
		return http.StatusOK
	}

	maxLimit := settings.GetInt(defs.ServerMaxItemLimitSetting)
	if maxLimit <= 0 {
		maxLimit = defaultMaxItemLimit
	}

	if hasStart {
		if vals := session.Parameters["start"]; len(vals) > 0 {
			n, err := strconv.Atoi(vals[0])
			if err != nil || n < 0 {
				return util.ErrorResponse(w, session.ID, i18n.ELang(session.Language, "paging.start.invalid"), http.StatusBadRequest)
			}

			session.Start = n
		}
	}

	if hasLimit {
		if vals := session.Parameters["limit"]; len(vals) > 0 {
			n, err := strconv.Atoi(vals[0])
			if err != nil || n <= 0 {
				return util.ErrorResponse(w, session.ID, i18n.ELang(session.Language, "paging.limit.invalid"), http.StatusBadRequest)
			}

			if n > maxLimit {
				return util.ErrorResponse(w, session.ID, i18n.ELang(session.Language, "paging.limit.exceeded"), http.StatusBadRequest)
			}

			session.Limit = n
		}
	}

	return http.StatusOK
}

// requestWantsBrowserHTML returns true when the request's Accept header indicates
// that the client prefers HTML — the hallmark of a browser navigation request.
// API clients (curl, programmatic HTTP, etc.) typically omit text/html entirely.
func requestWantsBrowserHTML(r *http.Request) bool {
	for _, headerVal := range r.Header["Accept"] {
		for _, token := range strings.Split(headerVal, ",") {
			// Strip quality parameters (e.g. "text/html;q=0.9") before comparing.
			mediaType := strings.TrimSpace(strings.SplitN(token, ";", 2)[0])
			if strings.EqualFold(mediaType, "text/html") {
				return true
			}
		}
	}

	return false
}

// negotiateLanguage figures out which language the response to this
// request should be written in, based on the standard HTTP
// "Accept-Language" request header. This is the same header a web browser
// sends to tell a website "show me this page in French if you can" — for
// example a header value of "fr-CA,fr;q=0.9,en;q=0.8" means the client
// would most prefer Canadian French, then any French, then English.
//
// i18n.NegotiateLanguage does the actual parsing of that header and
// matches it against the languages Ego actually has translations for. If
// the header is missing, empty, or names only languages Ego doesn't
// support, NegotiateLanguage returns "" — in that case we fall back to
// i18n.DefaultLanguage(), which is the same language Ego would use for a
// plain CLI invocation (normally English, unless overridden by the
// EGO_LANG/LANG environment variables or the --language command line
// option).
//
// The result of this function is stored once on the Session as
// Session.Language and does not change for the lifetime of the request,
// so handler code can read session.Language directly without needing to
// re-parse the header or worry about concurrent modification.
func negotiateLanguage(r *http.Request) string {
	if lang := i18n.NegotiateLanguage(r.Header.Get("Accept-Language")); lang != "" {
		return lang
	}

	return i18n.DefaultLanguage()
}

// serveNotFoundPage substitutes the requested path into the notfound.html asset
// and writes it as a 404 HTML response. If the asset file cannot be read, a
// minimal inline HTML page is used as a fallback.
func serveNotFoundPage(w http.ResponseWriter, r *http.Request) {
	page, err := readNotFoundAsset()
	if err != nil {
		page = []byte(notFoundFallbackHTML)
	}

	page = bytes.ReplaceAll(page, []byte("__PATH__"), []byte(html.EscapeString(r.URL.Path)))

	w.Header().Set(defs.ContentTypeHeader, "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(page)
}

// readNotFoundAsset resolves the lib directory the same way the asset handler
// does, reads lib/assets/notfound.html, and returns its contents. It does not
// use the server/assets package to avoid a circular import.
func readNotFoundAsset() ([]byte, error) {
	root := settings.Get(defs.EgoLibPathSetting)
	if root == "" {
		root = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	fn := filepath.Clean(filepath.Join(root, "assets", "notfound.html"))

	// Confinement check: reject paths that escape the lib root.
	if !strings.HasPrefix(fn, root+string(filepath.Separator)) {
		return nil, errors.New(errors.ErrInvalidSandboxPath)
	}

	return os.ReadFile(fn)
}

// notFoundFallbackHTML is used when the notfound.html asset file cannot be read.
const notFoundFallbackHTML = `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><title>404 Not Found</title>
<style>body{font-family:sans-serif;text-align:center;padding:4rem;background:#f0f4ff}
h1{font-size:4rem;color:#dbeafe;margin-bottom:.5rem}p{color:#6b7280}
a{color:#2563eb}</style></head>
<body><h1>404</h1><p>The address <code>__PATH__</code> was not found.</p>
<p><a href="/ui">Go to Dashboard</a> &nbsp;·&nbsp; <a href="javascript:history.back()">Go Back</a></p>
</body></html>`

// Given a request, build a map of the parameters in the URL. The primary
// key of the parameter map is the parameter name, and the value is a slice of strings
// representing the values for that parameter. If there is only a single value for
// the parameter, the map is a slice with a single entry. Otherwise, if the parameter
// appears multiple times in the URL, each instance is an entry in the slice.
func (r *Route) parmMap(req *http.Request) map[string][]string {
	result := map[string][]string{}

	parms := req.URL.Query()

	for parm, list := range parms {
		result[parm] = list
	}

	return result
}

// Given a path string from the user's request, use the route
// pattern information to create a map describing each field
// in the URL. If there is no pattern, this returns a nil map.
func (r *Route) partsMap(path string) map[string]any {
	m := map[string]any{}
	path = strings.TrimPrefix(strings.TrimSuffix(path, "/"), "/")
	segments := strings.Split(path, "?")
	pathSegment := strings.TrimPrefix(strings.TrimSuffix(segments[0], "/"), "/")
	pathParts := strings.Split(pathSegment, "/")
	patternParts := strings.Split(strings.TrimPrefix(strings.TrimSuffix(r.endpoint, "/"), "/"), "/")

	for index, part := range patternParts {
		// A glob variable ({{name...}}) captures all remaining path segments
		// as a single slash-joined string.
		if strings.HasPrefix(part, "{{") && strings.HasSuffix(part, "...}}") {
			key := strings.TrimSuffix(strings.TrimPrefix(part, "{{"), "...}}")

			if index < len(pathParts) {
				m[key] = strings.Join(pathParts[index:], "/")
			} else {
				m[key] = ""
			}

			break
		}

		// A normal variable ({{name}}) captures exactly one path segment.
		if strings.HasPrefix(part, "{{") && strings.HasSuffix(part, "}}") {
			key := strings.TrimPrefix(strings.TrimSuffix(part, "}}"), "{{")

			if index < len(pathParts) {
				m[key] = pathParts[index]
			} else {
				m[key] = ""
			}
		} else {
			if index >= len(pathParts) {
				m[part] = false
			} else {
				m[part] = (part == pathParts[index])
			}
		}
	}

	return m
}
