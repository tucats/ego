package util

import (
	"os"
	"sort"
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

// InList is a support function that checks to see if a string matches
// any of a list of other strings.
func InList(s string, test ...string) bool {
	for _, t := range test {
		if s == t {
			return true
		}
	}

	return false
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
