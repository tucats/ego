package util

import "strings"

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

// InList is a support function that checks to see if a string matches
// any of a list of other strings. This version of the test is insensitive
// to case; i.e. "root", "ROOT", and "Root" will all match a list that contains
// the word "root".
func InListInsensitive(s string, test ...string) bool {
	for _, t := range test {
		if strings.EqualFold(s, t) {
			return true
		}
	}

	return false
}
