package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// loadCatalogKeys reads a localization message file in the "messages_*.txt"
// format used under internal/i18n/languages (as produced by tools/langlint)
// and returns the set of fully-qualified keys it defines: "section.key" for
// an entry found under a "[section]" header, or bare "key" for an entry
// that precedes any header. This mirrors how internal/i18n looks strings up
// (see ofTypeLang in internal/i18n/strings.go), so an error's Message() key
// "child.run.timeout" is expected to appear here as "error.child.run.timeout".
func loadCatalogKeys(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	keys := map[string]bool{}
	section := ""

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)

		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
			continue

		case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
			section = trimmed[1 : len(trimmed)-1]

		default:
			index := strings.Index(line, "=")
			if index < 0 {
				continue
			}

			key := line[:index]
			if section != "" {
				key = section + "." + key
			}

			keys[key] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return keys, nil
}
