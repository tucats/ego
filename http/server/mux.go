package server

import (
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

var sessionID int32 = 0

// The type of a service handler that uses this router. This is
// the same as a standard http server, with the addition of the
// Session information that provides context for the specific
// service invocation.
type HandlerFunc func(*Session, http.ResponseWriter, *http.Request) int

type Session struct {
	Path     string
	URLParts map[string]interface{}
	handler  HandlerFunc
	Instance string
	ID       int
}

type Route struct {
	endpoint string
	pattern  string
	methods  map[string]bool
	filename string
	handler  HandlerFunc
}

type Router struct {
	Name   string
	routes map[string]*Route
	mutex  sync.Mutex
}

func NewRouter(name string) *Router {
	mux := Router{
		Name:   name,
		routes: map[string]*Route{},
	}

	return &mux
}

func (m *Router) NewRoute(endpoint string, fn HandlerFunc) *Route {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	route := &Route{
		endpoint: endpoint,
		pattern:  endpoint,
		handler:  fn,
		filename: strings.TrimSuffix(endpoint, "/") + ".ego",
	}

	m.routes[endpoint] = route

	return route
}

func (r *Route) Pattern(pattern string) *Route {
	if pattern != "" {
		r.pattern = pattern
	}

	return r
}

func (r *Route) Methods(methods ...string) *Route {
	if r.methods == nil {
		r.methods = map[string]bool{}
	}

	for _, method := range methods {
		r.methods[strings.ToUpper(method)] = true
	}

	return r
}

func (m *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := m.findRoute(r.URL.Path, r.Method)

	if route == nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	h := &Session{
		URLParts: route.makeMap(r.URL.Path),
		Path:     route.endpoint,
		handler:  route.handler,
		ID:       int(atomic.AddInt32(&sessionID, 1)),
		Instance: defs.ServerInstanceID,
	}

	// Call the designated route handler
	LogRequest(r, h.ID)
	status := h.handler(h, w, r)
	LogResponse(w, h.ID)

	contentType := w.Header().Get("Content-Type")
	if contentType != "" {
		contentType = "; " + contentType
	} else {
		w.Header().Set("Content-Type", "text")
		contentType = "; text"
	}

	ui.Log(ui.ServerLogger, "[%d] %d %s %s from %s%s", h.ID, status, r.Method, route.endpoint,
		r.RemoteAddr, contentType)
}

func (m *Router) findRoute(path, method string) *Route {
	var found *Route

	method = strings.ToUpper(method)

	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}

	// Find the best match for this path. This includes cases where
	// there is a pattern that helps us match up.
	for endpoint, route := range m.routes {
		if len(endpoint) > 1 {
			endpoint = strings.TrimSuffix(endpoint, "/")
		}

		// If there is a set of methods that must match this
		// route specification, if not valid for this route
		// the keep looking.
		if route.methods != nil && !route.methods[method] {
			continue
		}

		// If this is an endpoint match, then verify we don't
		// already have one that is a longer string match.
		testPath := path
		if len(testPath) > len(endpoint) {
			testPath = testPath[:len(endpoint)]
		}

		if testPath == endpoint {
			if found == nil {
				found = route
			} else if len(found.endpoint) < len(endpoint) {
				found = route
			}
		}
	}

	return found
}

func (r *Route) makeMap(path string) map[string]interface{} {
	if r.pattern == "" {
		return nil
	}

	m := map[string]interface{}{}
	path = strings.TrimPrefix(path, "/")
	segments := strings.Split(path, "?")
	pathParts := strings.Split(segments[0], "/")
	patternParts := strings.Split(strings.TrimPrefix(r.pattern, "/"), "/")

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
			m[part] = true
		}
	}

	return m
}
