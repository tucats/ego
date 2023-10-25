package main

import (
	"encoding/base64"
	"strings"
)

// encode converts a byte slice to a Go string constant containing a base64 representation
// of the data.
func encode(data []byte) string {
	text := base64.StdEncoding.EncodeToString(data)

	b := strings.Builder{}

	b.WriteRune('`')

	for _, ch := range text {
		if b.Len()%60 == 0 {
			b.WriteString("\n")
		}

		b.WriteRune(ch)
	}

	b.WriteRune('`')

	return b.String()
}
