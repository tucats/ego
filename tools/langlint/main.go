// Command langlint cleans up and reorganizes Ego localization message files.
//
// It normalizes blank-line spacing, sorts the keys within each section
// alphabetically, and reports issues such as malformed lines, duplicate
// keys, and unbalanced substitution braces. Run with -h for usage.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	verbose = false
	check   = false
)

func main() {
	files, err := parseArguments(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(files) == 0 {
		usage()
		os.Exit(1)
	}

	exitCode := 0

	for _, file := range files {
		if lintOneFile(file) {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// parseArguments walks the command line, handling options and collecting
// the list of files to process (either named directly or discovered via
// -p/--path).
func parseArguments(args []string) ([]string, error) {
	var (
		path  string
		files []string
	)

	for index := 0; index < len(args); index++ {
		arg := args[index]

		switch arg {
		case "-p", "--path":
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("missing directory name after %s", arg)
			}

			path = args[index]

		case "-c", "--check":
			check = true

		case "-v", "--verbose":
			verbose = true

		case "-h", "--help":
			usage()
			os.Exit(0)

		default:
			if strings.HasPrefix(arg, "-") {
				return nil, fmt.Errorf("unknown option: %s", arg)
			}

			files = append(files, arg)
		}
	}

	if path != "" {
		found, err := messageFiles(path)
		if err != nil {
			return nil, err
		}

		files = append(files, found...)
	}

	return files, nil
}

// messageFiles returns the messages_*.txt files found directly in dir.
func messageFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() || !entry.Type().IsRegular() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "messages_") || !strings.HasSuffix(name, ".txt") {
			continue
		}

		files = append(files, filepath.Join(dir, name))
	}

	return files, nil
}

// lintOneFile processes a single file and reports its outcome, returning
// true if the run should be considered to have failed (a parse error, a
// file that needs reformatting in check mode, or any warnings).
func lintOneFile(path string) bool {
	result, err := lintFile(path, check)
	if err != nil {
		fmt.Printf("%s: error: %v\n", path, err)

		return true
	}

	failed := false

	for _, w := range result.warnings {
		fmt.Printf("%s: warning: %s\n", path, w)

		failed = true
	}

	switch {
	case result.changed && check:
		fmt.Printf("%s: would reformat\n", path)

		failed = true

	case result.changed:
		fmt.Printf("%s: reformatted\n", path)

	case verbose:
		fmt.Printf("%s: unchanged\n", path)
	}

	return failed
}

func usage() {
	fmt.Println(`langlint - lint and reorganize Ego localization message files

Usage:
  langlint [options] [file ...]

Options:
  -p, --path <dir>   process all messages_*.txt files found in <dir>
  -c, --check        report issues without rewriting any file
  -v, --verbose      report unchanged files as well
  -h, --help         show this help message`)
}
