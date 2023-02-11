// Package i18n provides localization and internationalization
// functionality for Ego itself.
package i18n

import (
	"fmt"
	"os"
	"strings"
)

// Language is a string that identifies the current language, such as
// "en" for English or "fr" for French. This is used as a key in the
// internal localization dictionaries.
var Language string

func T(key string, valueMap ...map[string]interface{}) string {
	// If we haven't yet figure out what language, do that now.
	if Language == "" {
		Language = os.Getenv("EGO_LANG")
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

	if len(valueMap) > 0 {
		for tag, value := range valueMap[0] {
			text = strings.ReplaceAll(text, "{{"+tag+"}}", fmt.Sprintf("%v", value))
		}
	}

	return text
}

func ofType(prefix, key string, valueMap ...map[string]interface{}) string {
	prefix = prefix + "."
	m := T(prefix+key, valueMap...)
	return strings.TrimPrefix(m, prefix)
}

// L returns a label with the given key.
func L(key string, valueMap ...map[string]interface{}) string {
	return ofType("label", key, valueMap...)
}

// M returns a message with the given key.
func M(key string, valueMap ...map[string]interface{}) string {
	return ofType("msg", key, valueMap...)
}

// E returns an error with the given key.
func E(key string, valueMap ...map[string]interface{}) string {
	return ofType("error", key, valueMap...)
}
