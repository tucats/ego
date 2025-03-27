package admin

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/util"
	"github.com/tucats/ego/validate"
)

const (
	arrayPrefix = "[\n"
	arraySuffix = "\n]"
)

// Get all validation objects, or a specific one by name.
func GetValidationsHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		b, nb     []byte
		err       error
		method    string
		path      string
		entry     string
		hasMethod bool
		hasPath   bool
	)

	// If a method was given, validate it. If not give, assume POST method
	if parameters, found := session.Parameters["method"]; found {
		hasMethod = true
		method = strings.ToUpper(data.String(parameters[0]))

		if !util.InList(method, http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch) {
			err = errors.ErrInvalidKeyword.Clone().Context(method)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	} else {
		method = http.MethodPost
	}

	// If a path was given, validate it.
	if parameters, found := session.Parameters["path"]; found {
		path = data.String(parameters[0])
		hasPath = true

		if _, err := url.Parse(path); err != nil {
			err = errors.ErrInvalidURL.Clone().Context(path)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}
	}

	// If we are asking for a specific dictionary entry, get it.
	if parameters, found := session.Parameters["entry"]; found {
		if hasPath || hasMethod {
			conflict := "entry"
			if hasPath {
				conflict += ", path"
			}

			if hasMethod {
				conflict += ", method"
			}

			err = errors.ErrParameterConflict.Clone().Context(conflict)

			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}

		entry = data.String(parameters[0])
		b, err = validate.Encode(entry)
	} else {
		if len(path) == 0 {
			b, err = validate.EncodeDictionary()
		} else {
			route, status := session.Router.FindRoute(method, path)
			if status < 300 && route != nil {
				list := route.Validations()
				if len(list) > 0 {
					if len(list) == 1 {
						b, err = validate.Encode(list[0])
					} else {
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
	_, _ = w.Write(b)

	session.ResponseLength += len(b)

	return http.StatusOK
}
