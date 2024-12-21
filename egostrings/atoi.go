package egostrings

import (
	"strconv"
	"strings"

	"github.com/tucats/ego/errors"
)

// Replacement for strconv.Atoi that undestands hexadecimal, octal, and binary
// constants as well.
func Atoi(s string) (int, error) {
	var (
		err error
		v   int64
		vi  int
	)

	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasPrefix(s, "0x") {
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
		err = errors.ErrInvalidInteger.Context(s)
	}

	return int(v), err
}
