package main

import (
	"strconv"
	"strings"
)

// encode converts a byte slice to a Go string constant containing a base64 representation
// of the data.
func encode(data []byte) string {
	b := strings.Builder{}
	b.WriteString("[]byte{\n\t")

	lineLength := 0

	for _, ch := range data {
		if lineLength > 72 {
			b.WriteString("\n\t")

			lineLength = 0
		}

		text := strconv.FormatInt(int64(ch), 10) + ", "
		lineLength += len(text)

		b.WriteString(text)
	}

	if lineLength > 0 {
		b.WriteRune('\n')
	}

	b.WriteRune('}')

	return b.String()
}
