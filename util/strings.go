package util

import (
	"os"
	"unicode"
)

func HasCapitalizedName(name string) bool {
	var firstRune rune

	for _, ch := range name {
		firstRune = ch

		break
	}

	return unicode.IsUpper(firstRune)
}

func Hostname() string {
	if hostName, err := os.Hostname(); err == nil {
		result := ""

		for _, ch := range hostName {
			if ch == '.' {
				break
			}

			result = result + string(ch)
		}

		return result
	} else {
		return "<unknown hostname>"
	}
}
