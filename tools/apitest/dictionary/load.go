package dictionary

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/apitest/formats"
	"github.com/tucats/apitest/logging"
	"github.com/tucats/apitest/parser"
)

// Attempt to load a dictionary definition from an external JSON file.
func Load(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	data = parser.RemoveComments(data)

	var dictionary map[string]string

	if err := json.Unmarshal(data, &dictionary); err != nil {
		return err
	}

	for key, value := range dictionary {
		// Handle some special cases for values. "$uuid" is a reserved item for a
		// generated UUID that is created during initialization. Also, any substitution
		// string starting with "$" is looked up as an environment variable, and if non-empty
		// is used as the value for the item.
		switch {
		case value == "$uuid":
			value = uuid.New().String()

		case value == "$hash":
			value = formats.Gibberish(uuid.New())

		case strings.HasPrefix(value, "$file "):
			includePath := strings.TrimSpace(strings.TrimPrefix(value, "$file "))

			dirPath := filepath.Dir(filePath)
			includePath = filepath.Join(dirPath, includePath)

			data, err := os.ReadFile(includePath)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			value = Apply(string(data))

		case strings.HasPrefix(value, "$json "):
		case strings.HasPrefix(value, "$"):
			envVar := os.Getenv(value[1:])
			if envVar != "" {
				value = envVar
			}
		}

		Dictionary[key] = value
	}

	if logging.Verbose {
		fmt.Printf("Loaded %d definitions from %s\n", len(dictionary), filePath)
	}

	return nil
}
