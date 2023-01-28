package rest

import (
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	runtime_strings "github.com/tucats/ego/runtime/strings"
	"github.com/tucats/ego/symbols"
)

// utility function that prepends the base URL for this instance
// of a rest service to the supplied URL string. If there is
// no base URL defined, then nothing is changed.
func applyBaseURL(url string, this *data.Struct) string {
	if b := this.GetAlways(baseURLFieldName); b != nil {
		base := data.String(b)
		if base == "" {
			return url
		}

		base = strings.TrimSuffix(base, "/")

		if !strings.HasPrefix(url, "/") {
			url = "/" + url
		}

		url = base + url
	}

	return url
}

func ParseURL(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	urlString := data.String(args[0])

	url, err := url.Parse(urlString)
	if err != nil {
		return nil, errors.NewError(err).Context(urlString)
	}

	hasSchema := strings.Contains(urlString, "://")
	urlParts := map[string]interface{}{}

	// If the second parameter was provided, it's a template string. Use it to parse
	// apart the path components of the url.
	if len(args) > 1 {
		var valid bool

		path := url.Path
		templateString := data.String(args[1])

		// Scan the URL and the template, and bulid a map of the parts.
		urlParts, valid = runtime_strings.ParseURLPattern(path, templateString)
		if !valid {
			return nil, errors.ErrInvalidURL.Context(path)
		}
	}

	// Store parsed parts based on the parsed URL. Empty elements are not
	// reported in the string. This has to be done after the above because
	// otherwise the template parser will re-initialize the hash map of parts.

	// Clunky, but... if there was no scheme in the original URL string, then
	// the URL parser will have assigned the hostname as the scheme. If there
	// was a proper scheme, then the host is the hostname as expected.
	if !hasSchema && url.Scheme != "" {
		urlParts[urlHostElement] = url.Scheme
	} else if host := url.Hostname(); host != "" {
		urlParts[urlHostElement] = host
	}

	if port := url.Port(); port != "" {
		urlParts["urlPort"] = port
	}

	// Note that if there was no schema in the original URL, then we don't
	// have a schema. Otherwise, record any non-empty schema
	if schema := url.Scheme; hasSchema && schema != "" {
		urlParts[urlSchemeElement] = url.Scheme
	}

	if user := url.User.Username(); user != "" {
		urlParts[urlUsernameElement] = user
	}

	if pw, found := url.User.Password(); found {
		urlParts[urlPasswordElement] = pw
	}

	if path := url.Path; path != "" {
		urlParts[urlPathElement] = path
	}

	if queryParts := url.Query(); len(queryParts) != 0 {
		query := map[string]interface{}{}

		for key, value := range queryParts {
			values := make([]interface{}, len(value))
			for i, j := range value {
				values[i] = j
			}

			query[key] = data.NewArrayFromArray(data.StringType, values)
		}

		urlParts[urlQueryElmeent] = data.NewMapFromMap(query)
	}

	return data.NewStructFromMap(urlParts), nil
}

// setBase implements the setBase() rest function. This specifies a string that is used
// as the base prefix for any URL formed in a REST call. This lets you specify the
// protocol/host/port information once, and then have each Get(), Post(), etc. call
// just specify the endpoint.
func setBase(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	_, err := getClient(s)
	if err != nil {
		return nil, err
	}

	this := getThis(s)
	base := ""

	if len(args) > 0 {
		base = data.String(args[0])
	} else {
		base = settings.Get(defs.LogonServerSetting)
	}

	this.SetAlways(baseURLFieldName, base)

	return this, nil
}
