package datatypes

import (
	"strings"
)

// For any given _Ego_ object type, remove any metadata from it
// and return a sanitized copy. This is used to send HTTP response
// values, for example.
func Sanitize(v interface{}) interface{} {
	switch v := v.(type) {
	case *EgoArray:
		return v.data

	case *EgoStruct:
		return v.fields

	case *EgoMap:
		result := map[string]interface{}{}
		keys := v.Keys()

		for _, key := range keys {
			if keyString, ok := key.(string); ok {
				if strings.HasPrefix(keyString, MetadataPrefix) {
					continue
				}
			}

			value, _, _ := v.Get(key)
			result[GetString(key)] = Sanitize(value)
		}

		return result

	case EgoPackage:
		result := map[string]interface{}{}

		for key, value := range v {
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
