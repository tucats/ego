package parser

import "strings"

// For any payload that is really a text file, remove any lines that
// start with a "#" symbol. This is a common practice for comments
// in text files. This lets us put comments in JSON files.
func RemoveComments(data []byte) []byte {
	var result []string

	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		if !strings.HasPrefix(line, "#") {
			result = append(result, line)
		}
	}

	return []byte(strings.Join(result, "\n"))
}
