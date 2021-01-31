package cli

import (
	"strings"
)

// ValidKeyword does a case-insensitive compare of a string containing
// a keyword against a list of possible stirng values.
func ValidKeyword(test string, valid []string) bool {

	for _, v := range valid {
		if strings.EqualFold(test, v) {
			return true
		}
	}
	return false
}

// FindKeyword does a case-insensitive compare of a string containing
// a keyword against a list of possible string values. If the keyword
// is found, it's position in the list is returned. If it was not found,
// the value returned is -1
func FindKeyword(test string, valid []string) int {

	for n, v := range valid {
		if strings.EqualFold(test, v) {
			return n
		}
	}
	return -1
}

// ValidateBoolean tests to see if a string value contains a
// legitimate boolean value. The first return is the boolean
// value, and the second indicates if it was valid.
func ValidateBoolean(value string) (bool, bool) {
	valid := false
	for _, x := range []string{"1", "true", "t", "yes", "y"} {
		if strings.ToLower(value) == x {
			return true, true
		}
	}

	if !valid {
		for _, x := range []string{"0", "false", "f", "no", "n"} {
			if strings.ToLower(value) == x {
				return false, true
			}
		}
	}
	return false, false
}

// MakeList takes a string containing a comma-separated list of
// string values and converts it to an array of trimmed strings.
func MakeList(value string) []string {
	if len(strings.TrimSpace(value)) == 0 {
		return []string{}
	}
	list := strings.Split(value, ",")
	for n := 0; n < len(list); n++ {
		list[n] = strings.TrimSpace(list[n])
	}
	return list
}

// FindGlobal locates the top-most context structure in the chain
// of nested contexts. If our tree has no parent, we are the
// global grammar. Otherwise, ask our parent...
func (c *Context) FindGlobal() *Context {
	if c.Parent == nil {
		return c
	}
	return c.Parent.FindGlobal()

}
