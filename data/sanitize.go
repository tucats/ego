package data

import (
	"strings"
)

// For any given _Ego_ object type, remove any metadata from it
// and return a sanitized version. For scalar types like int,
// bool, or string, there is no operation performed and the
// object is returned unchanged. For Struct and Map types, the
// response is always a map[string]interface{}. For Array types,
// this will always be an []interface{} structure. This can then
// be used serialized to JSON to send HTTP response
// bodies, for example.
func Sanitize(v interface{}) interface{} {
	switch v := v.(type) {
	case *Array:
		return v.data

	case *Struct:
		return v.fields

	case *Map:
		result := map[string]interface{}{}
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
		result := map[string]interface{}{}

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
