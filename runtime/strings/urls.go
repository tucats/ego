package strings

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// URLPattern parses a URL using a provided pattern for what the string
// is expected to be, and then generates an Ego map that indicates each
// segment of the url endopint and it's value.
//
// If the pattern is
//
//	"/services/debug/processes/{{ID}}"
//
// and the url is
//
//	/services/debug/processses/1653
//
// Then the result map will be
//
//	map[string]interface{} {
//	       "ID" : 1653
//	}
func URLPattern(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	result := data.NewMap(data.StringType, data.InterfaceType)

	patternMap, match := ParseURLPattern(data.String(args.Get(0)), data.String(args.Get(1)))
	if !match {
		return result, nil
	}

	for k, v := range patternMap {
		_, err := result.Set(k, v)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

// ParseURLPattern accepts a pattern that tells what part of the URL is
// meant to be literal, and what is a user-supplied item. The result is
// a map of the URL items parsed.
//
// If the pattern is
//
//	"/services/debug/processes/{{ID}}"
//
// and the url is
//
//	/services/debug/processses/1653
//
// Then the result map will be
//
//	map[string]interface{} {
//	       "ID" : 1653
//	}
func ParseURLPattern(url, pattern string) (map[string]interface{}, bool) {
	urlParts := strings.Split(url, "/")
	patternParts := strings.Split(pattern, "/")
	result := map[string]interface{}{}

	if len(urlParts) > len(patternParts) {
		return nil, false
	}

	for idx, pat := range patternParts {
		if len(pat) == 0 {
			continue
		}

		// If the pattern continues longer than the
		// URL given, mark those as being absent
		if idx >= len(urlParts) {
			// Is this part of the pattern a substitution? If not, we store
			// it in the result as a field-not-found. If it is a substitution
			// operator, store as an empty string.
			if !strings.HasPrefix(pat, "{{") || !strings.HasSuffix(pat, "}}") {
				result[pat] = false
			} else {
				name := strings.Replace(strings.Replace(pat, "{{", "", 1), "}}", "", 1)
				result[name] = ""
			}

			continue
		}

		// If this part just matches, mark it as present.
		if strings.EqualFold(pat, urlParts[idx]) {
			result[pat] = true

			continue
		}

		// If this pattern is a substitution operator, get the value now
		// and store in the map using the substitution name
		if strings.HasPrefix(pat, "{{") && strings.HasSuffix(pat, "}}") {
			// Strip off the {{ }} from the name we going to save it as.
			name := strings.Replace(strings.Replace(pat, "{{", "", 1), "}}", "", 1)
			// Put it in the result using that key and the original value from the URL.
			result[name] = urlParts[idx]
		} else {
			// It didn't match the url, so no data
			return nil, false
		}
	}

	return result, true
}
