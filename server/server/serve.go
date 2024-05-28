package server

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/auth"
	"github.com/tucats/ego/util"
)

// ServeHTTP satisfies the requirements of an HTTP multiplexer to
// the Go "http" package. This accepts a request and reqponse writer,
// and determines which path to direct the request to.
//
// This function also handles creating the *Session object passed to
// the handler, and basic logging.
func (m *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var session *Session

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
		sessionID = int(atomic.AddInt32(&sequenceNumber, 1))
	}

	// Problem with the path? Log it based on whether the method was not found or
	// unsupported.
	if status != http.StatusOK {
		msg := "invalid URL"

		switch status {
		case http.StatusMethodNotAllowed:
			msg = "method " + r.Method + " not allowed"

		case http.StatusNotFound:
			msg = "endpoint " + r.URL.Path + " not found"
		}

		util.ErrorResponse(w, sessionID, msg, status)
		ui.Log(ui.ServerLogger, "[%d] %d %s %s from %s", sessionID, status, r.Method, r.URL.Path,
			r.RemoteAddr)

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
			handler:     route.handler,
			ID:          sessionID,
			Instance:    route.router.name,
			Filename:    route.filename,
			AcceptsJSON: json,
			AcceptsText: text,
			Redirect:    route.redirect,
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
			if route.handler == nil {
				msg := fmt.Sprintf("invalid route selected: %#v", route)

				ui.Log(ui.InternalLogger, msg)
				util.ErrorResponse(w, sessionID, msg, http.StatusInternalServerError)

				return
			}

			fn := runtime.FuncForPC(reflect.ValueOf(route.handler).Pointer()).Name()

			for _, prefix := range []string{"github.com/tucats/ego/", "http/", "tables/"} {
				fn = strings.TrimPrefix(fn, prefix)
			}

			if route.filename != "" {
				fn = fn + ", file " + strconv.Quote(route.filename)
			}

			ui.Log(ui.RestLogger, "[%d] Route %s selected, handler %s", sessionID, route.endpoint, fn)
		}
	}

	// Validate request media types required for this route, if any.
	if route != nil && route.mediaTypes != nil {
		ui.Log(ui.RestLogger, "[%d] Validating request against accepted media types: %v", sessionID, route.mediaTypes)

		if err := util.AcceptedMediaType(r, route.mediaTypes); err != nil {
			status = util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
		}
	}

	// Validate required permissions that must exist for this user. We skip this if the
	// user authenticated as an admin account. If any permissions are missing, we fail
	// with a Forbidden error.
	if status == http.StatusOK && (route.requiredPermissions != nil && !session.Admin) {
		for _, permission := range route.requiredPermissions {
			if !auth.GetPermission(session.User, permission) {
				ui.Log(ui.RouteLogger, "[0] Required route permission %s not authorized for user %s", permission, session.User)

				sts := http.StatusForbidden
				if session.User == "" {
					sts = http.StatusUnauthorized

					w.Header().Add("WWW-Authenticate", "Basic realm=\"Access to API\"")
				}

				status = util.ErrorResponse(w, session.ID, "User does not have privilege "+permission+" to access this endpoint", sts)
			}
		}
	}

	// Validate that the parameters provided are all permitted and of the correct form.
	if status == http.StatusOK {
		if err := util.ValidateParameters(r.URL, route.parameters); err != nil {
			status = util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	}

	// Validate that the user is authenticated if required by the route.
	if status == http.StatusOK {
		if route.mustAuthenticate && !session.Authenticated {
			w.Header().Set(defs.AuthenticateHeader, `Basic realm=`+strconv.Quote(Realm)+`, charset="UTF-8"`)
			ui.Log(ui.RouteLogger, "[0] Required credentials not provided")

			status = util.ErrorResponse(w, session.ID, "not authorized", http.StatusUnauthorized)
		} else if route.mustBeAdmin && !session.Admin {
			ui.Log(ui.RouteLogger, "[0] Required admin privilege not authorized for user")

			status = util.ErrorResponse(w, session.ID, "not authorized", http.StatusForbidden)
		}
	}

	// Stamp the response with the instance ID of this server and the
	// session ID for this request.
	w.Header()[defs.EgoServerInstanceHeader] = []string{fmt.Sprintf("%s:%d", defs.ServerInstanceID, session.ID)}

	// Call the designated route handler
	if status == http.StatusOK {
		status = session.handler(session, w, r)
	}

	if !route.lightweight {
		LogResponse(w, session.ID)

		// Prepare an end-of-request message for the SERVER logger.
		contentType := w.Header().Get(defs.ContentTypeHeader)
		if contentType != "" {
			contentType = "; content " + contentType
		} else {
			w.Header().Set(defs.ContentTypeHeader, "text")

			contentType = "; content text"
		}

		if session.ResponseLength > 0 {
			contentType = contentType + "; length " + strconv.Itoa(session.ResponseLength)
		}

		elapsed := time.Since(start).String()

		user := ""
		if session.User != "" {
			user = "; user " + session.User
		}

		ui.Log(ui.ServerLogger, "[%d] %d %s %s from %s%s%s; elapsed %s", session.ID, status, r.Method, r.URL.Path,
			r.RemoteAddr, user, contentType, elapsed)
	}
}

// Given a request, build a map of the parameters in the URL.
func (r *Route) parmMap(req *http.Request) map[string][]string {
	result := map[string][]string{}

	parms := req.URL.Query()

	for parm, list := range parms {
		result[parm] = list
	}

	return result
}

// Given a path string from the user's request, use the route
// pattern inforamtion to create a map describing each field
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
