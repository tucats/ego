package util

import (
	"os"
	"sort"
	"unicode"
)

// HasCapitalizedName returns true if the first rune/character of the
// string is considered a capital letter in Unicode.
func HasCapitalizedName(name string) bool {
	var firstRune rune

	for _, ch := range name {
		firstRune = ch

		break
	}

	return unicode.IsUpper(firstRune)
}

// Hostname gets a short form of the host namme (i.e. the first part of an FQDN).
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

// Given a list of strings, convert them to a sorted list in
// Ego array format.
func MakeSortedArray(array []string) []interface{} {
	sort.Strings(array)
	result := make([]interface{}, len(array))

	for i, v := range array {
		result[i] = v
	}

	return result
}

func InterfaceMapKeys(data map[string]interface{}) []string {
	keys := make([]string, 0)

	for key := range data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func StringMapKeys(data map[string]string) []string {
	keys := make([]string, 0)

	for key := range data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
