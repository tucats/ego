package tables

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

// Print writes the formatted table to standard output. The format argument
// must be one of the ui package constants:
//
//	ui.TextFormat         — fixed-width text with column headings
//	ui.JSONFormat         — compact JSON array printed on one line
//	ui.JSONIndentedFormat — pretty-printed JSON array
//
// Passing an empty string selects TextFormat. Any other value returns
// ErrInvalidOutputFormat. If a sort column has been set via SetOrderBy,
// the rows are sorted before output.
func (t *Table) Print(format string) error {
	// If there is an orderBy set for the table, do the sort now
	if t.orderBy >= 0 {
		_ = t.SortRows(t.orderBy, t.ascending)
	}

	if format == "" {
		format = ui.TextFormat
	}

	// Based on the selected format, generate the output
	switch format {
	case ui.TextFormat:
		s := t.FormatText()
		for _, line := range s {
			fmt.Printf("%s\n", line)
		}

	case ui.JSONFormat:
		fmt.Printf("%s\n", t.FormatJSON())

	case ui.JSONIndentedFormat:
		text := t.FormatJSON()

		var i any

		_ = json.Unmarshal([]byte(text), &i)
		b, _ := json.MarshalIndent(i, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		fmt.Printf("%s\n", string(b))

	default:
		return errors.ErrInvalidOutputFormat.Context(format)
	}

	return nil
}

// String returns the formatted table as a string. It accepts the same format
// constants as Print (ui.TextFormat, ui.JSONFormat, ui.JSONIndentedFormat)
// and applies the same defaulting and sorting behavior. Use String instead
// of Print when you need the output as a value rather than writing to stdout.
func (t *Table) String(format string) (string, error) {
	// If there is an orderBy set for the table, do the sort now
	if t.orderBy >= 0 {
		_ = t.SortRows(t.orderBy, t.ascending)
	}

	if format == "" {
		format = ui.TextFormat
	}

	var b strings.Builder

	// Based on the selected format, generate the output
	switch format {
	case ui.TextFormat:
		s := t.FormatText()
		for _, line := range s {
			b.WriteString(fmt.Sprintf("%s\n", line))
		}

	case ui.JSONFormat:
		b.WriteString(t.FormatJSON())

	case ui.JSONIndentedFormat:
		text := t.FormatJSON()

		var i any

		_ = json.Unmarshal([]byte(text), &i)
		buf, _ := json.MarshalIndent(i, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		b.WriteString(string(buf))
		b.WriteString("\n")

	default:
		return "", errors.ErrInvalidOutputFormat.Context(format)
	}

	return b.String(), nil
}

// FormatJSON returns the table as a compact JSON array of objects. Each
// object represents one row; the keys are the column names in display order
// (as set by SetColumnOrder or SetColumnOrderByName).
//
// Type inference at output time:
//   - If a cell value parses as an integer with egostrings.Atoi, it is
//     emitted as a bare JSON number (no quotes).
//   - If a cell value is the literal string "true" or "false" (defs.True /
//     defs.False), it is emitted as a JSON boolean.
//   - All other values are emitted as JSON strings (with double-quote
//     escaping via escape()).
//
// RowLimit and SetStartingRow are respected. An empty table returns "[]".
func (t *Table) FormatJSON() string {
	var (
		buffer   strings.Builder
		firstRow = true
	)

	buffer.WriteRune('[')

	for n, row := range t.rows {
		if n < t.startingRow {
			continue
		}

		if t.rowLimit > 0 && n >= t.startingRow+t.rowLimit {
			break
		}

		if !firstRow {
			buffer.WriteRune(',')
		}

		firstRow = false

		buffer.WriteRune('{')

		for ith, i := range t.columnOrder {
			header := t.names[i]

			if ith > 0 {
				buffer.WriteRune(',')
			}

			buffer.WriteRune('"')
			buffer.WriteString(header)
			buffer.WriteString("\":")

			if _, valid := egostrings.Atoi(row[i]); valid == nil {
				buffer.WriteString(row[i])
			} else if row[i] == defs.True || row[i] == defs.False {
				buffer.WriteString(row[i])
			} else {
				buffer.WriteString(strconv.Quote(escape(row[i])))
			}
		}

		buffer.WriteRune('}')
	}

	buffer.WriteRune(']')

	return buffer.String()
}

// FormatIndented returns the table as a pretty-printed JSON array. The
// structure and type-inference rules are identical to FormatJSON; the only
// difference is that newlines and three-space indentation are inserted to
// make the output human-readable.
func (t *Table) FormatIndented() string {
	var (
		buffer   strings.Builder
		firstRow = true
	)

	buffer.WriteString("[\n   ")

	for n, row := range t.rows {
		if n < t.startingRow {
			continue
		}

		if t.rowLimit > 0 && n >= t.startingRow+t.rowLimit {
			break
		}

		if !firstRow {
			buffer.WriteString(",\n   ")
		}

		firstRow = false

		buffer.WriteString("{\n      ")

		for ith, i := range t.columnOrder {
			header := t.names[i]

			if ith > 0 {
				buffer.WriteString(",\n      ")
			}

			buffer.WriteRune('"')
			buffer.WriteString(header)
			buffer.WriteString("\":")

			if _, valid := egostrings.Atoi(row[i]); valid == nil {
				buffer.WriteString(row[i])
			} else if row[i] == defs.True || row[i] == defs.False {
				buffer.WriteString(row[i])
			} else {
				buffer.WriteString(strconv.Quote(escape(row[i])))
			}
		}

		buffer.WriteString("\n   }")
	}

	buffer.WriteString("\n]\n")

	return buffer.String()
}

// SetPagination overrides the terminal dimensions used for column folding and
// vertical pagination. Pass (0, 0) to disable pagination entirely (FormatText
// will use simple linear output). Negative values are ignored; the existing
// dimension is kept when a negative argument is passed. Returns the receiver.
func (t *Table) SetPagination(height, width int) *Table {
	if height >= 0 {
		t.terminalHeight = height
	}

	if width >= 0 {
		t.terminalWidth = width
	}

	return t
}

// FormatText returns the table as a slice of strings, one element per output
// line. Each line ends without a newline character; callers are responsible
// for adding line endings.
//
// When terminalHeight or terminalWidth is non-zero, the output is routed
// through paginateText, which folds wide tables into multiple column groups.
// Call SetPagination(0, 0) before FormatText to suppress this behavior.
//
// In non-paginated mode the line order is:
//  1. Heading row (if ShowHeadings is true)
//  2. Underline row (if ShowUnderlines and ShowHeadings are both true)
//  3. One line per data row (starting at startingRow, limited by rowLimit)
func (t *Table) FormatText() []string {
	var buffer strings.Builder

	var rowLimit = t.rowLimit

	if (t.terminalHeight > 0) || (t.terminalWidth > 0) {
		return t.RenderPagelets()
	}

	ui.Log(ui.AppLogger, "app.table.column.order", ui.A{
		"columns": t.columnOrder})

	output := make([]string, 0)

	if rowLimit < 0 {
		rowLimit = len(t.rows)
	}

	if t.showHeadings {
		buffer.WriteString(t.indent)

		rowString := i18n.L("Row")

		if t.showRowNumbers {
			buffer.WriteString(rowString)
			buffer.WriteString(t.spacing)
		}

		for _, n := range t.columnOrder {
			buffer.WriteString(AlignText(t.names[n], t.maxWidth[n], t.alignment[n]))
			buffer.WriteString(t.spacing)
		}

		output = append(output, buffer.String())

		if t.showUnderlines {
			buffer.Reset()
			buffer.WriteString(t.indent)

			if t.showRowNumbers {
				buffer.WriteString(strings.Repeat("=", len(rowString)))
				buffer.WriteString(t.spacing)
			}

			for _, n := range t.columnOrder {
				for pad := 0; pad < t.maxWidth[n]; pad++ {
					buffer.WriteRune('=')
				}

				buffer.WriteString(t.spacing)
			}

			output = append(output, buffer.String())
		}
	}

	for i, r := range t.rows {
		if i < t.startingRow {
			continue
		}

		if i >= t.startingRow+rowLimit {
			break
		}

		buffer.Reset()
		buffer.WriteString(t.indent)

		if t.showRowNumbers {
			buffer.WriteString(fmt.Sprintf("%3d", i+1))
			buffer.WriteString(t.spacing)
		}

		// Loop over the elements of the row. Generate pre- or post-spacing as
		// appropriate for the requested alignment, and any intra-column spacing.
		for _, n := range t.columnOrder {
			buffer.WriteString(AlignText(r[n], t.maxWidth[n], t.alignment[n]))
			buffer.WriteString(t.spacing)
		}

		output = append(output, buffer.String())
	}

	return output
}

// AlignText pads or truncates text to exactly width rune positions using the
// specified alignment constant. It is Unicode-safe: multi-byte characters
// count as one position each.
//
//   - AlignmentLeft  — text is left-aligned; spaces are appended on the right.
//   - AlignmentRight — text is right-aligned; spaces are prepended on the left.
//   - AlignmentCenter — equal padding on each side; when padding is odd, the
//     extra space goes on the right.
//
// When textLength >= width the text is truncated:
//   - Left:   the first width runes
//   - Right:  the last width runes
//   - Center: width runes centered around the middle of the text
//
// A width of 0 returns an empty string.
func AlignText(text string, width int, alignment int) string {
	runes := []rune{}
	for _, ch := range text {
		runes = append(runes, ch)
	}

	textLength := len(runes)

	if textLength >= width {
		switch alignment {
		case AlignmentLeft:
			return string(runes[:width])

		case AlignmentRight:
			return string(runes[textLength-width:])

		case AlignmentCenter:
			pos := textLength/2 - (width / 2)

			return string(runes[pos : pos+width])

		default:
			return string(runes[:width])
		}
	}

	// Make an array of the pad character as runes to be sliced
	// along with the source text as runes.
	padRunes := make([]rune, width)
	for i := range padRunes {
		padRunes[i] = ' '
	}

	// Based on alignment, do the right thing.
	switch alignment {
	case AlignmentRight:
		r := append(padRunes, runes...)

		return string(r[len(r)-width:])

	case AlignmentLeft:
		r := append(runes, padRunes...)

		return string(r[:width])

	case AlignmentCenter:
		left := (width - textLength) / 2
		right := width - (left + textLength)

		if left+right+textLength != width {
			right = width - left
		}

		r := []rune{}
		r = append(r, padRunes[:left]...)
		r = append(r, runes...)
		r = append(r, padRunes[:right]...)

		return string(r)

	default: // same as AlignmentLeft
		r := append(runes, padRunes...)

		return string(r[:width])
	}
}

// escape replaces every double-quote character in s with the two-character
// sequence \" so the result can be safely embedded inside a JSON string.
// Note: strconv.Quote wraps the whole string in outer quotes and also escapes
// other control characters. FormatJSON calls strconv.Quote(escape(s)) so the
// outer quotes come from strconv.Quote while the inner quotes are pre-escaped
// here to avoid double-escaping.
func escape(s string) string {
	result := strings.Builder{}

	for _, ch := range s {
		if ch == '"' {
			result.WriteString("\\\"")
		} else {
			result.WriteRune(ch)
		}
	}

	return result.String()
}
