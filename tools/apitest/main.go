package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var verbose = false

func main() {
	var (
		err  error
		path string
	)

	hostname, _ := os.Hostname()

	// Set up some default values for the dictionary. These can be overridden with the --define
	// command line flag or placed in the dictionary.json file in the test directory.
	Dictionary["SCHEME"] = "https"
	Dictionary["HOST"] = hostname

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

	fmt.Println("DEBUG: path ", path)
	fmt.Println("DEBUG: dictionary ", Dictionary)

	if err == nil {
		err = runTests(path)
	}

	if err != nil {
		fmt.Printf("Error running tests: %v\n", err)
		os.Exit(1)
	}
}

func runTests(path string) error {
	fmt.Printf("Testing %s...\n", path)

	// First, try to load any dictionary in the path location. If not found, we don't care.
	err := LoadDictionary(filepath.Join(path, "dictionary.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
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

		err = TestFile(name)
		pad := ""

		if verbose {
			pad = "  "
		}

		if err != nil {
			fmt.Printf("%sFAIL       %s: %v\n", pad, file, err)
		} else {
			fmt.Printf("%sPASS       %s\n", pad, file)
		}
	}

	return err
}
