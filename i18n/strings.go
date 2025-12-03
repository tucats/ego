// Package i18n provides localization and internationalization
// functionality for Ego itself.
package i18n

// This generates the messages.go file that contains the messages map,
// using the language files
//go:generate go run ../tools/lang/ -c -p languages -s messages.go

import (
	"os"
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
)

// Language is a string that identifies the current language, such as
// "en" for English or "fr" for French. This is used as a key in the
// internal localization dictionaries.
var Language string

// T returns the translated string for the given key and language. If
// the language is not set, it will try to get it from the EGO_LANG or
// LANG environment variables. If the key is not found in the current
// language, it will try to find it in the "en" language. If it is not
// found in either language, it will return the key itself. If the optional
// valueMap is provided, it will replace any tags in the string with the
// corresponding values.
func T(key string, valueMap ...map[string]any) string {
	// If we haven't yet figure out what language, do that now.
	if Language == "" {
		Language = os.Getenv(defs.EgoLangEnv)
		if Language == "" {
			Language = os.Getenv("LANG")
		}

		if len(Language) > 2 {
			Language = Language[0:2]
		}
	}

	// Find the message using the current language
	text, ok := messages[key][Language]
	if !ok {
		text, ok = messages[key]["en"]
		if !ok {
			text = key
		}
	}

	if len(valueMap) > 1 {
		return "@@@ INCORRECT USAGE WITH MULTIPLE VALUE MAPS @@@"
	}

	if len(valueMap) > 0 {
		text = egostrings.SubstituteMap(text, valueMap[0])
	}

	return text
}

// ofType returns a localized string for the given prefix, key and valueMap.
// The prefix is used to identify the translation domain, and the key is the
// specific string to be translated. The valueMap is an optional set of key/value
// pairs that can be used to substitute values into the translated string.
func ofType(prefix, key string, valueMap ...map[string]any) string {
	prefix = prefix + "."
	m := T(prefix+key, valueMap...)

	return strings.TrimPrefix(m, prefix)
}

// L returns a label with the given key.
func L(key string, valueMap ...map[string]any) string {
	return ofType("label", key, valueMap...)
}

// M returns a message with the given key.
func M(key string, valueMap ...map[string]any) string {
	return ofType("msg", key, valueMap...)
}

// E returns an error with the given key.
func E(key string, valueMap ...map[string]any) string {
	return ofType("error", key, valueMap...)
}
