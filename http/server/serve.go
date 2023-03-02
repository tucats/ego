package server

import (
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/util"
)

// ServeHTTP satisifies the requirements of an HTTP multiplexer to
// the Go "http" package. This accepts a request and reqponse writer,
// and determines which path to direct the request to.
//
// This function also handles creating the *Session object passed to
// the handler, and basic logging.
func (m *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	route, status := m.FindRoute(r.URL.Path, r.Method)

	// Now that we (potentially) have a route, increment the session count
	// if this is not silent. Note that a failed route connection always
	// counts as a session id.
	sessionID := 0
	if route == nil || !route.lightweight {
		sessionID = int(atomic.AddInt32(&sequenceNumber, 1))
	}

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

	session := &Session{
		URLParts: route.makeMap(r.URL.Path),
		Path:     route.endpoint,
		handler:  route.handler,
		ID:       sessionID,
		Instance: route.router.name,
		Filename: route.filename,
	}

	// IF this route has a service class associated with it for auditing service
	// stats, then count it.
	if route.auditClass > NotCounted {
		CountRequest(route.auditClass)
	}

	if !route.lightweight {
		// Log the detailed information on the request, before any conditions that might
		// set the result status.
		LogRequest(r, session.ID)

		// Process any authentication info in the request, and add it to the session.
		session.Authenticate(r)
	}

	// Validate that the parameters provided are all permitted.
	if err := util.ValidateParameters(r.URL, route.parameters); err != nil {
		status = util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
	}

	// If the service requires authentication or admin status, then if either test
	// fails, set the result accordingly. If both are okay, then just run the handler.
	if status == http.StatusOK {
		if route.mustAuthenticate && !session.Authenticated {
			w.Header().Set(defs.AuthenticateHeader, `Basic realm=`+strconv.Quote(Realm)+`, charset="UTF-8"`)
			status = util.ErrorResponse(w, session.ID, "not authorized", http.StatusUnauthorized)
		} else if route.mustBeAdmin && !session.Admin {
			status = util.ErrorResponse(w, session.ID, "not authorized", http.StatusForbidden)
		}
	}

	// Call the designated route handler
	if status == http.StatusOK {
		status = session.handler(session, w, r)
	}

	w.Header().Add(defs.EgoServerInstanceHeader, defs.ServerInstanceID)

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

		elapsed := time.Since(start).String()

		user := ""
		if session.User != "" {
			user = "; user " + session.User
		}

		ui.Log(ui.ServerLogger, "[%d] %d %s %s from %s%s%s; elapsed %s", session.ID, status, r.Method, r.URL.Path,
			r.RemoteAddr, user, contentType, elapsed)
	}
}

// Given a path string from the user's request, use the route
// pattern inforamtion to create a map describing each field
// in the URL. If there is no patter, this returns a nil map.
func (r *Route) makeMap(path string) map[string]interface{} {
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
