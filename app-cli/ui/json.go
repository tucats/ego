package ui

import (
	"os"
	"strings"

	"github.com/tucats/ego/app-cli/parser"
	"github.com/tucats/ego/errors"
)

// Read a JSON file and return as a byte array. This strips out lines that
// are comments (starting with "#" or "//"). This function returns an error if
// the file does not exist or cannot be read.
func ReadJSONFile(filePath string) ([]byte, error) {
	// Read the contents of the file.
	b, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.New(err)
	}

	// Convert the byte buffer to an array of strings for each line of
	// text.
	lines := strings.Split(string(b), "\n")

	// Strip out comment lines that start with "#" or "//".
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "#") || strings.HasPrefix(lines[i], "//") {
			lines = append(lines[:i], lines[i+1:]...)
			i--
		}
	}

	// Now that blank lines and comments have been removed, reassemble the array of
	// strings back into a byte array, and we/re done.
	b = []byte(strings.Join(lines, "\n"))

	return b, nil
}

func JSONOutput(jsonString, queryString string) error {
	outputs, err := parser.GetItem(jsonString, queryString)
	if err != nil {
		Say("msg.json.query.error", A{"error": err.Error()})

		return err
	}

	for _, output := range outputs {
		Say(output)
	}

	return nil
}
