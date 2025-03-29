package server

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
	"github.com/tucats/ego/validate"
)

// The router HTTP method to specify when you mean all possible methods are sent
// to the same router. If a specific method is to be used, then it must be one
// of the valid http method names like "GET", "DELETE", etc.
const AnyMethod = "ANY"

// This value contains the sequence number for sessions (individual REST requests).
var SequenceNumber int32 = 0

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

	// Validations is the list of valications that can be
	// used to verify the request payload.
	Validations []string

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

	// This is the list of permission strings associated
	// with the user who started this session.
	Permissions []string

	// True if the user was successfully authenticated
	Authenticated bool

	// True if the user has administrator privileges
	Admin bool

	// True if the request will accept a JSON response
	AcceptsJSON bool

	// True if the request will accept a TEXT response
	AcceptsText bool

	// Thje body of the requst. Nil if no body present
	Body []byte

	// The Router map used to find this endpoint
	Router *Router

	// Length (in bytes) of the response body
	ResponseLength int
}

// Route describes the mapping of an endpoint to a function. This includes the
// base endpoint string, an optional pattern used to identify elements of a
// collection-style URL, an optional filename where the service code can be
// found, the method supported by this route, the function handler, and status
// information about the requirements for authentication for this route.
type Route struct {
	// The endpoing string for this route.
	endpoint string

	// If the endpoint uses an Ego source file as the service handler, this
	// is the full path to the source file.
	filename string

	// What HTTP method (GET, PUT, etc.) is being handled by this route. There
	// is an individual route for each method.
	method string

	// If this route redirects to another location, the destination path is
	// found here.
	redirect string

	// What is the function that will handle the route? This may be a local handler
	// function, or the redirector handler, or the service handler.
	handler HandlerFunc

	// Point back to the parent router that contains this individual route.
	router *Router

	// A map of any parameters to be parsed in the url. The key is th eparameter
	// name (normalized to lowercase) and a string indicated the allowed value
	// of the paraemter, if any.
	parameters map[string]string

	// Does this route require one or more permissions to be granted to the caller
	// of this endpoint? If the list is empty, no permission checks are done.
	requiredPermissions []string

	// Disallowed parameters. Primary key is the parameter found, the map
	// is a list of all parameters that are disallowed with it.
	disallow map[string][]string

	// What are the allowed media types that can be requested by the caller for this
	// endpoint? If this is an empty list, no media type checking i sdone.
	acceptMediaTypes []string

	// What are the allowed media types that can be passed in by the caller for this
	// endpoint? If this is an empty list, no media type checking i sdone.
	contentMediaTypes []string

	// What validation object names do we use to validate the payload? If this
	// is an empty list, no validation is done.
	validations []string

	// Does this route require authentication? If so, there must be a valid Bearer token
	// or BasicAuth authentication associated with the request.
	mustAuthenticate bool

	// Does this endpoint require a user with admin privileges to access this endpoint?
	mustBeAdmin bool

	// If true, this is a "lightweight" endpoint that has reduced logging. For example,
	// an endpoing used to see if the server is up may not be logged.
	lightweight bool

	// Does this endpoint allow redirect? For example, if the endpoint is to the insecure
	// scheme (HTTP) but the server is running in secure mode, if this flag is true then
	// the endpoint may be redirected to the HTTPS scheme.
	allowRedirects bool

	// Does this endpoint generate a large response body that should normally be suppressed
	// from logging? An exmple is the admin service that retrieves the server log.
	largeReponse bool

	// An indicator of the class of service this endpoint is used for such as an Ego
	// service, and admin function, authentication, etc. This is used to log how many
	// requests of each service class occur in each ten-minute interval and are logged
	// by the server.
	auditClass ServiceClass
}

// routeSelector is the key used to uniquely identify each route. It consists of the
// endpoint (with optional collection pattern) and the method. If all methods are
// supported, the method value must be defs.AnyMethod.
type routeSelector struct {
	endpoint string
	method   string
}

// Router is a service router that is used to handle HTTP requests and dispatch them
// to handlers based on the path, method, etc. The mutex is used so map traversals
// within the router are serialzied to be thread-safe.
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
		name = defs.InstanceID
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
//
// Adding routes is a serialized operation so map traversals are thread-safe.
func (m *Router) New(endpoint string, fn HandlerFunc, method string) *Route {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if method == "" || method == "*" {
		method = AnyMethod
	}

	method = strings.ToUpper(method)
	if !util.InList(method, "GET", "POST", "DELETE", "UPDATE", "PUT", "PATCH", AnyMethod) {
		ui.Panic(fmt.Errorf("attempt to define new route with invalid route method %v", method))
	}

	route := &Route{
		endpoint:       endpoint,
		handler:        fn,
		router:         m,
		auditClass:     NotCounted,
		method:         method,
		allowRedirects: true,
	}

	// Construct a possible validation name for the route if it is a POST, PUT, or PATCH request.
	if util.InList(method, "POST", "PUT", "PATCH") {
		// Edit the endpoint to be a vaidation key string.
		key := strings.ToLower(strings.TrimPrefix(strings.TrimSuffix(endpoint, "/"), "/")) + ":" + strings.ToLower(method)
		key = strings.ReplaceAll(key, "/", ".")
		key = strings.ReplaceAll(key, "{{", "")
		key = strings.ReplaceAll(key, "}}", "")

		if validate.Exists(key) {
			route.validations = []string{key}
			ui.Log(ui.RouteLogger, "route.validation", ui.A{
				"key":    key,
				"route":  endpoint,
				"method": method})
		}
	}

	// Routes are stored using an object that describes both the endpoint and the method
	index := routeSelector{
		endpoint: endpoint,
		method:   method,
	}

	// This should never happen and indicates a fatal error if we are defining the same
	// endpoint and method more than once.
	if _, found := m.routes[index]; found {
		ui.Panic(fmt.Errorf("attempt to create new duplicate route definition %v", index))
	}

	m.routes[index] = route

	return route
}

// Disallowed determines if the session parameters include any disallowed combinations
// based on the route specfiication. If there are no conflicting combinations, the
// function returns nil. Otherwise, it returns an error indicating the conflict. Note
// that this will return the first conflict found, not all possible conflicts in the
// session.
func (r *Route) Disallowed(session *Session) error {
	for key, list := range r.disallow {
		if _, found := session.Parameters[strings.ToLower(key)]; found {
			for _, disallowed := range list {
				if _, found := session.Parameters[strings.ToLower(disallowed)]; found {
					return errors.ErrParameterConflict.Clone().Context(key + ", " + disallowed)
				}
			}
		}
	}

	return nil
}

// Disallow specifies a disallowed combination of parameters for this route. The
// parameter is a string with the following format:
//
//	"parm0: parm1, parm2,..."
//
// For the parameter named 'parm0', the comma-separated list of disallowed other
// parameters is specfieed. There must be at least one disallowed parameter for each
// specification.
func (r *Route) Disallow(specification string) *Route {
	if r != nil {
		if r.disallow == nil {
			r.disallow = make(map[string][]string)
		}

		parts := strings.SplitN(specification, ":", 2)
		if len(parts) != 2 {
			ui.Log(ui.RouteLogger, "route.disallow.invalid", ui.A{
				"route": r.endpoint,
				"parms": specification})

			return r
		}

		parms := strings.Split(parts[1], ",")
		if len(parms) > 0 {
			list := []string{}
			for _, parm := range parms {
				list = append(list, strings.ToLower(strings.TrimSpace(parm)))
			}

			r.disallow[strings.TrimSpace(parts[0])] = list
		}
	}

	return r
}

// AllowRedirects specifies whether or not redirects are allowed for this route, such
// that a call via the insecure scheme is redirected to the secure scheme.
func (r *Route) AllowRedirects(allow bool) *Route {
	if r != nil {
		r.allowRedirects = allow
	}

	return r
}

// IsRedirectAllowed returns true if redirects are allowed for this route.
func (r *Route) IsRedirectAllowed() bool {
	if r != nil {
		return r.allowRedirects
	}

	return false
}

// Set the flag indicating this is expected to return a large response body.
func (r *Route) LargeResponse() *Route {
	if r != nil {
		r.largeReponse = true
	}

	return r
}

// ValidateUsing specifies the list of validation objects thare are used to
// verify the payload of the request. If this is empty, no validation is done.
func (r *Route) ValidateUsing(validations ...string) *Route {
	if r != nil {
		r.validations = append(r.validations, validations...)
	}

	return r
}

// Validations gets the list of validation objects used to verify the payload of the request.
func (r *Route) Validations() []string {
	if r != nil {
		return r.validations
	}

	return nil
}

// IsLargeResponse returns true if this route expects a large response body.
func (r *Route) IsLargeResponse() bool {
	if r != nil {
		return r.largeReponse
	}

	return false
}

// Permissions specifies one or more user permissions that are required for the authenticated
// user to be able to access the endpoint.
func (r *Route) Permissions(permissions ...string) *Route {
	if r != nil {
		r.mustAuthenticate = true
		r.allowRedirects = false

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

// AcceptMedia specifies one or more user media types that are required for endpoint
// validation. If no media types are assigned, then all media types are accepted.
func (r *Route) AcceptMedia(mediaTypes ...string) *Route {
	if r != nil {
		if r.acceptMediaTypes == nil {
			r.acceptMediaTypes = []string{}
		}

		for _, mediaType := range mediaTypes {
			duplicate := false

			for _, expectedMediaType := range r.acceptMediaTypes {
				if expectedMediaType == mediaType {
					duplicate = true

					break
				}
			}

			if !duplicate {
				r.acceptMediaTypes = append(r.acceptMediaTypes, mediaType)
			}
		}
	}

	return r
}

// AcceptMedia specifies one or more user media types that are required for endpoint
// validation. If no media types are assigned, then all media types are accepted.
func (r *Route) ContentMedia(mediaTypes ...string) *Route {
	if r != nil {
		if r.contentMediaTypes == nil {
			r.contentMediaTypes = []string{}
		}

		for _, mediaType := range mediaTypes {
			duplicate := false

			for _, expectedMediaType := range r.contentMediaTypes {
				if expectedMediaType == mediaType {
					duplicate = true

					break
				}
			}

			if !duplicate {
				r.acceptMediaTypes = append(r.contentMediaTypes, mediaType)
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
		ui.Panic("invalid route parameter validation type: " + kind)
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
// status. Note that any route that requires authentication will be
// marked as not allowing automatic redirection from HTTP to HTTPS,
// since that would imply transmission of credentials in plain text.
// The intent is to catch those users immediately with an unsuupported
// request error.
//
// If these are not set, they are not checked. But if they are set, the
// router will return suitable HTTP status without calling the handler.
func (r *Route) Authentication(valid, administrator bool) *Route {
	if r != nil {
		r.mustAuthenticate = valid || administrator
		r.mustBeAdmin = administrator
		r.allowRedirects = !(valid || administrator)
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

// For a given method and path, find the most appropriate route from which to
// invoke a handler.
//
// The function returns the route found, and any HTTP status value that might
// arrise from validating the request. If the status value is not StatusOK, it
// means one ore more validations failed and the route pointer is typically nil.
func (m *Router) FindRoute(method, path string) (*Route, int) {
	candidates := []*Route{}
	method = strings.ToUpper(method)

	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/") + "/"
	}

	ui.Log(ui.RouteLogger, "route.search", ui.A{
		"method": method,
		"path":   path})

	// Find the best match for this path. This includes cases where there is a
	// pattern that helps us match up. Make a list of possible routes in the
	// candidates array.
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

		// If the endpoint definitionm for this route includes substitutions,
		// then convert the user-supplied path to match so we can detect if this
		// URL matches the pattern of the route.
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

	// Based on the length of the candidate list, return the best route found.
	switch len(candidates) {
	case 0:
		ui.Log(ui.RouteLogger, "route.search.none", nil)

		return nil, http.StatusNotFound

	case 1:
		// We only found one, but let's make sure that if the route method either
		// accepts all methods, or it matches the method we're using for this request.
		ui.Log(ui.RouteLogger, "route.search.one", ui.A{
			"endpoint": candidates[0].endpoint})

		route := candidates[0]
		if route.method == AnyMethod || strings.EqualFold(route.method, method) {
			return route, http.StatusOK
		}

		// Method didn't match up, so not found...
		return nil, http.StatusMethodNotAllowed

	default:
		ui.Log(ui.RouteLogger, "route.search.multiple", ui.A{
			"count": len(candidates)})

		for _, candidate := range candidates {
			ui.Log(ui.RouteLogger, "route.search.candidate", ui.A{
				"endpoint": candidate.endpoint})
		}

		// Find the candidate with the exact match, if any.
		for _, candidate := range candidates {
			if candidate.endpoint == path {
				ui.Log(ui.RouteLogger, "route.search.match.exact", ui.A{
					"endpoint": candidate.endpoint})

				return candidate, http.StatusOK
			}
		}

		// If there is one that has no variables, let's use that one (this lets
		// priority to /tables/@sql over /tables/{{name}} for example.)
		var fewestVariables *Route

		minCount := 100
		maxCount := 0

		for _, candidate := range candidates {
			// If there are no variable fields in the URL, choose this one first.
			if !strings.Contains(candidate.endpoint, "{{") && !strings.Contains(candidate.endpoint, "}}") {
				ui.Log(ui.RouteLogger, "route.serach.match.novars", ui.A{
					"endpoint": candidate.endpoint})

				return candidate, http.StatusOK
			}

			// Count the number of variables in the URL. We'll use this later
			// if we end up only with routes that have variables...
			variableCount := strings.Count(candidate.endpoint, "{{")
			if variableCount < minCount {
				minCount = variableCount
				fewestVariables = candidate
			}

			if variableCount > maxCount {
				maxCount = variableCount
			}
		}

		// If after that, one of the routes had fewer variables, use that one.
		if maxCount > minCount {
			ui.Log(ui.RouteLogger, "route.search.match.vars", ui.A{
				"endpoint": fewestVariables.endpoint,
				"min":      minCount,
				"max":      maxCount})

			return fewestVariables, http.StatusOK
		}

		// Is there a match with the exact same number of URL parts? If so, use that.
		pathPartCount := strings.Count(path, "/")

		for _, candidate := range candidates {
			routePartCount := strings.Count(candidate.endpoint, "/")
			if !strings.HasSuffix(candidate.endpoint, "/") {
				routePartCount++
			}

			if pathPartCount == routePartCount {
				return candidate, http.StatusOK
			}
		}

		// Not sure, so use the candidate with the longest path, assuming is the most complete
		// specification of the user's intent.
		longest := 0
		for index := 1; index < len(candidates); index++ {
			if len(candidates[index].endpoint) > len(candidates[longest].endpoint) {
				longest = index
			}
		}

		return candidates[longest], http.StatusOK
	}
}

// If the ROUTE logger is active, dump out the router map. This is used for debugging purposes
// and puts the output in the server log.
func (m *Router) Dump() {
	if !ui.IsActive(ui.RouteLogger) {
		return
	}

	ui.Log(ui.RouteLogger, "route.dump.header", ui.A{
		"name":  m.name,
		"count": len(m.routes)})

	// Make an array that contains the endpoint and methods as a single string, and sort the
	// keys so the output is easier to read.
	keys := []string{}
	for route := range m.routes {
		keys = append(keys, route.endpoint+" "+route.method)
	}

	sort.Strings(keys)

	// Dump out each route's information.
	for _, key := range keys {
		parts := strings.Fields(key)
		selector := routeSelector{method: parts[1], endpoint: parts[0]}
		route := m.routes[selector]

		fn := runtime.FuncForPC(reflect.ValueOf(route.handler).Pointer()).Name()

		// This trims off some of the noisy prefix to function names returned from the
		// reflection system to meke the handler name more readable.
		for _, prefix := range []string{"github.com/tucats/ego/", "http/", "tables/"} {
			fn = strings.TrimPrefix(fn, prefix)
		}

		ui.Log(ui.RouteLogger, "route.dump", ui.A{
			"method":   selector.method,
			"endpoint": selector.endpoint,
			"file":     route.filename,
			"media":    route.acceptMediaTypes,
			"admin":    route.mustBeAdmin,
			"auth":     route.mustAuthenticate,
			"perms":    route.requiredPermissions,
		})
	}
}
