package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tucats/apitest/defs"
	"github.com/tucats/apitest/dictionary"
	"github.com/tucats/apitest/logging"
	"github.com/tucats/apitest/parser"
	"github.com/tucats/apitest/tester"
)

func TestFile(filename string) (time.Duration, error) {
	var (
		err  error
		test defs.Test
	)

	// Load the test definition form the file into a Test object.
	b, err := os.ReadFile(filename)
	if err != nil {
		return 0, err
	}

	b = []byte(dictionary.Apply(string(parser.RemoveComments(b))))

	// Validate the test definition JSON
	err = validate.Validate(string(b))
	if err != nil {
		return 0, fmt.Errorf("test definition validation error: %v", err)
	}

	// Now unmarshal the JSON into the test structure
	err = json.Unmarshal(b, &test)
	if err != nil {
		return 0, err
	}

	if logging.Verbose {
		base := filepath.Base(filename)
		desc := ""

		if test.Description != "" {
			desc = fmt.Sprintf(", %s", test.Description)
		}

		fmt.Printf("Running %s%s\n", base, desc)
	}

	return run(&test)
}

func run(test *defs.Test) (time.Duration, error) {
	var err error

	err = tester.ExecuteTest(test)
	if err != nil {
		return 0, err
	}

	// Save any results from the test back in the dictionary.
	err = dictionary.Update(test.Response.Body, test.Response.Save)

	return test.Duration, err
}
