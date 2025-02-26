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
var rest = false

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
				exit("missing argument --path")
			}

			path = os.Args[i+1]
			i++

		case "-r", "--rest":
			rest = true

		case "-d", "--define":
			if i+1 >= len(os.Args) {
				exit("missing argument for --define")
			}

			parts := strings.SplitN(os.Args[i+1], "=", 2)
			if len(parts) != 2 {
				exit("invalid key=value format for --define: " + os.Args[i+1])
			}

			Dictionary[parts[0]] = parts[1]

			i++

		case "-v", "--verbose":
			verbose = true

		default:
			exit("unknown option: " + arg)
		}
	}

	if path == "" {
		exit("no path specified")
	}

	rootPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		exit("bad path: " + err.Error())
	}

	Dictionary["ROOT"] = rootPath

	// Run all the tests in the path
	err = runTests(path)

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

func exit(msg string) {
	fmt.Println("Error: " + msg)
	os.Exit(1)
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
			subdir := filepath.Join(path, file.Name())

			// Recursively run the tests in the subdirectory.
			err = runTests(subdir)
			if err != nil {
				return err
			}

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
