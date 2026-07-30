package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// blockKind identifies the two kinds of top-level elements a message file
// is made of once blank lines are discarded: a run of comment lines, or a
// section (an optional "[prefix]" header followed by its key/value lines).
type blockKind int

const (
	commentBlockKind blockKind = iota
	sectionBlockKind
)

// kvEntry is a single "key=value" line found under some prefix.
type kvEntry struct {
	key   string
	value string
	line  int
}

// block is one contiguous unit of output: either a comment block (preserved
// verbatim) or a section block (an optional header line plus its entries,
// which are sorted alphabetically by key when rendered).
type block struct {
	kind      blockKind
	comments  []string
	header    string
	hasHeader bool
	entries   []kvEntry
}

// formatError reports a malformed line that prevents the file from being
// safely reorganized.
type formatError struct {
	line int
	text string
}

func (e *formatError) Error() string {
	return fmt.Sprintf("line %d: %s", e.line, e.text)
}

// parseResult is the block structure recovered from a message file, plus any
// non-fatal issues noticed along the way.
type parseResult struct {
	blocks   []*block
	warnings []string
}

// parse reads the raw contents of a message file and recovers its block
// structure. Blank lines carry no information beyond visual separation, so
// they are discarded here; render derives correct spacing from the rules
// instead of preserving whatever was in the source.
func parse(data []byte) (*parseResult, error) {
	var (
		blocks        []*block
		cur           *block
		currentPrefix string
	)

	warnings := []string{}
	seenAt := map[string][]int{}

	for index, raw := range strings.Split(string(data), "\n") {
		lineNumber := index + 1
		line := strings.TrimRight(raw, "\r")

		if strings.TrimSpace(line) == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "#"):
			if cur == nil || cur.kind != commentBlockKind {
				cur = &block{kind: commentBlockKind}
				blocks = append(blocks, cur)
			}

			cur.comments = append(cur.comments, line)

		case strings.HasPrefix(line, "["):
			if !strings.HasSuffix(line, "]") {
				return nil, &formatError{line: lineNumber, text: "malformed section header: " + line}
			}

			currentPrefix = line[1 : len(line)-1]
			cur = &block{kind: sectionBlockKind, header: currentPrefix, hasHeader: true}
			blocks = append(blocks, cur)

		default:
			key, value, err := splitEntry(line, lineNumber)
			if err != nil {
				return nil, err
			}

			if cur == nil || cur.kind != sectionBlockKind {
				cur = &block{kind: sectionBlockKind, header: currentPrefix}
				blocks = append(blocks, cur)
			}

			cur.entries = append(cur.entries, kvEntry{key: key, value: value, line: lineNumber})

			fullKey := key
			if cur.header != "" {
				fullKey = cur.header + "." + key
			}

			seenAt[fullKey] = append(seenAt[fullKey], lineNumber)

			if w := unmatchedBraceWarning(fullKey, value, lineNumber); w != "" {
				warnings = append(warnings, w)
			}
		}
	}

	warnings = append(warnings, duplicateKeyWarnings(seenAt)...)

	return &parseResult{blocks: blocks, warnings: warnings}, nil
}

// splitEntry breaks a "key=value" line into its parts, reporting a
// formatError if the line has no "=" separator or an empty key.
func splitEntry(line string, lineNumber int) (key, value string, err error) {
	i := strings.Index(line, "=")
	if i < 0 {
		return "", "", &formatError{line: lineNumber, text: "malformed line, missing '=': " + line}
	}

	key = line[:i]
	value = line[i+1:]

	if key == "" {
		return "", "", &formatError{line: lineNumber, text: "malformed line, empty key: " + line}
	}

	return key, value, nil
}

// unmatchedBraceWarning returns a warning string if the substitution braces
// in a message value are not balanced, ignoring escaped braces written as
// '{' or '}'.
func unmatchedBraceWarning(fullKey, value string, lineNumber int) string {
	stripped := strings.ReplaceAll(value, "'{'", "")
	stripped = strings.ReplaceAll(stripped, "'}", "")

	if strings.Count(stripped, "{") != strings.Count(stripped, "}") {
		return fmt.Sprintf("line %d: unmatched braces in value for key %q", lineNumber, fullKey)
	}

	return ""
}

// duplicateKeyWarnings reports every fully-qualified key that was defined
// more than once in the file, in alphabetical order.
func duplicateKeyWarnings(seenAt map[string][]int) []string {
	var (
		keys []string
	)

	for key, lines := range seenAt {
		if len(lines) > 1 {
			keys = append(keys, key)
		}
	}

	sort.Strings(keys)

	warnings := make([]string, 0, len(keys))
	for _, key := range keys {
		warnings = append(warnings, fmt.Sprintf("duplicate key %q defined on lines %v", key, seenAt[key]))
	}

	return warnings
}

// render writes the blocks back out following the layout rules: comment
// blocks are preserved verbatim, section entries are sorted alphabetically
// by key with no blank lines between them, and a single blank line separates
// every block from the one before it.
func render(blocks []*block) []byte {
	var buf bytes.Buffer

	for index, b := range blocks {
		if index > 0 {
			buf.WriteString("\n")
		}

		switch b.kind {
		case commentBlockKind:
			for _, line := range b.comments {
				buf.WriteString(line)
				buf.WriteString("\n")
			}

		case sectionBlockKind:
			if b.hasHeader {
				buf.WriteString("[")
				buf.WriteString(b.header)
				buf.WriteString("]\n")
			}

			entries := append([]kvEntry(nil), b.entries...)
			sort.SliceStable(entries, func(i, j int) bool { return entries[i].key < entries[j].key })

			for _, e := range entries {
				buf.WriteString(e.key)
				buf.WriteString("=")
				buf.WriteString(e.value)
				buf.WriteString("\n")
			}
		}
	}

	return buf.Bytes()
}

// Format reorganizes a message file's contents according to the langlint
// rules, returning the new contents plus any non-fatal warnings. An error is
// returned only when the input cannot be safely parsed.
func Format(data []byte) ([]byte, []string, error) {
	result, err := parse(data)
	if err != nil {
		return nil, nil, err
	}

	return render(result.blocks), result.warnings, nil
}

// fileResult summarizes the outcome of linting a single file.
type fileResult struct {
	path     string
	changed  bool
	warnings []string
}

// lintFile reads and formats a single message file. In check mode the file
// is never modified; otherwise a changed file is rewritten in place.
func lintFile(path string, checkOnly bool) (fileResult, error) {
	result := fileResult{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}

	formatted, warnings, err := Format(data)
	if err != nil {
		return result, err
	}

	result.warnings = warnings
	result.changed = !bytes.Equal(data, formatted)

	if result.changed && !checkOnly {
		if err := rewriteFile(path, formatted); err != nil {
			return result, err
		}
	}

	return result, nil
}

// rewriteFile replaces path with newContent without ever leaving the
// original file in a partially-written state: the new content is written to
// a temporary file in the same directory, the original is moved aside, the
// temporary file is moved into place, and only then is the original removed.
func rewriteFile(path string, newContent []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".langlint-*")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()

	if _, err := tmp.Write(newContent); err != nil {
		tmp.Close()
		os.Remove(tmpName)

		return err
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)

		return err
	}

	if info, err := os.Stat(path); err == nil {
		_ = os.Chmod(tmpName, info.Mode())
	}

	backupName := path + ".langlint-bak"

	if err := os.Rename(path, backupName); err != nil {
		os.Remove(tmpName)

		return err
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Rename(backupName, path)

		return err
	}

	return os.Remove(backupName)
}
