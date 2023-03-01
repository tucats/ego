package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/util"
)

// The router HTTP method to specify when you mean all possible methods are sent
// to the same router. If a specific method is to be used, then it must be one
// of the valid http method names like "GET", "DELETE", etc.
const AnyMethod = "ANY"

// This value contains the sequence number for sessions (individual HTTP requests).
var sessionID int32 = 0

// The type of a service handler that uses this router. This is the same as a
// standard http server, with the addition of the *Session information that provides
// context for the specific service invocation.
type HandlerFunc func(*Session, http.ResponseWriter, *http.Request) int

// Session contains information passed to the service handler.
type Session struct {
	// The path that resulted in this route being selected.
	Path string

	// The filename of the associated service file, if any
	Filename string

	// A map of each part of the URL (or user value).
	URLParts map[string]interface{}

	// The function pointer to the handler itself.
	handler HandlerFunc

	// The UUID of this server instance as a string
	Instance string

	// The unique session ID for this request.
	ID int

	// The token string used to authenticate, if any
	Token string

	// The username used to authenticate, if any
	User string

	// True if the user was successfully authenticated
	Authenticated bool

	// True if the user was an administrator
	Admin bool
}

// Route describes the mapping of an endpoint to a function. This includes the
// base endpoint string, an optional pattern used to identify elements of a
// collection-style URL, an optional filename where the service code can be
// found, the method supported by this route, the function handler, and status
// information about the requirements for authentication for this route.
type Route struct {
	endpoint         string
	pattern          string
	filename         string
	method           string
	handler          HandlerFunc
	router           *Router
	mustAuthenticate bool
	mustBeAdmin      bool
	class            ServiceClass
}

// Selector is the key used to uniquely identify each route. It consists of the
// endpoint base (not the pattern) and the method. If all mets are supported, this
// must be the AnyMethod value.
type Selector struct {
	endpoint string
	method   string
}

// Router is a service router that is used to handle HTTP requests
// and dispatch them to handlers based on the path, method, etc.
type Router struct {
	name   string
	routes map[Selector]*Route
	mutex  sync.Mutex
}

// NewRouter creates a new router object. The name is a descriptive
// name used only for debugging.
func NewRouter(name string) *Router {
	mux := Router{
		name:   name,
		routes: map[Selector]*Route{},
	}

	return &mux
}

// NewRoute defines a new endpoint route. The endpoint string is provided
// as a parameter, along with the function pointer that implements the
// handle. Finally, the method for this route is specified. If the method
// is an empty string or "ALL" then it applies to all  methods.
//
// This returns a *Route, which can be used to chain additional attributes.
// If the method type was invalid, a nil pointer is returned.
func (m *Router) NewRoute(endpoint string, fn HandlerFunc, method string) *Route {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if method == "" || method == "*" {
		method = AnyMethod
	}

	method = strings.ToUpper(method)
	if !util.InList(method, "GET", "POST", "DELETE", "UPDATE", "PUT", AnyMethod) {
		return nil
	}

	route := &Route{
		endpoint: endpoint,
		pattern:  endpoint,
		handler:  fn,
		router:   m,
		class:    NotCounted,
		method:   method,
	}

	index := Selector{endpoint: endpoint, method: method}
	if _, found := m.routes[index]; found {
		panic(fmt.Errorf("Internal error, duplicate route definition %v", index))
	}

	m.routes[index] = route

	return route
}

// Filename sets the physical file name of the service file, if any,
// if it is different than the location referenced by the endpoint.
func (r *Route) Filename(filename string) *Route {
	if r != nil {
		r.filename = filename
	}

	return r
}

// Pattern adds a pattern match to the route. A pattern match extends the
// endpoint by indicating fields in the URL that are user-supplied and can
// be provided to the handler in a map.
func (r *Route) Pattern(pattern string) *Route {
	if r != nil {
		if pattern != "" {
			r.pattern = pattern
		}
	}

	return r
}

// Authentication indicates that the route might be otherwise valid but
// must also match the required valid authentication and adnimistrator
// status.
//
// If these are not set, they are not checked. But if they are set, the
// router will return suitable HTTP status without calling the handler.
func (r *Route) Authentication(valid, administrator bool) *Route {
	if r != nil {
		r.mustAuthenticate = valid || administrator
		r.mustBeAdmin = administrator
	}

	return r
}

// Class sets the request classification for counting purposes in the
// server audit function.
func (r *Route) Class(class ServiceClass) *Route {
	if r != nil {
		r.class = class
	}

	return r
}

// ServeHTTP satisifies the requirements of an HTTP multiplexer to
// the Go "http" package. This accepts a request and reqponse writer,
// and determines which path to direct the request to.
//
// This function also handles creating the *Session object passed to
// the handler, and basic logging.
func (m *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	status := http.StatusOK
	route := m.findRoute(r.URL.Path, r.Method)

	if route == nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	session := &Session{
		URLParts: route.makeMap(r.URL.Path),
		Path:     route.endpoint,
		handler:  route.handler,
		ID:       int(atomic.AddInt32(&sessionID, 1)),
		Instance: route.router.name,
		Filename: route.filename,
	}

	// IF this route has a service class associated with it for auditing service
	// stats, then count it.
	if route.class > NotCounted {
		CountRequest(route.class)
	}

	// Process any authentication info in the request, and add it to the session.
	session.Authenticate(r)

	// Log the detailed information on the request, before any conditions that might
	// set the result status.
	LogRequest(r, session.ID)

	// If the service requires authenitication or admin status, then if either fails
	// set the result accordingly. If both are okay, then just run the handler.
	if route.mustAuthenticate && !session.Authenticated {
		w.Header().Set("WWW-Authenticate", `Basic realm=`+strconv.Quote(Realm)+`, charset="UTF-8"`)
		status = util.ErrorResponse(w, session.ID, "not authorized", http.StatusUnauthorized)
	} else if route.mustBeAdmin && !session.Admin {
		status = util.ErrorResponse(w, session.ID, "not authorized", http.StatusForbidden)
	} else {
		// Call the designated route handler
		status = session.handler(session, w, r)
	}

	w.Header().Add("X-Ego-Server", defs.ServerInstanceID)
	LogResponse(w, session.ID)

	// Prepare an end-of-request message for the SERVER logger.
	contentType := w.Header().Get("Content-Type")
	if contentType != "" {
		contentType = "; " + contentType
	} else {
		w.Header().Set("Content-Type", "text")
		contentType = "; text"
	}

	ui.Log(ui.ServerLogger, "[%d] %d %s %s from %s%s", session.ID, status, r.Method, route.endpoint,
		r.RemoteAddr, contentType)
}

// For a given path and method ("GET", "DELETE", etc.), find  the appropriate
// route to a handler.
func (m *Router) findRoute(path, method string) *Route {
	candidates := []*Route{}

	method = strings.ToUpper(method)

	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/") + "/"
	}

	// Find the best match for this path. This includes cases where
	// there is a pattern that helps us match up.
	for selector, route := range m.routes {
		endpoint := selector.endpoint

		if len(endpoint) > 1 {
			endpoint = strings.TrimSuffix(endpoint, "/") + "/"
		}

		ui.Log(ui.InternalLogger, "Comparing %s with %s", path, endpoint)
		// If this is an endpoint match, add it to the candidate list.
		testPath := path
		if len(testPath) > len(endpoint) {
			testPath = testPath[:len(endpoint)]
		}

		if testPath == endpoint {
			candidates = append(candidates, route)
			ui.Log(ui.InternalLogger, "adding %v", selector)

		}
	}

	// Based on the length of the candidate list, return the best
	// route candidate found.
	switch len(candidates) {
	case 0:
		return nil

	case 1:
		return candidates[0]

	default:
		// If a candidate has a method, prioritize that one.
		for _, route := range candidates {
			if strings.EqualFold(route.method, method) {
				return route
			}
		}

		// Otherwise, use the candidate with the longest path
		longest := 0
		for index := 1; index < len(candidates); index++ {
			if len(candidates[index].endpoint) > len(candidates[longest].endpoint) {
				longest = index
			}
		}

		return candidates[longest]
	}
}

// Given a path string from the user's request, use the route
// pattern inforamtion to create a map describing each field
// in the URL. If there is no patter, this returns a nil map.
func (r *Route) makeMap(path string) map[string]interface{} {
	if r.pattern == "" {
		return nil
	}

	m := map[string]interface{}{}
	path = strings.TrimPrefix(strings.TrimSuffix(path, "/"), "/")
	segments := strings.Split(path, "?")
	pathSegment := strings.TrimPrefix(strings.TrimSuffix(segments[0], "/"), "/")
	pathParts := strings.Split(pathSegment, "/")
	patternParts := strings.Split(strings.TrimPrefix(strings.TrimSuffix(r.pattern, "/"), "/"), "/")

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
