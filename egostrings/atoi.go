package egostrings

import (
	"errors"
	"strconv"
	"strings"
)

// Replacement for strconv.Atoi that understands hexadecimal, octal, and binary
// constants as well.
func Atoi(s string) (int, error) {
	var (
		err error
		v   int64
		vi  int
	)

	s = strings.TrimSpace(strings.ToLower(s))

	// If the string contains a single-quoted rune, convert it to an integer.
	if len(s) > 1 && s[0] == '\'' && s[len(s)-1] == '\'' {
		runes := []rune(s[1 : len(s)-1])
		if len(runes) != 1 {
			return 0, errors.New("error.invalid.rune")
		}

		v = int64(runes[0])
	} else if strings.HasPrefix(s, "0x") {
		v, err = strconv.ParseInt(s[2:], 16, 64)
	} else if strings.HasPrefix(s, "0o") {
		v, err = strconv.ParseInt(s[2:], 8, 64)
	} else if strings.HasPrefix(s, "0b") {
		v, err = strconv.ParseInt(s[2:], 2, 64)
	} else {
		vi, err = strconv.Atoi(s)
		v = int64(vi)
	}

	if err != nil {
		v = 0
	}

	return int(v), err
}
