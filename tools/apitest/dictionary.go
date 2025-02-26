package main

import (
	"encoding/json"
	"fmt"
	"os"
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
		if value == "$uuid" {
			value = uuid.New().String()
		} else if strings.HasPrefix(value, "$") {
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
		// Handle special cases for values. "$seq" is a reserved item for a
		// increasing sequence number. Any other string starting with "$" is
		// looked up as an environment variable, and if non-empty is used as
		// the value for the item.
		if value == "$seq" {
			value = strconv.Itoa(int(sequence.Add(1)))
		} else if strings.HasPrefix(value, "$") {
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
		item, err := GetItem(text, value)
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
