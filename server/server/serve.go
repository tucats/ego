package server

import (
	"bytes"
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

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/util"
	"github.com/tucats/ego/validate"
)

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

var shutdownLock sync.Mutex

// ServeHTTP satisfies the requirements of an HTTP multiplexer to
// the Go "http" package. This accepts a request and response writer,
// and determines which path to direct the request to.
//
// This function also handles creating the *Session object passed to
// the handler, and basic logging.
func (m *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var session *Session

	// Make sure we aren't blocked on shutdown.
	shutdownLock.Lock()
	shutdownLock.Unlock()

	// Record when this particular request began, and find the matching
	// route for this request.
	start := time.Now()
	route, status := m.FindRoute(r.Method, r.URL.Path)

	// Now that we (potentially) have a route, increment the session count
	// if this is not a "lightweight" request type. Note that a failed route
	// connection always counts as a session attempt and increments the
	// sequence number.
	sessionID := 0
	if route == nil || !route.lightweight {
		sessionID = int(atomic.AddInt32(&SequenceNumber, 1))
	}

	// Stamp the response with the instance ID of this server and the
	// session ID for this request.
	w.Header()[defs.EgoServerInstanceHeader] = []string{fmt.Sprintf("%s:%d", defs.InstanceID, sessionID)}

	// Problem with the path? Log it based on whether the method was not found or
	// unsupported.
	if status != http.StatusOK {
		msg := "invalid URL"

		switch status {
		case http.StatusMethodNotAllowed:
			msg = "method " + r.Method + " not allowed"

		case http.StatusForbidden:
			msg = "forbidden access to " + r.URL.Path

		case http.StatusNotFound:
			msg = "endpoint " + r.URL.Path + " not found"
		}

		util.ErrorResponse(w, sessionID, msg, status)
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

	// If we found a route, make a session object.  Set the media type
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
			URLParts:    route.partsMap(r.URL.Path),
			Parameters:  route.parmMap(r),
			Path:        route.endpoint,
			URL:         r.URL,
			handler:     route.handler,
			ID:          sessionID,
			Instance:    route.router.name,
			Filename:    route.filename,
			AcceptsJSON: json,
			AcceptsText: text,
			Redirect:    route.redirect,
			Validations: route.Validations(),
			Router:      m,
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
			if !auth.GetPermission(session.User, permission) {
				ui.Log(ui.RouteLogger, "route.perm.auth", ui.A{
					"session":    session.ID,
					"permission": permission,
					"user":       session.User,
				})

				sts := http.StatusForbidden
				if session.User == "" {
					sts = http.StatusUnauthorized

					w.Header().Add(defs.AuthenticateHeader,
						fmt.Sprintf(`Basic realm=%s, charset="UTF-8"`, strconv.Quote(Realm)))
				}

				status = util.ErrorResponse(w, session.ID, "User does not have privilege "+permission+" to access this endpoint", sts)
			}
		}
	}

	// While we're here, copy the permissions list to the session for future use.
	if session.User != "" {
		session.Permissions = auth.GetPermissions(session.User)
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
		if route.mustAuthenticate && !session.Authenticated {
			w.Header().Set(defs.AuthenticateHeader, `Basic realm=`+strconv.Quote(Realm)+`, charset="UTF-8"`)
			ui.Log(ui.RouteLogger, "route.cred", ui.A{
				"session": session.ID,
			})

			status = util.ErrorResponse(w, session.ID, "not authorized", http.StatusUnauthorized)
		} else if route.mustBeAdmin && !session.Admin {
			ui.Log(ui.RouteLogger, "route.admin", ui.A{
				"session": session.ID,
			})

			status = util.ErrorResponse(w, session.ID, "not authorized", http.StatusForbidden)
		}
	}

	// Move the request body to the session object.
	if r.Body != nil {
		session.Body, _ = io.ReadAll(r.Body)
		r.Body.Close()

		// We also reset the body reader to be a new reader that will just re-read
		// the bytes already in memory.
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
			shutdownLock.Lock()
			go func() {
				time.Sleep(1 * time.Second)
				ui.Log(ui.ServerLogger, "server.shutdown", nil)
				os.Exit(0)
			}()
		}
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
func (r *Route) partsMap(path string) map[string]interface{} {
	m := map[string]interface{}{}
	path = strings.TrimPrefix(strings.TrimSuffix(path, "/"), "/")
	segments := strings.Split(path, "?")
	pathSegment := strings.TrimPrefix(strings.TrimSuffix(segments[0], "/"), "/")
	pathParts := strings.Split(pathSegment, "/")
	patternParts := strings.Split(strings.TrimPrefix(strings.TrimSuffix(r.endpoint, "/"), "/"), "/")

	for index, part := range patternParts {
		// if this part of the pattern is a named value, make it part
		// of the result with a string value.
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
