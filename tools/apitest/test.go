package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func TestFile(filename string) error {
	var (
		err  error
		test Test
	)

	// Load the test definition form the file into a Test object.
	b, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, &test)
	if err != nil {
		return err
	}

	if verbose {
		base := filepath.Base(filename)
		desc := ""

		if test.Description != "" {
			desc = fmt.Sprintf(", %s", test.Description)
		}

		fmt.Printf("Running %s%s\n", base, desc)
	}

	return run(&test)
}

func run(test *Test) error {
	var err error

	err = executeTest(test)
	if err != nil {
		return err
	}

	// Save any results from the test back in the dictionary.
	err = UpdateDictionary(test.Response.Body, test.Response.Save)

	return err
}
