package tasks

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"strings"

	"github.com/tucats/ego/internal/errors"
)

// jsonFieldPatch describes one top-level field to set (or insert) in a
// comment-preserving JSON/JSONC document -- see patchJSONFields. Value is
// marshaled with encoding/json to produce the exact bytes written to the
// file, so a Go string becomes a quoted JSON string and an int becomes a
// bare JSON number. A caller needing a field whose on-disk shape is a
// quoted string containing "true"/"false" (e.g. Task.Active, tagged
// `json:"active,string"`) passes that string directly as Value, not a
// bool.
type jsonFieldPatch struct {
	Key   string
	Value any
}

// patchJSONFields rewrites zero or more top-level fields of a JSONC
// document -- JSON with whole-line "#"/"//" comments, the format
// ui.ReadJSONFile strips before parsing (see docs/internals/TASKS.md) --
// and returns the patched bytes with every comment, every other field, and
// all original formatting otherwise untouched.
//
// For each patch: if the key already exists as a top-level field, only its
// value text is replaced -- the key itself, and the whitespace/colon
// separating it from the value, are left exactly as they were. If the key
// does not exist, a new "key": value entry is inserted just before the
// document's closing brace, indented to match the line above it.
//
// This deliberately only ever touches TOP-LEVEL fields of the outermost
// JSON object. A same-named key nested inside another field's value (for
// example a "count" key inside a task's own free-form "body" or
// "parameters", which are opaque to this function) is left alone, since it
// has nothing to do with the document's own top-level "count" field. A
// plain string or regex replace can't tell top-level and nested apart --
// that's why this function walks the document with encoding/json's own
// token scanner instead.
//
// This function is intentionally general-purpose: it knows nothing about
// tasks specifically, only about JSONC documents with whole-line comments,
// in case a caller elsewhere in the server ever needs to surgically patch
// another comment-preserving JSON document the same way.
func patchJSONFields(original []byte, patches []jsonFieldPatch) ([]byte, error) {
	if len(patches) == 0 {
		return original, nil
	}

	mapper := newCommentLineMapper(original)

	fields, insertAt, err := locateTopLevelFields(mapper.stripped)
	if err != nil {
		return nil, err
	}

	indent := detectIndent(original, mapper.toOriginal(insertAt))

	type edit struct {
		start, end int // byte offsets into `original`
		text       []byte
		order      int // patches[] index, for deterministic ordering of same-offset edits
	}

	edits := make([]edit, 0, len(patches))

	for i, patch := range patches {
		valueBytes, err := json.Marshal(patch.Value)
		if err != nil {
			return nil, errors.New(err)
		}

		if span, found := fields[patch.Key]; found {
			edits = append(edits, edit{
				start: mapper.toOriginal(span.valueStart),
				end:   mapper.toOriginal(span.valueEnd),
				text:  valueBytes,
				order: i,
			})
		} else {
			at := mapper.toOriginal(insertAt)

			text := []byte(",\n" + indent + `"` + patch.Key + `": `)
			text = append(text, valueBytes...)

			edits = append(edits, edit{start: at, end: at, text: text, order: i})
		}
	}

	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].start != edits[j].start {
			return edits[i].start < edits[j].start
		}

		return edits[i].order < edits[j].order
	})

	var out bytes.Buffer

	cursor := 0

	for _, e := range edits {
		out.Write(original[cursor:e.start])
		out.Write(e.text)

		cursor = e.end
	}

	out.Write(original[cursor:])

	return out.Bytes(), nil
}

// fieldValueSpan records where one top-level field's value text lives
// within the comment-stripped content: valueStart is the offset of the
// value's own first byte (the separating colon and any interstitial
// whitespace, captured alongside the value by encoding/json's token
// boundaries -- see locateTopLevelFields -- are excluded, so replacing
// [valueStart,valueEnd) leaves the key and the "colon + whitespace" that
// introduces its value exactly as they were), and valueEnd is the offset
// immediately following the value's last byte.
type fieldValueSpan struct {
	valueStart int
	valueEnd   int
}

// locateTopLevelFields scans stripped -- a comment-free JSON document --
// and returns the value span of every top-level (depth-1) field, keyed by
// field name, plus the offset at which a new top-level field could be
// inserted (immediately before the document's closing brace).
//
// It uses encoding/json's own streaming token scanner (json.Decoder,
// walked via Token()/InputOffset()) rather than a hand-rolled parser, so
// string escaping, nested braces/brackets inside string values, and
// Unicode are all handled exactly the way encoding/json itself would parse
// them -- the same guarantee a regex-based scan cannot offer.
func locateTopLevelFields(stripped []byte) (fields map[string]fieldValueSpan, insertAt int, err error) {
	dec := json.NewDecoder(bytes.NewReader(stripped))

	fields = make(map[string]fieldValueSpan)
	insertAt = -1

	depth := 0
	haveKey := false

	var pendingKey string

	for {
		before := dec.InputOffset()

		tok, tokErr := dec.Token()
		if tokErr == io.EOF {
			break
		}

		if tokErr != nil {
			return nil, 0, errors.New(tokErr)
		}

		after := dec.InputOffset()

		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{', '[':
				if depth == 0 && delim != '{' {
					return nil, 0, errors.New(errors.ErrTasksInvalidField).Context("document root is not an object")
				}

				depth++
			case '}', ']':
				depth--

				if depth == 0 && insertAt == -1 {
					insertAt = int(before)
				}

				if depth == 1 {
					// A compound value (object/array) belonging to the
					// pending key has just closed back down to top level.
					haveKey = false
				}
			}

			continue
		}

		if depth != 1 {
			continue
		}

		if !haveKey {
			key, isString := tok.(string)
			if !isString {
				return nil, 0, errors.New(errors.ErrTasksInvalidField).Context("non-string object key")
			}

			pendingKey = key
			haveKey = true

			continue
		}

		// Scalar value for pendingKey. `before` marks the start of this
		// token's captured span, which -- per json.Decoder's InputOffset
		// boundary semantics -- begins at the separating colon, not the
		// value itself (e.g. for `"count": 4`, the value token's span is
		// `: 4`, not just `4`). Skip past the colon and any interstitial
		// whitespace so only the true value is replaced, leaving the key
		// and its formatting untouched.
		valueStart := skipColonAndWhitespace(stripped, int(before))

		fields[pendingKey] = fieldValueSpan{valueStart: valueStart, valueEnd: int(after)}
		haveKey = false
	}

	if insertAt == -1 {
		return nil, 0, errors.New(errors.ErrTasksInvalidField).Context("document has no top-level object")
	}

	return fields, insertAt, nil
}

// commentLineMapper maps a byte offset in stripped -- the document with
// whole-line "#"/"//" comments removed, exactly as ui.ReadJSONFile
// produces it -- back to the corresponding byte offset in the original
// document. Comment stripping only ever removes entire lines, so every
// byte that survives into stripped is byte-identical to, and at the same
// within-line column as, its counterpart in original; only the line
// number shifts, by however many comment lines preceded it.
type commentLineMapper struct {
	original []byte
	stripped []byte

	// origLineStart[i] is the byte offset in original where its i'th line
	// (0-based, split on '\n') begins.
	origLineStart []int

	// strippedLineStart[i] is the byte offset in stripped where its i'th
	// kept line begins.
	strippedLineStart []int

	// strippedLineOrigIndex[i] is the index into origLineStart of the
	// original line that stripped's i'th kept line came from.
	strippedLineOrigIndex []int
}

func newCommentLineMapper(original []byte) *commentLineMapper {
	origLines := strings.Split(string(original), "\n")
	origLineStart := lineStartOffsets(origLines)

	keptLines := make([]string, 0, len(origLines))
	keptOrigIndex := make([]int, 0, len(origLines))

	for i, line := range origLines {
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		keptLines = append(keptLines, line)
		keptOrigIndex = append(keptOrigIndex, i)
	}

	stripped := strings.Join(keptLines, "\n")

	return &commentLineMapper{
		original:              original,
		stripped:              []byte(stripped),
		origLineStart:         origLineStart,
		strippedLineStart:     lineStartOffsets(keptLines),
		strippedLineOrigIndex: keptOrigIndex,
	}
}

// toOriginal maps a byte offset within m.stripped to the corresponding
// byte offset within m.original.
func (m *commentLineMapper) toOriginal(strippedOffset int) int {
	// Find the last line whose start is <= strippedOffset. A linear scan
	// is fine here: task files are at most a few dozen lines.
	line := 0

	for line+1 < len(m.strippedLineStart) && m.strippedLineStart[line+1] <= strippedOffset {
		line++
	}

	delta := strippedOffset - m.strippedLineStart[line]
	origLine := m.strippedLineOrigIndex[line]

	return m.origLineStart[origLine] + delta
}

// lineStartOffsets returns, for a slice of lines produced by splitting
// some byte slice on '\n' (the separators themselves not included in any
// line), the byte offset at which each line begins within that original
// byte slice.
func lineStartOffsets(lines []string) []int {
	starts := make([]int, len(lines))
	offset := 0

	for i, line := range lines {
		starts[i] = offset
		offset += len(line) + 1 // +1 for the '\n' separator
	}

	return starts
}

// detectIndent returns the leading whitespace of the line containing the
// last non-whitespace byte before offset in original -- used to match a
// newly-inserted field's indentation to the field above it. Falls back to
// a single tab if offset is at (or near) the start of the document.
func detectIndent(original []byte, offset int) string {
	i := offset - 1
	for i >= 0 && isJSONWhitespace(original[i]) {
		i--
	}

	if i < 0 {
		return "\t"
	}

	lineStart := i
	for lineStart > 0 && original[lineStart-1] != '\n' {
		lineStart--
	}

	j := lineStart
	for j < len(original) && (original[j] == ' ' || original[j] == '\t') {
		j++
	}

	if j == lineStart {
		return "\t"
	}

	return string(original[lineStart:j])
}

func isJSONWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// skipColonAndWhitespace advances offset past any whitespace, then a
// single ':' (if present), then any further whitespace, returning the
// resulting offset. Used to trim a value token's captured span -- which
// json.Decoder's InputOffset boundaries include the separating colon and
// any whitespace on either side of it within (e.g. `"count"  :4`'s value
// token spans "  :4", not just "4") -- down to just the value itself.
func skipColonAndWhitespace(data []byte, offset int) int {
	i := offset

	for i < len(data) && isJSONWhitespace(data[i]) {
		i++
	}

	if i < len(data) && data[i] == ':' {
		i++
	}

	for i < len(data) && isJSONWhitespace(data[i]) {
		i++
	}

	return i
}
