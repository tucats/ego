package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var verbose = false

func main() {
	var (
		err  error
		path string
	)

	now := time.Now()

	hostname, _ := os.Hostname()
	if !strings.Contains(hostname, ".") {
		hostname += ".local"
	}

	// Set up some default values for the dictionary. These can be overridden with the --define
	// command line flag or placed in the dictionary.json file in the test directory.
	Dictionary["SCHEME"] = "https"
	Dictionary["HOST"] = hostname
	Dictionary["PASSWORD"] = "password" // Default testing password

	// Scan over the commadn line arguments to set up the test environment.
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-p", "--path":
			if i+1 >= len(os.Args) {
				err = errors.New("missing argument --path")

				break
			}

			path = os.Args[i+1]
			i++

		case "-d", "--define":
			if i+1 >= len(os.Args) {
				err = errors.New("missing argument for --define")

				break
			}

			parts := strings.SplitN(os.Args[i+1], "=", 2)
			Dictionary[parts[0]] = parts[1]

			i++

		case "-v", "--verbose":
			verbose = true

		default:
			err = errors.New("unknown option: " + arg)

			break
		}
	}

	if err == nil && path == "" {
		err = errors.New("no path specified")
	}

	if err == nil {
		err = runTests(path)
	}

	if err != nil && strings.Contains(err.Error(), abortError) {
		fmt.Printf("Server testing unavailable, %v\n", err)

		err = nil
	}

	if err != nil {
		fmt.Printf("Error running tests: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		duration := time.Since(now)
		fmt.Printf("\nTotal test duration: %v\n", FormatDuration(duration, true))
	}
}

func runTests(path string) error {
	var duration time.Duration

	if verbose {
		fmt.Printf("Testing suite %s...\n", path)
	}

	// First, try to load any dictionary in the path location. If not found, we don't care.
	err := LoadDictionary(filepath.Join(path, "dictionary.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// If the dictionary resulted in a different abort error string, update the one we
	// test against now.
	if text, ok := Dictionary["CONNECTION_REFUSED"]; ok {
		abortError = text
	}

	// Read the contents of the tests directory.
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	fileNames := make([]string, 0)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if name == "dictionary.json" {
			continue
		}

		if filepath.Ext(name) != ".json" {
			continue
		}

		fileNames = append(fileNames, name)
	}

	sort.Strings(fileNames)

	// For each test file that ends in ".json", run the tests.
	for _, file := range fileNames {
		if file == "dictionary.json" {
			continue
		}

		if filepath.Ext(file) != ".json" {
			continue
		}

		name := filepath.Join(path, file)

		duration, err = TestFile(name)
		if err != nil && strings.Contains(err.Error(), abortError) {
			break
		}

		pad := ""

		if verbose {
			pad = "  "
		}

		if err != nil {
			fmt.Printf("%sFAIL       %-30s: %v\n", pad, file, err)
		} else {
			fmt.Printf("%sPASS       %-30s %v\n", pad, file, FormatDuration(duration, true))
		}
	}

	return err
}
