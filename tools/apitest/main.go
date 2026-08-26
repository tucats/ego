package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/apitest/defs"
	"github.com/tucats/apitest/dictionary"
	"github.com/tucats/apitest/formats"
	"github.com/tucats/apitest/logging"
	"github.com/tucats/apitest/stats"
	"github.com/tucats/apitest/tester"
	"github.com/tucats/validator"
)

var BuildVersion = "developer build"
var filter string
var testsExecuted = 0
var validate *validator.Item
var quiet = false

// parallelStreams, loopDuration and loopIterations are set from the
// --parallel/--duration/--iterations flags to drive load-exerciser mode.
// collector is non-nil only when running in load mode (loopDuration > 0 or
// loopIterations > 0); every method on it tolerates a nil receiver, so code
// paths that aren't load-aware can call it unconditionally.
var (
	parallelStreams = 1
	loopDuration    time.Duration
	loopIterations  int
	statsOutPath    string
	collector       *stats.Collector
)

func main() {
	var (
		err            error
		pathList       []string
		dictionaryList []string
	)

	now := time.Now()

	if BuildVersion == "developer build" {
		tester.VersionString = "internal testing tool; resty.v1"
	} else {
		tester.VersionString = BuildVersion
	}

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
		case "-q", "--quiet":
			quiet = true

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

		case "--parallel":
			if i+1 >= len(os.Args) {
				exit("missing argument for --parallel")
			}

			parallelStreams, err = strconv.Atoi(os.Args[i+1])
			if err != nil || parallelStreams < 1 {
				exit("invalid stream count for --parallel: " + os.Args[i+1])
			}

			i++

		case "--duration":
			if i+1 >= len(os.Args) {
				exit("missing argument for --duration")
			}

			loopDuration, err = time.ParseDuration(os.Args[i+1])
			if err != nil {
				exit("invalid duration for --duration: " + os.Args[i+1])
			}

			i++

		case "--iterations":
			if i+1 >= len(os.Args) {
				exit("missing argument for --iterations")
			}

			loopIterations, err = strconv.Atoi(os.Args[i+1])
			if err != nil || loopIterations < 1 {
				exit("invalid count for --iterations: " + os.Args[i+1])
			}

			i++

		case "--stats-out":
			if i+1 >= len(os.Args) {
				exit("missing argument for --stats-out")
			}

			statsOutPath = os.Args[i+1]
			i++

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

	// --parallel hands off entirely to the orchestrator, which re-execs this
	// same binary N times as independent child processes and merges their
	// results; it never returns.
	if parallelStreams > 1 {
		os.Exit(runParallel(parallelStreams))
	}

	// --duration/--iterations puts this stream into load-exerciser mode:
	// repeat the whole suite pass until the budget is exhausted, recording
	// every test's outcome into a stats.Collector instead of printing a
	// PASS/FAIL line per test.
	if loopDuration > 0 || loopIterations > 0 {
		collector = stats.New(streamIndex())

		runLoop(pathList)

		if statsOutPath != "" {
			if werr := collector.WriteFile(statsOutPath); werr != nil {
				exit("writing stats file: " + werr.Error())
			}
		} else {
			fmt.Print(collector.Finish().Report())
		}

		return
	}

	// For all paths provided, run the tests.
	err = runAllPaths(pathList)
	if err != nil {
		if isAbortErr(err) {
			fmt.Printf("Server testing unavailable, %v\n", err)
		} else {
			fmt.Printf("Error running tests: %v\n", err)
			os.Exit(1)
		}
	}

	duration := time.Since(now)
	fmt.Printf("TEST: Completed %d tests in %v\n", testsExecuted, strings.TrimSpace(formats.Duration(duration, true)))
}

// runAllPaths runs every path in pathList once, in order, stopping at the
// first error -- this is the tool's original single-pass behavior, factored
// out so both the default (single-pass) mode and runLoop's repeated passes
// share it.
func runAllPaths(pathList []string) error {
	var err error

	for _, path := range pathList {
		rootPath, absErr := filepath.Abs(filepath.Clean(path))
		if absErr != nil {
			exit("bad test suite path: " + absErr.Error())
		}

		dictionary.Dictionary["ROOT"] = rootPath

		// if the path isn't a directory, just run the single test named.
		info, statErr := os.Stat(rootPath)
		if statErr != nil {
			exit("bad test suite path: " + statErr.Error())
		}

		if !info.IsDir() {
			err = runSingleTest(rootPath)
		} else {
			// Run all the tests in the path
			err = runTests(path)
		}

		if err != nil {
			break
		}
	}

	return err
}

// runLoop repeats runAllPaths until the --duration or --iterations budget
// is exhausted. A genuine abort condition (connection refused / deadline
// exceeded) stops the loop early -- there's no point hammering a target
// that's unreachable for the rest of the budget -- but an ordinary
// assertion failure inside one iteration does not; it's recorded in the
// collector and the loop moves on to the next iteration.
func runLoop(pathList []string) {
	var deadline time.Time

	if loopDuration > 0 {
		deadline = time.Now().Add(loopDuration)
	}

	count := 0

	for {
		if loopIterations > 0 && count >= loopIterations {
			return
		}

		if !deadline.IsZero() && time.Now().After(deadline) {
			return
		}

		err := runAllPaths(pathList)

		collector.IterationDone()
		count++

		if isAbortErr(err) {
			fmt.Printf("Server testing unavailable, %v\n", err)

			return
		}
	}
}

// streamIndex reports this process's STREAM dictionary value (set by the
// --parallel orchestrator via "-x STREAM=<i>"), or 0 when running standalone.
func streamIndex() int {
	if v, ok := dictionary.Dictionary["STREAM"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}

	return 0
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

	if collector == nil || logging.Verbose {
		if err != nil {
			fmt.Printf("%sFAIL       %-40s: %v\n", pad, file, err)
		} else if !quiet {
			fmt.Printf("%sPASS       %-40s %v\n", pad, file, formats.Duration(duration, true))
		}
	}

	testsExecuted++

	return err
}

// isAbortErr reports whether err represents a condition that should stop the
// whole run immediately -- the server refused the connection, or a request
// timed out waiting for one that was never coming (deadline exceeded) -- as
// opposed to an ordinary test assertion failure, which must be recorded and
// reported but must not prevent any other test, file, or directory from
// still being attempted. runTests' directory-recursion loop and its own file
// loop both need this same distinction.
func isAbortErr(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, defs.ErrAbort) {
		return true
	}

	return strings.Contains(err.Error(), defs.AbortError) || strings.Contains(err.Error(), "deadline exceeded")
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

			// Recursively run the tests in the subdirectory. A genuine
			// abort (connection refused / deadline exceeded) propagates
			// immediately, stopping the whole run early, same as it
			// always has. An ordinary test failure inside subdir must
			// NOT do the same -- before this fix, any single failing
			// test anywhere silently skipped every sibling directory (and
			// file) that would have sorted after it, with no indication
			// anything was skipped at all. It's recorded in lastErr
			// instead, so the run's final exit status still reflects it,
			// while every other directory still gets a chance to run and
			// report its own results.
			subErr := runTests(subdir)
			if isAbortErr(subErr) {
				return subErr
			}

			if subErr != nil && lastErr == nil {
				lastErr = subErr
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
		if isAbortErr(err) {
			if lastErr == nil {
				lastErr = defs.ErrAbort
			}

			break
		}

		if err != nil {
			lastErr = err
		}

		pad := ""

		if logging.Verbose {
			pad = "  "
		}

		if collector == nil || logging.Verbose {
			if err != nil {
				fmt.Printf("%sFAIL       %-60s: %v\n", pad, file, err)
			} else if !quiet {
				fmt.Printf("%sPASS       %-60s %v\n", pad, file, formats.Duration(duration, true))
			}
		}

		testsExecuted++
	}

	return lastErr
}
