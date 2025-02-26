package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/tucats/ego/i18n"
)

var Dictionary = make(map[string]string)

var sequence = atomic.Int32{}

func LoadDictionary(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

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
			value = Gibberish(uuid.New())

		case strings.HasPrefix(value, "$file "):
			includePath := strings.TrimSpace(strings.TrimPrefix(value, "$file "))

			dirPath := filepath.Dir(filePath)
			includePath = filepath.Join(dirPath, includePath)

			data, err := os.ReadFile(includePath)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			value = ApplyDictionary(string(data))

		case strings.HasPrefix(value, "$json "):
		case strings.HasPrefix(value, "$"):
			envVar := os.Getenv(value[1:])
			if envVar != "" {
				value = envVar
			}
		}

		Dictionary[key] = value
	}

	return nil
}

func ApplyDictionary(text string) string {
	subs := make(map[string]interface{})

	for key, value := range Dictionary {
		// Handle special cases for values.
		switch {
		case value == "$uuid":
			value = uuid.New().String()

		case value == "$hash":
			value = Gibberish(uuid.New())

		case strings.HasPrefix(value, "$file "):
			includePath := strings.TrimSpace(strings.TrimPrefix(value, "$file "))

			dirPath := filepath.Dir(".")
			includePath = filepath.Join(dirPath, includePath)

			data, err := os.ReadFile(includePath)
			if err != nil {
				fmt.Printf("failed to read file: %v\n", err)
			}

			value = ApplyDictionary(string(data))

		case value == "$seq":
			value = strconv.Itoa(int(sequence.Add(1)))

		case strings.HasPrefix(value, "$"):
			envVar := os.Getenv(value[1:])
			if envVar != "" {
				value = envVar
			}
		}

		subs[key] = value
	}

	return i18n.HandleSubstitutionMap(text, subs)
}

func UpdateDictionary(text string, items map[string]string) error {
	for key, value := range items {
		item, err := GetOneItem(text, value)
		if err != nil {
			return err
		}

		Dictionary[key] = item

		if verbose {
			if key == "API_TOKEN" {
				item = "***REDACTED***"
			}

			fmt.Printf("  Updating   {{%s}} = %s\n", key, item)
		}
	}

	return nil
}
