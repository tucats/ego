// Command errlint checks Ego's global error definitions for two kinds of
// drift: an error symbol that is defined but never referenced anywhere in
// the source tree, and an error symbol whose localization key has no entry
// in the message catalog. Run with -h for usage.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// options holds the parsed command-line arguments. Each of the three
// options may be repeated, so every field is a slice.
type options struct {
	errorFiles  []string
	stringFiles []string
	sourcePaths []string
}

func main() {
	opts, err := parseArguments(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	rep, err := run(opts)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	rep.print(os.Stdout)

	if rep.hasIssues() {
		os.Exit(1)
	}
}

// parseArguments walks the command line, collecting the required
// --errors/--strings/--source options (each of which may appear more than
// once) plus the -v/--verbose and -h/--help flags.
func parseArguments(args []string) (options, error) {
	var opts options

	for index := 0; index < len(args); index++ {
		arg := args[index]

		next := func() (string, error) {
			index++
			if index >= len(args) {
				return "", fmt.Errorf("missing value after %s", arg)
			}

			return args[index], nil
		}

		switch arg {
		case "-e", "--errors":
			value, err := next()
			if err != nil {
				return opts, err
			}

			opts.errorFiles = append(opts.errorFiles, value)

		case "-s", "--strings":
			value, err := next()
			if err != nil {
				return opts, err
			}

			opts.stringFiles = append(opts.stringFiles, value)

		case "-p", "--source":
			value, err := next()
			if err != nil {
				return opts, err
			}

			opts.sourcePaths = append(opts.sourcePaths, value)

		case "-v", "--verbose":
			verbose = true

		case "-h", "--help":
			usage()
			os.Exit(0)

		default:
			return opts, fmt.Errorf("unknown option: %s", arg)
		}
	}

	if len(opts.errorFiles) == 0 {
		return opts, fmt.Errorf("at least one --errors <file> is required")
	}

	if len(opts.stringFiles) == 0 {
		return opts, fmt.Errorf("at least one --strings <file> is required")
	}

	if len(opts.sourcePaths) == 0 {
		return opts, fmt.Errorf("at least one --source <path> is required")
	}

	return opts, nil
}

var verbose = false

// run performs the three-phase check described in the package comment and
// returns the accumulated report.
func run(opts options) (*report, error) {
	var defs []errorDef

	skip := map[string]bool{}

	for _, path := range opts.errorFiles {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolving %s: %w", path, err)
		}

		skip[abs] = true

		found, err := extractErrorDefs(path)
		if err != nil {
			return nil, err
		}

		defs = append(defs, found...)
	}

	catalog := map[string]bool{}

	for _, path := range opts.stringFiles {
		keys, err := loadCatalogKeys(path)
		if err != nil {
			return nil, err
		}

		for key := range keys {
			catalog[key] = true
		}
	}

	used := map[string]bool{}

	for _, path := range opts.sourcePaths {
		found, err := findUsedSymbols(path, skip)
		if err != nil {
			return nil, err
		}

		for symbol := range found {
			used[symbol] = true
		}
	}

	return buildReport(defs, catalog, used), nil
}

// report is the accumulated result of a run: every error symbol found to be
// unused, every error symbol found to be missing its localization entry,
// and every symbol defined more than once.
type report struct {
	unused     []errorDef
	missing    []errorDef
	duplicates []string
	total      int
}

func (r *report) hasIssues() bool {
	return len(r.unused) > 0 || len(r.missing) > 0 || len(r.duplicates) > 0
}

// buildReport compares the defined error symbols against the catalog keys
// and the set of symbols actually referenced in the source tree, in that
// order, so a duplicate definition is reported once rather than being
// double-counted as both unused and missing.
func buildReport(defs []errorDef, catalog, used map[string]bool) *report {
	rep := &report{total: len(defs)}
	seen := map[string]errorDef{}

	for _, d := range defs {
		if prev, ok := seen[d.Symbol]; ok {
			rep.duplicates = append(rep.duplicates, fmt.Sprintf(
				"%s:%d: %s redefines error symbol already defined at %s:%d",
				d.File, d.Line, d.Symbol, prev.File, prev.Line))

			continue
		}

		seen[d.Symbol] = d

		if !used[d.Symbol] {
			rep.unused = append(rep.unused, d)
		}

		// Keys with a leading underscore are the small set of internal
		// flow-control sentinels (ErrContinue, ErrStop, and similar) that
		// are documented in messages.go as deliberately not localized, so
		// they are exempt from the missing-localization check.
		if !strings.HasPrefix(d.Key, "_") {
			if !catalog[catalogKey(d.Key)] {
				rep.missing = append(rep.missing, d)
			}
		}
	}

	sort.Slice(rep.unused, func(i, j int) bool { return rep.unused[i].Symbol < rep.unused[j].Symbol })
	sort.Slice(rep.missing, func(i, j int) bool { return rep.missing[i].Symbol < rep.missing[j].Symbol })
	sort.Strings(rep.duplicates)

	return rep
}

// catalogKey converts an error's Message() key into the fully-qualified
// catalog key that i18n.E looks it up by, e.g. "child.run.timeout" becomes
// "error.child.run.timeout" (see internal/i18n's E/ELang functions).
func catalogKey(key string) string {
	return "error." + key
}

func (r *report) print(w *os.File) {
	for _, d := range r.duplicates {
		fmt.Fprintf(w, "%s\n", d)
	}

	for _, d := range r.unused {
		fmt.Fprintf(w, "%s:%d: unused error: %s is never referenced outside its definition\n", d.File, d.Line, d.Symbol)
	}

	for _, d := range r.missing {
		fmt.Fprintf(w, "%s:%d: missing localization: %s has no catalog entry for key %q\n", d.File, d.Line, d.Symbol, catalogKey(d.Key))
	}

	if !r.hasIssues() {
		if verbose {
			fmt.Fprintf(w, "errlint: checked %d error definitions, no issues found\n", r.total)
		}

		return
	}

	fmt.Fprintf(w, "errlint: %d error definitions checked, %d unused, %d missing localization",
		r.total, len(r.unused), len(r.missing))

	if len(r.duplicates) > 0 {
		fmt.Fprintf(w, ", %d duplicate", len(r.duplicates))
	}

	fmt.Fprintln(w)
}

func usage() {
	fmt.Println(`errlint - verify Ego error definitions are used and localized

Usage:
  errlint --errors <file> [--errors <file> ...] \
          --strings <file> [--strings <file> ...] \
          --source <path> [--source <path> ...] [options]

Options:
  -e, --errors <file>   file defining error symbols as var X = Message("key");
                         may be repeated
  -s, --strings <file>  localization catalog file (messages_*.txt) to check
                         error keys against; may be repeated
  -p, --source <path>   root of a source tree to scan for references to the
                         error symbols; may be repeated
  -v, --verbose         report a summary line even when no issues are found
  -h, --help            show this help message

errlint reports two kinds of drift, and exits 1 if either is found:
  - error symbols defined but never referenced in the given source tree
    (excluding the --errors files themselves)
  - error symbols whose Message() key has no matching entry in the given
    localization catalog(s)`)
}
