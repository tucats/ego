package data

import (
	"strings"
	"unicode"
)

// For any given _Ego_ object type, remove any metadata from it
// and return a sanitized version. For scalar types like int,
// bool, or string, there is no operation performed and the
// object is returned unchanged. For Struct and Map types, the
// response is always a map[string]any. For Array types,
// this will always be an []any structure. This can then
// be used serialized to JSON to send HTTP response
// bodies, for example.
func Sanitize(v any) any {
	switch v := v.(type) {
	case *Array:
		return v.data

	case *Struct:
		return v.fields

	case *Map:
		result := map[string]any{}
		keys := v.Keys()

		for _, key := range keys {
			if keyString, ok := key.(string); ok {
				if strings.HasPrefix(keyString, MetadataPrefix) {
					continue
				}
			}

			value, _, _ := v.Get(key)
			result[String(key)] = Sanitize(value)
		}

		return result

	case *Package:
		result := map[string]any{}

		for _, key := range v.Keys() {
			value, _ := v.Get(key)
			if !strings.HasPrefix(key, MetadataPrefix) {
				result[key] = Sanitize(value)
			}
		}

		return result

	// For anything else, just return the thing we were given.
	default:
		return v
	}
}

// SanitizeName is used to examine a string that is used as a name (a filename,
// a module name, etc.). The function will ensure it has no embedded characters
// that would either reformat a string inappropriately -- such as entries in a
// log -- or allow any kind of unwanted injection.
//
// The function converts all control characters that could affect line ending or
// spacing to a "." character. It also processes other selection punctuation that
// is not allowed in an Ego name.
func SanitizeName(name string) string {
	result := strings.Builder{}
	blackList := []rune{'$', '\\', '/', '.', ';', ':'}

	for _, ch := range name {
		if unicode.IsSpace(ch) {
			ch = '.'
		} else {
			for _, badCh := range blackList {
				if ch == badCh {
					ch = '.'

					break
				}
			}
		}

		result.WriteRune(ch)
	}

	return result.String()
}
