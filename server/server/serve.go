package server

import (
	"bytes"
	nativeErrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/util"
	"github.com/tucats/ego/validate"
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

// ServeHTTP satisfies the requirements of an HTTP multiplexer to
// the Go "http" package. This accepts a request and response writer,
// and determines which path to direct the request to.
//
// This function also handles creating the *Session object passed to
// the handler, and basic logging.
func (m *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var session *Session

	// Make sure we aren't blocked on shutdown.
	ServerShutdownLock.Lock()

	// Record when this particular request began, and find the matching
	// route for this request.
	start := time.Now()
	route, status := m.FindRoute(r.Method, r.URL.Path, true)
	defer route.Unlock()

	// If we've gotten this far, not blocked for shutdown.
	ServerShutdownLock.Unlock()

	// Now that we (potentially) have a route, increment the session count
	// if this is not a "lightweight" request type. Note that a failed route
	// connection always counts as a session attempt and increments the
	// sequence number.
	sessionID := 0
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

		switch status {
		case http.StatusMethodNotAllowed:
			msg = "method " + r.Method + " not allowed"
			clientMsg = msg // method name is safe to echo; it's our own validated string

		case http.StatusForbidden:
			msg = "forbidden access to " + r.URL.Path
			clientMsg = "forbidden"

		case http.StatusNotFound:
			msg = "endpoint " + r.URL.Path + " not found"
			clientMsg = "not found"
		}

		util.ErrorResponse(w, sessionID, clientMsg, status)
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
			util.ErrorResponse(w, session.ID, "too many failed login attempts", http.StatusTooManyRequests)

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

			util.ErrorResponse(w, session.ID, errors.ErrInvalidCredentials.Error(), status)

			return
		}
		// Log which route we're using. This is helpful for debugging service route
		// declaration errors.
		if ui.IsActive(ui.RestLogger) {
			// No route handler found, log it and report the error to the caller.
			if route.handler == nil {
				msg := fmt.Sprintf("invalid route selected: %#v", route)

				ui.Log(ui.InternalLogger, "route.handler.nil", ui.A{
					"route": fmt.Sprintf("%#v", route)})

				util.ErrorResponse(w, sessionID, msg, http.StatusInternalServerError)

				return
			}

			// Get the real name of the handler function, and clean it up by removing
			// noisy prefixes supplied by the reflection system.
			fn := runtime.FuncForPC(reflect.ValueOf(route.handler).Pointer()).Name()

			for _, prefix := range []string{"github.com/tucats/ego/", "http/", "tables/"} {
				fn = strings.TrimPrefix(fn, prefix)
			}

			if route.filename != "" {
				fn = fn + ", file " + strconv.Quote(route.filename)
			}

			ui.Log(ui.RestLogger, "route.handler", ui.A{
				"session":  sessionID,
				"endpoint": route.endpoint,
				"handler":  fn})
		}
	}

	// Validate request media types required for this route, if any.
	if route != nil && route.acceptMediaTypes != nil {
		ui.Log(ui.RestLogger, "rest.media.check", ui.A{
			"session": sessionID,
			"media":   route.acceptMediaTypes})

		if err := util.AcceptedMediaType(r, route.acceptMediaTypes); err != nil {
			status = util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
		}
	}

	if route != nil && route.contentMediaTypes != nil {
		ui.Log(ui.RestLogger, "rest.media.check", ui.A{
			"session": sessionID,
			"media":   route.contentMediaTypes})

		if err := util.ContentMediaType(r, route.contentMediaTypes); err != nil {
			status = util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
		}
	}

	// Validate required permissions that must exist for this user. We skip this if the
	// user authenticated as an admin account. If any permissions are missing, we fail
	// with a Forbidden error.
	if status == http.StatusOK && (route.requiredPermissions != nil && !session.Admin) {
		for _, permission := range route.requiredPermissions {
			if !auth.GetPermission(session.ID, session.User, permission) {
				ui.Log(ui.RouteLogger, "route.perm.auth", ui.A{
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

				status = util.ErrorResponse(w, session.ID, i18n.T("error.perm.privilege", ui.A{"permission": permission}), sts)
			}
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
			status = util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		} else if err := util.ValidateParameters(r.URL, route.parameters); err != nil {
			status = util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	}

	// Validate that the user is authenticated if required by the route.
	if status == http.StatusOK {
		if route.mustAuthenticate && !session.Authenticated && route.canAuthenticate {
			w.Header().Set(defs.AuthenticateHeader, `Basic realm=`+strconv.Quote(Realm)+`, charset="UTF-8"`)
			ui.Log(ui.RouteLogger, "route.cred", ui.A{
				"session": session.ID,
			})

			util.ErrorResponse(w, session.ID, "not authorized", http.StatusUnauthorized)

			return
		} else if route.mustBeAdmin && !session.Admin {
			ui.Log(ui.RouteLogger, "route.admin", ui.A{
				"session": session.ID,
			})

			util.ErrorResponse(w, session.ID, "not authorized", http.StatusForbidden)

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
				util.ErrorResponse(w, session.ID, "request body too large", http.StatusRequestEntityTooLarge)

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
			status = util.ErrorResponse(w, session.ID, last.Error(), status)
		}
	}

	// Call the designated route handler. This is where the actual work of the request will be done.
	if status == http.StatusOK {
		status = session.handler(session, w, r)
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
