package tables

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

// Print will output a table using current rows and format specifications.
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

		var i interface{}

		_ = json.Unmarshal([]byte(text), &i)
		b, _ := json.MarshalIndent(i, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		fmt.Printf("%s\n", string(b))

	default:
		return errors.ErrInvalidOutputFormat.Context(format)
	}

	return nil
}

// String will output a table using current rows and format specifications.
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

		var i interface{}

		_ = json.Unmarshal([]byte(text), &i)
		buf, _ := json.MarshalIndent(i, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		b.WriteString(string(buf))
		b.WriteString("\n")

	default:
		return "", errors.ErrInvalidOutputFormat.Context(format)
	}

	return b.String(), nil
}

// FormatJSON will produce the text of the table as JSON.
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

			if _, valid := strconv.Atoi(row[i]); valid == nil {
				buffer.WriteString(row[i])
			} else if row[i] == defs.True || row[i] == defs.False {
				buffer.WriteString(row[i])
			} else {
				buffer.WriteString("\"" + escape(row[i]) + "\"")
			}
		}

		buffer.WriteRune('}')
	}

	buffer.WriteRune(']')

	return buffer.String()
}

func (t *Table) SetPagination(height, width int) *Table {
	if height >= 0 {
		t.terminalHeight = height
	}

	if width >= 0 {
		t.terminalWidth = width
	}

	return t
}

// FormatText will output a table using current rows and format specifications.
func (t *Table) FormatText() []string {
	var buffer strings.Builder

	var rowLimit = t.rowLimit

	if (t.terminalHeight > 0) || (t.terminalWidth > 0) {
		return t.paginateText()
	}

	ui.Log(ui.AppLogger, "Print column order: %v", t.columnOrder)

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

// AlignText aligns a string to a given width and alignment. This
// is used to manage columns once the contents are formatted. This
// is Unicode-safe.
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

	// Based on aligntment, do the right thing.
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

// Helper function for formatting JSON output so quotes
// are properly escaped.
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
