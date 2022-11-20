package i18n

import (
	"fmt"
	"os"
	"strings"
)

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
	text, ok := Messages[Language][key]
	if !ok {
		text, ok = Messages["en"][key]
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
