package io

import (
	"strings"

	"github.com/tucats/ego/util"
)

// Pad the formatted value of a given object to the specified number
// of characters. Negative numbers are right-aligned, positive numbers
// are left-aligned.
func Pad(v interface{}, w int) string {
	s := util.FormatUnquoted(v)
	count := w

	if count < 0 {
		count = -count
	}

	padString := ""
	if count > len(s) {
		padString = strings.Repeat(" ", count-len(s))
	}

	var r string

	if w < 0 {
		r = padString + s
	} else {
		r = s + padString
	}

	if len(r) > count {
		r = r[:count]
	}

	return r
}
