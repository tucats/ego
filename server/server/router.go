package server

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/util"
)

// The router HTTP method to specify when you mean all possible methods are sent
// to the same router. If a specific method is to be used, then it must be one
// of the valid http method names like "GET", "DELETE", etc.
const AnyMethod = "ANY"

// This value contains the sequence number for sessions (individual HTTP requests).
var sequenceNumber int32 = 0

// The type of a service handler that uses this router. This is the same as a
// standard http server, with the addition of the *Session information that provides
// context for the specific service invocation.
type HandlerFunc func(*Session, http.ResponseWriter, *http.Request) int

// Session contains information passed to the service handler.
type Session struct {
	// The path that resulted in this route being selected. This
	// includes any substitution (or variable) parts of the URL
	// string.
	Path string

	// The filename of the associated service file, if any. For
	// services that do not use an Ego program, this field is
	// an empty string.
	Filename string

	// If there is a redirect for this session, the redirect URL
	// is stored here from the route table.
	Redirect string

	// A map of each part of the URL (or user value). For static
	// parts, the map key is the static field in the URL and the
	// value is a boolean indicating if that part of the URL was
	// found in the path. For variable values, the map key is the
	// name of the variable from the path, and the value is the
	// string value of the source URL component.
	URLParts map[string]interface{}

	// Map of the parameters found on the URL, by name. The
	// value is always an array of strings.
	Parameters map[string][]string

	// The function pointer to the handler itself.
	handler HandlerFunc

	// The UUID of this server instance as a string
	Instance string

	// The unique session ID for this request. Each inbound
	// REST request to the server (other than lightweight
	// services) increments the sesion sequence number,
	// which is returned here. When the server is restarted,
	// this sequence number starts over again, so a unique
	// service must be a combination of this sequence number
	// and the UUUID of the Instance.
	ID int

	// The token string used to authenticate, if any.
	// If the user did not supply credentials, or they
	// used Basic authentication, this will be an empty
	// string.
	Token string

	// The username used to authenticate. If there was no
	// data, this will be an empty string. If the user
	// was specified in a Basic authentication header,
	// the username will be here even if it did not
	// successfully authenticate.
	User string

	// If there is an expiration associated with this
	// session, it is stored here.
	Expiration string

	// True if the user was successfully authenticated
	Authenticated bool

	// True if the user has administrator privileges
	Admin bool

	// True if the request will accept a JSON response
	AcceptsJSON bool

	// True if the request will accept a TEXT response
	AcceptsText bool

	// Length (in bytes) of the response body
	ResponseLength int
}

// Route describes the mapping of an endpoint to a function. This includes the
// base endpoint string, an optional pattern used to identify elements of a
// collection-style URL, an optional filename where the service code can be
// found, the method supported by this route, the function handler, and status
// information about the requirements for authentication for this route.
type Route struct {
	endpoint            string
	filename            string
	method              string
	redirect            string
	handler             HandlerFunc
	router              *Router
	parameters          map[string]string
	requiredPermissions []string
	mediaTypes          []string
	mustAuthenticate    bool
	mustBeAdmin         bool
	lightweight         bool
	auditClass          ServiceClass
}

// routeSelector is the key used to uniquely identify each route. It consists of the
// endpoint (with optional collection pattern) and the method. If all methods are
// supported, the method value must be defs.AnyMethod.
type routeSelector struct {
	endpoint string
	method   string
}

// Router is a service router that is used to handle HTTP requests
// and dispatch them to handlers based on the path, method, etc.
type Router struct {
	name   string
	routes map[routeSelector]*Route
	mutex  sync.Mutex
}

// NewRouter creates a new router object. The name is a descriptive
// name used only for debugging and is set to the UUID of the server
// instance if not specified.
func NewRouter(name string) *Router {
	if name == "" {
		name = defs.ServerInstanceID
	}

	mux := Router{
		name:   name,
		routes: map[routeSelector]*Route{},
	}

	return &mux
}

// New defines a new endpoint route. The endpoint string is provided
// as a parameter, along with the function pointer that implements the
// handle. Finally, the method for this route is specified. If the method
// is an empty string or "ALL" then it applies to all  methods.
//
// This returns a *Route, which can be used to chain additional attributes.
// If the method type was invalid, a nil pointer is returned.
func (m *Router) New(endpoint string, fn HandlerFunc, method string) *Route {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if method == "" || method == "*" {
		method = AnyMethod
	}

	method = strings.ToUpper(method)
	if !util.InList(method, "GET", "POST", "DELETE", "UPDATE", "PUT", "PATCH", AnyMethod) {
		panic(fmt.Errorf("internal error, invalid route method %v", method))
	}

	route := &Route{
		endpoint:   endpoint,
		handler:    fn,
		router:     m,
		auditClass: NotCounted,
		method:     method,
	}

	index := routeSelector{endpoint: endpoint, method: method}
	if _, found := m.routes[index]; found {
		panic(fmt.Errorf("internal error, duplicate route definition %v", index))
	}

	m.routes[index] = route

	return route
}

// Permissions specifies one or more user permissions that are required or the authenticated
// user to be able to access the endpoint.
func (r *Route) Permissions(permissions ...string) *Route {
	if r != nil {
		r.mustAuthenticate = true

		if r.requiredPermissions == nil {
			r.requiredPermissions = []string{}
		}

		for _, permission := range permissions {
			duplicate := false

			for _, requiredPermission := range r.requiredPermissions {
				if requiredPermission == permission {
					duplicate = true

					break
				}
			}

			if !duplicate {
				r.requiredPermissions = append(r.requiredPermissions, permission)
			}
		}

		r.mustAuthenticate = true
	}

	return r
}

// AcceptMedia specifies one or more user mediat types that are required for endpoint
// validation. If no media types are assigned, then all media types are accepted.
func (r *Route) AcceptMedia(mediaTypes ...string) *Route {
	if r != nil {
		if r.mediaTypes == nil {
			r.mediaTypes = []string{}
		}

		for _, mediaType := range mediaTypes {
			duplicate := false

			for _, expectedMediaType := range r.mediaTypes {
				if expectedMediaType == mediaType {
					duplicate = true

					break
				}
			}

			if !duplicate {
				r.mediaTypes = append(r.mediaTypes, mediaType)
			}
		}
	}

	return r
}

// LightWeight marks this route as requiring little support, and is not
// logged. For example, a heartbeat endpoint would be counted but not
// logged, so this flag would be set.
//
// When the flag is true, the log does not contain REST or SERVER entries
// for the endpoint, and the session sequence number is not incremented.
// A lightweight route also cannot require/use authentication.
func (r *Route) LightWeight(flag bool) *Route {
	if r != nil {
		r.lightweight = flag
		r.mustAuthenticate = !flag
	}

	return r
}

func (r *Route) Parameter(name, kind string) *Route {
	if r == nil {
		return nil
	}

	if r.parameters == nil {
		r.parameters = map[string]string{}
	}

	if !util.InList(kind, defs.Any, util.FlagParameterType, util.BoolParameterType, util.IntParameterType, util.StringParameterType, util.ListParameterType) {
		panic("invalid parameter validation type: " + kind)
	}

	r.parameters[name] = kind

	return r
}

// Redirect specifies that this route is not a service, but instead is a
// redirect to another URL.
func (r *Route) Redirect(url string) *Route {
	if r != nil {
		r.redirect = url
	}

	return r
}

// Filename sets the physical file name of the service file, if any,
// if it is different than the location referenced by the endpoint.
func (r *Route) Filename(filename string) *Route {
	if r != nil {
		r.filename = filename
	}

	return r
}

// Authentication indicates that the route might be otherwise valid but
// must also match the required valid authentication and administrator
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
		r.auditClass = class
	}

	return r
}

// For a given method and path and method, find  the appropriate route to
// a handler.
//
// The function returns the route found, and any HTTP status value that might
// arrise from processing the request. If the status value is not StatusOK,
// the route pointer is typically nil.
func (m *Router) FindRoute(method, path string) (*Route, int) {
	candidates := []*Route{}
	method = strings.ToUpper(method)

	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/") + "/"
	}

	ui.Log(ui.RouteLogger, "[0] searching for route for %s %s", method, path)

	// Find the best match for this path. This includes cases where
	// there is a pattern that helps us match up.
	for selector, route := range m.routes {
		endpoint := selector.endpoint

		// Special case the "everything" to always be considered a candidate
		// if it was a defined route.
		if route.endpoint == "/" {
			candidates = append(candidates, route)

			continue
		}

		if len(endpoint) > 1 {
			endpoint = strings.TrimSuffix(endpoint, "/") + "/"
		}

		// If the endpoint includes substitutions, then convert the
		// path to match.
		testPath := path
		testParts := strings.Split(testPath, "/")
		endpointParts := strings.Split(endpoint, "/")

		maskedParts := []string{}

		for i, endpointPart := range endpointParts {
			if strings.HasPrefix(endpointPart, "{{") {
				maskedParts = append(maskedParts, endpointPart)
			} else {
				if i >= len(testParts) {
					maskedParts = append(maskedParts, endpointPart)
				} else {
					maskedParts = append(maskedParts, testParts[i])
				}
			}
		}

		maskedEndpoint := strings.Join(maskedParts, "/")

		// If the endpoints line up, ensure that the method is acceptable
		// for this route, and then append as needed.
		if endpoint == maskedEndpoint {
			if route.method == AnyMethod || strings.EqualFold(route.method, method) {
				candidates = append(candidates, route)
			}
		}
	}

	// Based on the length of the candidate list, return the best
	// route candidate found.
	switch len(candidates) {
	case 0:
		ui.Log(ui.RouteLogger, "[0] no routes matched")

		return nil, http.StatusNotFound

	case 1:
		// We only found one, but let's make sure that if the route method either
		// accepts all methods, or it matches the method we're using for this request.
		ui.Log(ui.RouteLogger, "[0] only route that matched was %s", candidates[0].endpoint)

		route := candidates[0]
		if route.method == AnyMethod || strings.EqualFold(route.method, method) {
			return route, http.StatusOK
		}

		// Method didn't match up, so not found...
		return nil, http.StatusMethodNotAllowed

	default:
		ui.Log(ui.RouteLogger, "[0] There are %d possible candidate routes", len(candidates))

		for _, candidate := range candidates {
			ui.Log(ui.RouteLogger, "[0] route candidate: %s", candidate.endpoint)
		}

		// Find the candidate with the exact match, if any.
		for _, candidate := range candidates {
			if candidate.endpoint == path {
				ui.Log(ui.RouteLogger, "[0] exact match: %s", candidate.endpoint)

				return candidate, http.StatusOK
			}
		}

		// If there is one that has no variables, let's use that one (this lets
		// priority to to /tables/@sql over /tables/{{name}} for example.)
		for _, candidate := range candidates {
			// If there are no variable fields in the URL, choose this one first.
			if !strings.Contains(candidate.endpoint, "{{") && !strings.Contains(candidate.endpoint, "}}") {
				ui.Log(ui.RouteLogger, "[0] no variables: %s", candidate.endpoint)

				return candidate, http.StatusOK
			}
		}

		// Is there a match with the exact same number of URL parts? If so, use that.
		pathPartCount := strings.Count(path, "/")

		ui.Log(ui.RouteLogger, "[0] path contains %d parts", pathPartCount-1)

		for _, candidate := range candidates {
			routePartCount := strings.Count(candidate.endpoint, "/")
			if !strings.HasSuffix(candidate.endpoint, "/") {
				routePartCount++
			}

			ui.Log(ui.RouteLogger, "[0] candidate %s contains %d parts", candidate.endpoint, routePartCount-1)

			if pathPartCount == routePartCount {
				return candidate, http.StatusOK
			}
		}

		// Not sure, so use the candidate with the longest path
		longest := 0
		for index := 1; index < len(candidates); index++ {
			if len(candidates[index].endpoint) > len(candidates[longest].endpoint) {
				longest = index
			}
		}

		ui.Log(ui.RouteLogger, "[0] longest match: %s", candidates[longest].endpoint)

		return candidates[longest], http.StatusOK
	}
}

func (m *Router) Dump() {
	if !ui.IsActive(ui.RouteLogger) {
		return
	}

	ui.Log(ui.RouteLogger, "Dump of router %s, with %d routes defined", m.name, len(m.routes))

	keys := []string{}
	for route := range m.routes {
		keys = append(keys, route.endpoint+" "+route.method)
	}

	sort.Strings(keys)

	for _, key := range keys {
		parts := strings.Fields(key)
		selector := routeSelector{method: parts[1], endpoint: parts[0]}
		route := m.routes[selector]
		key = pad(selector.method, 6) + " " + pad(selector.endpoint, 42)

		fn := runtime.FuncForPC(reflect.ValueOf(route.handler).Pointer()).Name()

		for _, prefix := range []string{"github.com/tucats/ego/", "http/", "tables/"} {
			fn = strings.TrimPrefix(fn, prefix)
		}

		if route.filename != "" {
			fn = fn + ", file=" + strconv.Quote(route.filename)
		}

		msg := fmt.Sprintf("   %s %s", key, fn)
		if len(route.mediaTypes) > 0 {
			msg = msg + fmt.Sprintf(", media=%v", route.mediaTypes)
		}

		if route.mustBeAdmin {
			msg = msg + ", mustBeAdmin"
		} else {
			if route.mustAuthenticate {
				msg = msg + ", mustAuth"
			}
		}

		if len(route.requiredPermissions) > 0 {
			msg = msg + fmt.Sprintf(", perms=%v", route.requiredPermissions)
		}

		ui.Log(ui.RouteLogger, msg)
	}
}

func pad(text string, width int) string {
	for len(text) < width {
		text = text + " "
	}

	return text
}
