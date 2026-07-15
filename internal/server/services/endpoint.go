package services

import (
	"net/http"
	"os"
	"strings"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/tokenizer"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/util"
)

// endpointSpec holds the fully-parsed result of an @endpoint directive.
type endpointSpec struct {
	// Path is the route's URL pattern, e.g. "/services/factor/{{value}}".
	Path string

	// Method is the HTTP method the route responds to, e.g. http.MethodGet,
	// or router.AnyMethod if no method term was given.
	Method string

	// MediaTypes is the list of media types this route will respond with
	// (wired to route.AcceptMedia), or nil if no media= term was given.
	MediaTypes []string

	// Permissions is the list of user permissions required to access this
	// route (wired to route.Permissions), or nil if neither permissions=
	// nor the admin/root shorthand was given.
	Permissions []string

	// Parameters maps query parameter name to its validation kind (wired to
	// route.Parameter), built from one or more parameter= terms.
	Parameters map[string]string

	// Authenticated is true when the bare "authenticated" term was given,
	// requiring a valid (non-admin) authenticated session.
	Authenticated bool

	// Admin is true when the bare "admin" or "root" term was given. Unlike
	// Authenticated, this requires the caller to specifically be an admin
	// (route.Authentication(true, true)) *and* contributes the "ego.root"
	// permission to Permissions -- matching the legacy "@authenticated
	// admin" directive's behavior exactly, since admin/root is its
	// intended replacement.
	Admin bool
}

// endpointMethods maps the lowercase spelling of each bare method term
// @endpoint accepts to the actual HTTP method string Router.New() expects.
// This mirrors (and is intentionally a subset of) the method set
// Router.New() itself validates against in internal/router/router.go --
// "UPDATE" is not a standard net/http constant, so it's spelled out.
var endpointMethods = map[string]string{
	"get":    http.MethodGet,
	"put":    http.MethodPut,
	"delete": http.MethodDelete,
	"patch":  http.MethodPatch,
	"post":   http.MethodPost,
	"update": "UPDATE",
}

// endpointParameterKinds is the set of parameter kind strings accepted by a
// parameter="name:kind" term, matching exactly what router.Route.Parameter
// validates against (see internal/router/router.go and internal/util/urls.go).
// This parser validates against the same list *before* ever calling
// Parameter(), since that function calls ui.Panic (a process exit) on an
// unrecognized kind -- a malformed single service file must never be able to
// take down the whole server that way.
var endpointParameterKinds = map[string]bool{
	defs.Any:                       true,
	util.DurationParameterType:     true,
	util.StringOrFlagParameterType: true,
	util.FlagParameterType:         true,
	util.BoolParameterType:         true,
	util.IntParameterType:          true,
	util.StringParameterType:       true,
	util.ListParameterType:         true,
}

// parseEndpoint scans filename for a leading @endpoint directive and parses
// it using the term grammar documented in docs/SERVER.md:
//
//	@endpoint [method] [path=]"/path/{param}" [media="type"[,"type"]]
//	          [permissions="perm"[,"perm"]] [parameter="name:kind"[,"name:kind"]]
//	          [authenticated|admin|root]
//
// Three outcomes are possible:
//
//   - (nil, nil): no @endpoint directive is present at all. The caller
//     should fall back to its own default, file-location-derived path --
//     this is unchanged, pre-existing behavior for files with no directive.
//   - (nil, err): an @endpoint directive is present but malformed. The
//     caller should log the error (see ErrInvalidEndPointDefinition) and
//     skip registering a route for this one file, without aborting the
//     rest of the directory scan.
//   - (spec, nil): success.
//
// The @authenticated directive is parsed separately by parseAuthenticated,
// unaffected by this function.
func parseEndpoint(filename string) (*endpointSpec, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil
	}

	t := tokenizer.New(string(b), true)

	// @endpoint must be the first statement in the file. Skip past any
	// leading blank statements (semicolons the tokenizer may have inserted
	// for blank/comment-only lines) before checking.
	for t.IsNext(tokenizer.SemicolonToken) {
	}

	if !t.Peek(1).Is(tokenizer.DirectiveToken) || t.Peek(2).Spelling() != "endpoint" {
		return nil, nil
	}

	t.Advance(2)

	return parseEndpointTerms(t)
}

// parseEndpointTerms parses the term list following "@endpoint" (with the
// directive and its name already consumed) until end of statement.
func parseEndpointTerms(t *tokenizer.Tokenizer) (*endpointSpec, error) {
	if t.EndOfStatement() {
		return nil, endpointError(t, "missing endpoint specification")
	}

	var (
		spec           = &endpointSpec{Parameters: map[string]string{}}
		pathSet        bool
		methodSet      bool
		mediaSet       bool
		permissionsSet bool
		authTermSet    bool
	)

	for !t.EndOfStatement() {
		next := t.Peek(1)

		switch {
		case next.IsString():
			// Legacy bare-string path, e.g. @endpoint "GET /services/foo?n=int".
			// Only valid if path= wasn't already given.
			if pathSet {
				return nil, endpointError(t, "unexpected string; use path=")
			}

			t.Advance(1)

			legacyPath, legacyMethod, legacyParams, err := parseLegacyPathString(next.Spelling())
			if err != nil {
				return nil, err
			}

			spec.Path = legacyPath
			pathSet = true

			if legacyMethod != "" {
				if methodSet {
					return nil, endpointError(t, "duplicate method term")
				}

				spec.Method = legacyMethod
				methodSet = true
			}

			for name, kind := range legacyParams {
				if _, exists := spec.Parameters[name]; exists {
					return nil, endpointError(t, "duplicate parameter name: "+name)
				}

				spec.Parameters[name] = kind
			}

		case next.IsIdentifier():
			word := strings.ToLower(next.Spelling())
			
			t.Advance(1)

			switch {
			case endpointMethods[word] != "":
				if methodSet {
					return nil, endpointError(t, "duplicate method term: "+word)
				}

				spec.Method = endpointMethods[word]
				methodSet = true

			case word == "authenticated" || word == "admin" || word == "root":
				if authTermSet {
					return nil, endpointError(t, "duplicate authentication term: "+word)
				}

				authTermSet = true

				if word == "authenticated" {
					spec.Authenticated = true
				} else {
					spec.Admin = true
					spec.Permissions = append(spec.Permissions, defs.RootPermission)
				}

			case word == "path":
				if pathSet {
					return nil, endpointError(t, "duplicate path term")
				}

				value, err := parseSingleTermValue(t, word)
				if err != nil {
					return nil, err
				}

				spec.Path = value
				pathSet = true

			case word == "media":
				if mediaSet {
					return nil, endpointError(t, "duplicate media term")
				}

				values, err := parseTermValueList(t, word)
				if err != nil {
					return nil, err
				}

				spec.MediaTypes = values
				mediaSet = true

			case word == "permissions":
				if permissionsSet {
					return nil, endpointError(t, "duplicate permissions term")
				}

				values, err := parseTermValueList(t, word)
				if err != nil {
					return nil, err
				}

				spec.Permissions = append(spec.Permissions, values...)
				permissionsSet = true

			case word == "parameter":
				values, err := parseTermValueList(t, word)
				if err != nil {
					return nil, err
				}

				if err := addParameters(t, spec, values); err != nil {
					return nil, err
				}

			default:
				return nil, endpointError(t, "unrecognized endpoint term: "+word)
			}

		default:
			return nil, endpointError(t, "unexpected token: "+next.Spelling())
		}
	}

	if !pathSet {
		return nil, endpointError(t, "missing path")
	}

	if spec.Method == "" {
		spec.Method = router.AnyMethod
	}

	return spec, nil
}

// parseLegacyPathString implements the pre-existing (legacy) single-string
// @endpoint syntax: an optional "METHOD " prefix, followed by the path,
// optionally followed by "?name=kind&name2=kind2" parameter declarations
// jammed into the same string. This sub-syntax predates the
// path=/media=/permissions=/parameter= term grammar and is retained only for
// backward compatibility with existing service files -- new services should
// use the explicit terms instead.
func parseLegacyPathString(raw string) (path string, method string, parameters map[string]string, err error) {
	path = raw

	for word, verb := range endpointMethods {
		prefix := strings.ToUpper(word) + " "
		if strings.HasPrefix(strings.ToUpper(path), prefix) {
			method = verb
			path = strings.TrimSpace(path[len(prefix):])

			break
		}
	}

	i := strings.Index(path, "?")
	if i <= 0 {
		return path, method, nil, nil
	}

	paramDefs := path[i+1:]
	path = path[:i]
	parameters = map[string]string{}

	for _, param := range strings.Split(paramDefs, "&") {
		if strings.TrimSpace(param) == "" {
			continue
		}

		name, kind, ok := strings.Cut(param, "=")
		if !ok {
			return "", "", nil, errors.New(errors.ErrInvalidEndPointDefinition).Context("malformed parameter: " + param)
		}

		name = strings.TrimSpace(name)
		kind = strings.ToLower(strings.TrimSpace(kind))

		if !endpointParameterKinds[kind] {
			return "", "", nil, errors.New(errors.ErrInvalidEndPointDefinition).Context("unrecognized parameter kind: " + kind)
		}

		parameters[name] = kind
	}

	return path, method, parameters, nil
}

// addParameters splits each "name:kind" value from a parameter= term and adds
// it to spec.Parameters, validating the kind against endpointParameterKinds
// and rejecting a name that was already defined by an earlier parameter= term.
func addParameters(t *tokenizer.Tokenizer, spec *endpointSpec, values []string) error {
	for _, value := range values {
		name, kind, ok := strings.Cut(value, ":")
		if !ok || name == "" || kind == "" {
			return endpointError(t, "malformed parameter (want name:kind): "+value)
		}

		if !endpointParameterKinds[kind] {
			return endpointError(t, "unrecognized parameter kind: "+kind)
		}

		if _, exists := spec.Parameters[name]; exists {
			return endpointError(t, "duplicate parameter name: "+name)
		}

		spec.Parameters[name] = kind
	}

	return nil
}

// parseSingleTermValue expects the tokenizer to be positioned right after a
// keyword (e.g. "path"), and parses "= <string>", returning the string's
// value.
func parseSingleTermValue(t *tokenizer.Tokenizer, keyword string) (string, error) {
	if !t.IsNext(tokenizer.AssignToken) {
		return "", endpointError(t, keyword+"= expected")
	}

	value := t.Peek(1)
	if !value.IsString() {
		return "", endpointError(t, keyword+"= requires a string value")
	}

	t.Advance(1)

	return value.Spelling(), nil
}

// parseTermValueList expects the tokenizer to be positioned right after a
// keyword (e.g. "media"), and parses "= <string> [, <string>]*", returning
// the list of string values.
func parseTermValueList(t *tokenizer.Tokenizer, keyword string) ([]string, error) {
	if !t.IsNext(tokenizer.AssignToken) {
		return nil, endpointError(t, keyword+"= expected")
	}

	var values []string

	for {
		value := t.Peek(1)
		if !value.IsString() {
			return nil, endpointError(t, keyword+"= requires a string value")
		}

		t.Advance(1)

		values = append(values, value.Spelling())

		if !t.IsNext(tokenizer.CommaToken) {
			break
		}
	}

	return values, nil
}

// endpointError wraps ErrInvalidEndPointDefinition with a detail message and,
// when available, the current source line/column, so a malformed @endpoint
// produces a useful diagnostic in the server log.
func endpointError(t *tokenizer.Tokenizer, detail string) error {
	err := errors.New(errors.ErrInvalidEndPointDefinition).Context(detail)

	if line := t.CurrentLine(); line > 0 {
		err = err.At(line, t.CurrentColumn())
	}

	return err
}

// parseAuthenticated scans filename for a leading @authenticated directive.
// This directive is now deprecated in favor of the authenticated/admin/root
// terms on @endpoint itself (see parseEndpointTerms), but continues to work
// unchanged for backward compatibility -- both mechanisms compose if a file
// uses both.
func parseAuthenticated(filename string) (authenticate bool, admin bool) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return false, false
	}

	t := tokenizer.New(string(b), true)

	for !t.IsNext(tokenizer.EndOfTokens) {
		if t.IsNext(tokenizer.DirectiveToken) && t.NextText() == "authenticated" {
			authenticate = true
			kind := t.NextText()

			if kind == "admin" || kind == "root" || kind == "admin_root" {
				admin = true
			}

			// If the token was "none" (or was ";" which means end-of-line) then the
			// authentication is turned off.
			if kind == "none" || kind == ";" {
				authenticate = false
				admin = false
			}

			break
		}

		t.Advance(1)
	}

	return authenticate, admin
}
