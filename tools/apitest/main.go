package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tucats/validator"
	"github.com/tucats/apitest/defs"
	"github.com/tucats/apitest/dictionary"
	"github.com/tucats/apitest/formats"
	"github.com/tucats/apitest/logging"
)

var BuildVersion = "developer build"
var filter string
var testsExecuted = 0
var validate *validator.Item

func main() {
	var (
		err            error
		rootPath       string
		pathList       []string
		dictionaryList []string
	)

	now := time.Now()

	// Load data structures into the dictionary.
	validate, err = validator.New(&defs.Test{})
	if err != nil {
		exit(fmt.Sprintf("Error defining JSON validation, %v", err))
	}

	// Figure out what host we are running on to use as a default
	// in constructing endpoints.
	hostname, _ := os.Hostname()
	if !strings.Contains(hostname, ".") {
		hostname += ".local"
	}

	// Set up some default values for the dictionary. These can be overridden with the --define
	// command line flag or placed in the dictionary.json file in the test directory.
	dictionary.Dictionary["SCHEME"] = "https"
	dictionary.Dictionary["HOST"] = hostname
	dictionary.Dictionary["PASSWORD"] = "password" // Default testing password
	dictionary.Dictionary["VERSION"] = BuildVersion

	// Scan over the command line arguments to set up the test environment.
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-h", "--help":
			help()
			os.Exit(0)

		case "-f", "--filter":
			if i+1 >= len(os.Args) {
				exit("missing argument for --filter")
			}

			filter = os.Args[i+1]
			i++

		case "-p", "--path":
			if i+1 >= len(os.Args) {
				exit("missing argument --path")
			}

			pathList = append(pathList, os.Args[i+1])
			i++

		case "-r", "--rest":
			logging.Rest = true

		case "-d", "--dictionary", "--dict":
			if i+1 >= len(os.Args) {
				exit("missing argument for --dictionary")
			}

			dictionaryList = append(dictionaryList, os.Args[i+1])
			i++

		case "-x", "--set", "--define":
			if i+1 >= len(os.Args) {
				exit("missing argument for --define")
			}

			parts := strings.SplitN(os.Args[i+1], "=", 2)
			if len(parts) != 2 {
				exit("invalid key=value format for --define: " + os.Args[i+1])
			}

			dictionary.Dictionary[parts[0]] = parts[1]

			i++

		case "-v", "--verbose":
			logging.Verbose = true

		default:
			if !strings.HasPrefix(arg, "-") {
				pathList = append(pathList, arg)
			} else {
				exit("unknown option: " + arg)
			}
		}
	}

	if len(pathList) == 0 {
		exit("no path specified")
	}

	// Load all dictionaries referenced.
	for _, path := range dictionaryList {
		err := dictionary.Load(path)
		if err != nil {
			exit("bad dictionary path: " + err.Error())
		}
	}

	// For all paths provided, run the tests.
	for _, path := range pathList {
		rootPath, err = filepath.Abs(filepath.Clean(path))
		if err != nil {
			exit("bad test suite path: " + err.Error())
		}

		dictionary.Dictionary["ROOT"] = rootPath

		// if the path isn't a dictionary, just run the single test named.
		info, err := os.Stat(rootPath)
		if err != nil {
			exit("bad test suite path: " + err.Error())
		}

		if !info.IsDir() {
			err = runSingleTest(rootPath)
		} else {
			// Run all the tests in the path
			err = runTests(path)
		}

		if err != nil {
			if strings.Contains(err.Error(), defs.AbortError) {
				fmt.Printf("Server testing unavailable, %v\n", err)

				err = nil
			}

			break
		}
	}

	if err != nil {
		fmt.Printf("Error running tests: %v\n", err)
		os.Exit(1)
	}

	duration := time.Since(now)
	fmt.Printf("\nExecuted %d tests in %v\n", testsExecuted, strings.TrimSpace(formats.Duration(duration, true)))
}

func exit(msg string) {
	fmt.Println("Error: " + msg)
	os.Exit(1)
}

func runSingleTest(file string) error {
	duration, err := TestFile(file)

	pad := ""

	if logging.Verbose {
		pad = "  "
	}

	if err != nil {
		fmt.Printf("%sFAIL       %-40s: %v\n", pad, file, err)
	} else {
		fmt.Printf("%sPASS       %-40s %v\n", pad, file, formats.Duration(duration, true))
	}

	testsExecuted++

	return err
}

func runTests(path string) error {
	var (
		duration time.Duration
		lastErr  error
	)

	if logging.Verbose {
		fmt.Printf("Testing suite %s...\n", path)
	}

	// First, try to load any dictionary in the path location. If not found, we don't care.
	err := dictionary.Load(filepath.Join(path, "dictionary.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// If the dictionary included in a different abort error string, update the one we
	// test against now.
	if text, ok := dictionary.Dictionary["CONNECTION_REFUSED"]; ok {
		defs.AbortError = text
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

		// If it's the reserved name "dictionary.json", skip it.
		name := file.Name()
		if name == "dictionary.json" {
			continue
		}

		// If it's not a JSON file, skip it.
		if filepath.Ext(name) != ".json" {
			continue
		}

		if filter != "" {
			if !strings.Contains(name, filter) {
				continue
			}
		}

		fileNames = append(fileNames, name)
	}

	// Sort all the names in alphabetical order. This ensures that tests
	// are run in a consistent order.
	sort.Strings(fileNames)

	// For each test file, run the tests.
	for _, file := range fileNames {
		name := filepath.Join(path, file)

		duration, err = TestFile(name)
		if err != nil && strings.Contains(err.Error(), defs.AbortError) {
			break
		}

		if err != nil {
			lastErr = err
		}

		pad := ""

		if logging.Verbose {
			pad = "  "
		}

		if err != nil {
			fmt.Printf("%sFAIL       %-40s: %v\n", pad, file, err)
		} else {
			fmt.Printf("%sPASS       %-40s %v\n", pad, file, formats.Duration(duration, true))
		}

		testsExecuted++
	}

	return lastErr
}
