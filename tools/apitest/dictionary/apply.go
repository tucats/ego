package dictionary

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/apitest/formats"
)

// Apply applies the substitutions to the given text from the active dictionary. The
// result is returned as a new string with the substitutions applied. If there were
// no substitutions made, the original text is returned.
func Apply(text string) string {
	subs := make(map[string]interface{})

	for key, value := range Dictionary {
		// Handle special cases for values.
		switch {
		case value == "$uuid":
			value = uuid.New().String()

		case value == "$hash":
			value = formats.Gibberish(uuid.New())

		case strings.HasPrefix(value, "$file "):
			includePath := strings.TrimSpace(strings.TrimPrefix(value, "$file "))

			dirPath := filepath.Dir(".")
			includePath = filepath.Join(dirPath, includePath)

			data, err := os.ReadFile(includePath)
			if err != nil {
				fmt.Printf("failed to read file: %v\n", err)
			}

			value = Apply(string(data))

		case value == "$seq":
			value = strconv.Itoa(int(sequence.Add(1)))

		case strings.HasPrefix(value, "$env "):
			envVar := os.Getenv(value[len("$env "):])
			if envVar != "" {
				value = envVar
			}
		}

		subs[key] = value
	}

	return HandleSubstitutionMap(text, subs)
}
