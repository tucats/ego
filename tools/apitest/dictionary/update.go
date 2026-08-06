package dictionary

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/apitest/logging"
	"github.com/tucats/apitest/parser"
)

// Update will updated (or add) items in the dictionary from the text
// of the response body, which is assumed to be a valid JSON object, or
// from the actual response headers. For each item in the map, the key is
// used as the name of the item to add or update the item in the dictionary.
// The value of the key is normally a dot-notation string that specifies the
// item to extract from the JSON response body object. A value starting with
// "header:" instead extracts from headers -- see extractFromHeader.
func Update(text string, headers map[string][]string, items map[string]string) error {
	for key, value := range items {
		var (
			item string
			err  error
		)

		if rest, ok := strings.CutPrefix(value, "header:"); ok {
			item, err = extractFromHeader(headers, rest)
		} else {
			item, err = parser.GetOneItem(text, value)
		}

		if err != nil {
			return err
		}

		Dictionary[key] = item

		if logging.Verbose {
			if key == "API_TOKEN" {
				item = "***REDACTED***"
			}

			fmt.Printf("  Updating   {{%s}} = %s\n", key, item)
		}
	}

	return nil
}

// extractFromHeader implements the "header:Name" and "header:Name:param" forms of a
// Save value. spec is everything after the "header:" prefix. Four forms are recognized:
//
//   - "Name" -- the header's raw value.
//   - "Name:cookie" -- Name must be "Set-Cookie"; returns just the "cookie=value" pair
//     (the first ";"-delimited segment), stripping attributes like Path/HttpOnly/SameSite
//     that are only meaningful in a Set-Cookie response header and would corrupt a
//     request's Cookie header if sent back verbatim. This is how a test replays a
//     CSRF-protection cookie (e.g. Ego's OAuth2 AS login form) from a GET response into a
//     subsequent POST's Cookie header, since apitest's HTTP client has no cookie jar of
//     its own and does not carry cookies between requests automatically.
//   - "Name:value" -- like "cookie" above, but returns only the bare cookie value (no
//     "name=" prefix) -- for embedding directly into a form field, such as the AS login
//     form's own "csrf_token" input, that expects the raw token rather than "name=value".
//   - "Name:param" (any other param) -- the header value is parsed as a URL and the named
//     query string parameter is returned. This is how an OAuth2 Authorization Code flow
//     test captures "code" and "state" out of a redirect's Location header, values that
//     never appear in a JSON response body at all.
func extractFromHeader(headers map[string][]string, spec string) (string, error) {
	name, param, hasParam := strings.Cut(spec, ":")

	values, ok := lookupHeader(headers, name)
	if !ok || len(values) == 0 {
		return "", fmt.Errorf("no such response header: %s", name)
	}

	raw := values[0]

	if !hasParam {
		return raw, nil
	}

	if param == "cookie" || param == "value" {
		pair, _, _ := strings.Cut(raw, ";")
		pair = strings.TrimSpace(pair)

		if param == "cookie" {
			return pair, nil
		}

		_, value, _ := strings.Cut(pair, "=")

		return value, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("header %s is not a valid URL: %v", name, err)
	}

	if !u.Query().Has(param) {
		return "", fmt.Errorf("query parameter %s not found in header %s", param, name)
	}

	return u.Query().Get(param), nil
}

// lookupHeader finds a header by name, ignoring case (HTTP header names are
// case-insensitive, but the map preserves whatever casing the server sent).
func lookupHeader(headers map[string][]string, name string) ([]string, bool) {
	for key, values := range headers {
		if strings.EqualFold(key, name) {
			return values, true
		}
	}

	return nil, false
}
