package data

import (
	"strings"
	"unicode"
)

// Sanitize strips Ego-internal metadata from a value and returns the
// "clean" version suitable for JSON serialization or passing to code
// outside the Ego runtime.
//
// Ego's Array, Struct, Map, and Package types carry extra bookkeeping
// fields (type information, immutability counters, etc.) that make no
// sense to an external consumer.  Sanitize peels those wrappers away:
//
//   - *Array  → the raw []any slice of element values
//   - *Struct → the raw map[string]any of field values
//   - *Map    → a new map[string]any, recursively sanitized, with any
//               metadata keys (those starting with MetadataPrefix) removed
//   - *Package → a new map[string]any, metadata keys removed
//   - anything else → returned unchanged (scalar types need no stripping)
//
// The conversion is intentionally shallow for arrays and structs: the
// inner []any / map[string]any slices are returned directly, not copied.
// For maps and packages a new map is built so callers can safely mutate it.
func Sanitize(v any) any {
	switch v := v.(type) {
	case *Array:
		// Return the underlying Go slice directly.  Elements are not
		// recursively sanitized because Array already stores any values.
		return v.data

	case *Struct:
		// Return the underlying map directly.  This includes any metadata
		// fields (keys beginning with MetadataPrefix) — callers that need
		// them removed should use a Map instead.
		return v.fields

	case *Map:
		// Build a plain map[string]any from the Ego map.  We skip metadata
		// keys (which begin with MetadataPrefix, typically "__") and convert
		// each key to its string representation.  Values are recursively
		// sanitized so nested maps/structs are also unwrapped.
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
		// Same logic as *Map: build a clean map, omitting metadata keys.
		result := map[string]any{}

		for _, key := range v.Keys() {
			value, _ := v.Get(key)
			if !strings.HasPrefix(key, MetadataPrefix) {
				result[key] = Sanitize(value)
			}
		}

		return result

	default:
		// Scalar types (int, bool, string, float64, etc.) carry no metadata
		// and are returned as-is.
		return v
	}
}

// SanitizeName makes a string safe to use as a name in log messages, module
// identifiers, or other contexts where special characters could cause problems.
//
// Two categories of characters are replaced with a "." dot:
//   - Unicode whitespace (spaces, tabs, newlines, …) — these would split
//     a log line or confuse line-based parsers.
//   - A hard-coded blacklist of punctuation ($, \, /, ., ;, :) — these
//     characters either have special meaning in file paths, URLs, or Ego
//     identifiers, or could enable injection attacks in log entries.
//
// All other characters (including letters, digits, and most punctuation)
// are passed through unchanged.
func SanitizeName(name string) string {
	result := strings.Builder{}
	blackList := []rune{'$', '\\', '/', '.', ';', ':'}

	// Iterate over the Unicode code points (runes) of the string.
	// Go's "for _, ch := range string" decodes UTF-8 automatically
	// so this works correctly for non-ASCII characters.
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
