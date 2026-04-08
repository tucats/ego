package admin

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
	"github.com/tucats/ego/validate"
)

// arrayPrefix and arraySuffix are the JSON fragments used to wrap multiple
// validation objects into a valid JSON array when a route has more than one
// validation entry.
const (
	arrayPrefix = "[\n"
	arraySuffix = "\n]"
)

// GetValidationsHandler is the HTTP handler for GET /admin/validations. It
// returns the JSON representation of one or more request-validation schemas
// registered with the server.
//
// The handler supports three optional query parameters that narrow the result:
//
//	method — the HTTP method to look up (GET, POST, PUT, DELETE, PATCH).
//	         Defaults to POST if not supplied.
//	path   — a URL path to look up in the router's route table.  When given,
//	         the handler resolves the route and returns its validation schemas.
//	entry  — the name of a specific entry in the validation dictionary.
//	         When given, method and path are ignored.
//
// If none of the parameters match anything, the full validation dictionary is
// returned (when path is also absent) or a 404 is returned.
func GetValidationsHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		b, nb  []byte // b accumulates the JSON output; nb holds one encoded entry
		err    error
		method string // HTTP method extracted from query parameters
		path   string // URL path extracted from query parameters
		entry  string // dictionary entry name extracted from query parameters
	)

	// session.Parameters is a map[string][]string populated by the router from
	// the request's URL query string.  Each key maps to a slice of string
	// values because a query parameter can appear multiple times
	// (e.g. ?method=GET&method=POST).
	//
	// Read the "method" parameter.  If present, validate it against the known
	// HTTP method names and upper-case it for consistency.
	if parameters, found := session.Parameters["method"]; found {
		method = strings.ToUpper(data.String(parameters[0]))

		// util.InList checks whether method is one of the listed values.
		// Any other value is rejected with 400 Bad Request.
		if !util.InList(method, http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch) {
			err = errors.ErrInvalidKeyword.Clone().Context(method)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	} else {
		// Default to POST when no method is specified; most Ego service
		// endpoints are POST-based, so this is the most common lookup.
		method = http.MethodPost
	}

	// Read the "path" parameter and validate it as a well-formed URL path.
	// url.Parse returns an error only for very malformed strings; we treat any
	// error as a 400 to give the caller early feedback.
	if parameters, found := session.Parameters["path"]; found {
		path = data.String(parameters[0])

		if _, err := url.Parse(path); err != nil {
			err = errors.ErrInvalidURL.Clone().Context(path)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	}

	// If the caller named a specific dictionary entry, encode just that entry.
	// validate.Encode returns the JSON bytes for a single named schema.
	if parameters, found := session.Parameters["entry"]; found {
		entry = data.String(parameters[0])
		b, err = validate.Encode(entry)
	} else {
		if len(path) == 0 {
			// No path and no entry → return the entire validation dictionary.
			b, err = validate.EncodeDictionary()
		} else {
			// A path was given — resolve the route in the server's router and
			// return the validation schemas attached to it.
			//
			// session.Router.FindRoute returns (route, statusCode).  A status
			// below 300 means the route was found successfully.
			route, status := session.Router.FindRoute(method, path, false)
			if status < 300 && route != nil {
				list := route.Validations()
				if len(list) > 0 {
					if len(list) == 1 {
						// Single validation entry — encode it directly.
						b, err = validate.Encode(list[0])
					} else {
						// Multiple validation entries — build a JSON array by
						// concatenating the encoded entries between "[" and "]".
						b = []byte(arrayPrefix)

						for _, validation := range list {
							nb, err = validate.Encode(validation)
							if err != nil {
								break
							}

							b = append(b, nb...)
						}

						b = append(b, []byte(arraySuffix)...)
					}
				}
			}
		}
	}

	// If nothing was found (b is still empty), construct a 404 error.
	// When looking up by path the context is "METHOD /path"; when looking up
	// by entry name the context is just the entry name.
	if len(b) == 0 {
		if entry == "" {
			path = method + " " + path
		}

		err = errors.ErrNotFound.Clone().Context(entry + path)
	}

	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusNotFound)
	}

	w.Header().Add("Content-Type", defs.ValidationDictionaryMediaType)

	// egostrings.JSONMinify strips unnecessary whitespace from the JSON before
	// writing it to the wire, saving bandwidth while keeping the in-memory
	// representation readable.
	minifiedBytes := []byte(egostrings.JSONMinify(string(b)))
	_, _ = w.Write(minifiedBytes)
	session.ResponseLength += len(minifiedBytes)

	return http.StatusOK
}
