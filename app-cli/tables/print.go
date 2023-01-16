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
	var buffer strings.Builder

	buffer.WriteRune('[')

	firstRow := true

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

func (t *Table) SetPagination(height, width int) {
	if height >= 0 {
		t.terminalHeight = height
	}

	if width >= 0 {
		t.terminalWidth = width
	}
}

// paginateText will output a table with column folding and pagination.
func (t *Table) paginateText() []string {
	var headers []strings.Builder

	headerCount := 0
	rowCount := len(t.rows)

	if t.startingRow > 0 {
		rowCount = rowCount - t.startingRow
	}

	if t.rowLimit > 0 {
		rowCount = rowCount - (len(t.rows) - t.rowLimit)
	}

	columnMap := make([]int, len(t.columnOrder))
	headerIndex := 0
	columnIndex := 0
	headers = make([]strings.Builder, 1)

	// @tomcole this is a hack to turn off vertical pagination until it
	// is working correctly.
	savedTerminalHeight := t.terminalHeight
	defer func() {
		t.terminalHeight = savedTerminalHeight
	}()

	// Temporarily set to a ridiculously huge number
	// t.terminalHeight = 9999999
	ui.Log(ui.DebugLogger, "terminal height is %d lines\n", t.terminalHeight)

	// Do we need to include the Row header first?
	availableWidth := t.terminalWidth
	rowNumberWidth := 0

	if t.showRowNumbers {
		rowNumberWidth = len(fmt.Sprintf("%d", len(t.rows)))
		if rowNumberWidth < 3 {
			rowNumberWidth = 3
		}

		availableWidth = availableWidth - (rowNumberWidth + len(t.spacing))
	}

	first := true

	// Build the headings map.
	for i, n := range t.columnOrder {
		w := t.maxWidth[n]

		if headers[headerIndex].Len()+len(t.spacing)+w > availableWidth {
			headerIndex++
			headerCount++

			if t.showUnderlines && t.showHeadings {
				headers = append(headers, strings.Builder{})

				columnIndexes := t.columnOrder[columnIndex:i]

				if rowNumberWidth > 0 {
					for pad := 0; pad < rowNumberWidth; pad++ {
						headers[headerIndex].WriteRune('=')
					}

					headers[headerIndex].WriteString(t.spacing)
				}

				for _, h := range columnIndexes {
					for pad := 0; pad < t.maxWidth[h]; pad++ {
						headers[headerIndex].WriteRune('=')
					}

					headers[headerIndex].WriteString(t.spacing)
				}

				columnIndex = i
				headerIndex++
			}

			if t.showHeadings {
				headers = append(headers, strings.Builder{})
				if rowNumberWidth > 0 {
					headers[headerIndex].WriteString(i18n.L("Row"))

					for pad := 0; pad < rowNumberWidth-3; pad++ {
						headers[headerIndex].WriteRune(' ')
					}

					headers[headerIndex].WriteString(t.spacing)
				}
			}
		}

		columnMap[i] = headerCount

		if t.showHeadings {
			if first && rowNumberWidth > 0 {
				headers[headerIndex].WriteString(i18n.L("Row"))

				for pad := 0; pad < rowNumberWidth-3; pad++ {
					headers[headerIndex].WriteRune(' ')
				}

				headers[headerIndex].WriteString(t.spacing)
			}

			first = false

			headers[headerIndex].WriteString(AlignText(t.names[n], t.maxWidth[n], t.alignment[n]))
			headers[headerIndex].WriteString(t.spacing)
		}
	}

	// If we're outputting underlines, then we have to set up the final row of
	// underlines from the last set of headers.
	if t.showHeadings && t.showUnderlines {
		headers = append(headers, strings.Builder{})
		headerIndex++

		if rowNumberWidth > 0 {
			for pad := 0; pad < rowNumberWidth; pad++ {
				headers[headerIndex].WriteRune('=')
			}

			headers[headerIndex].WriteString(t.spacing)
		}

		columnIndexes := t.columnOrder[columnIndex:]
		for _, h := range columnIndexes {
			for pad := 0; pad < t.maxWidth[h]; pad++ {
				headers[headerIndex].WriteRune('=')
			}

			headers[headerIndex].WriteString(t.spacing)
		}
	}

	rowLimit := t.rowLimit
	headerCount++

	pageletSize := rowCount
	pageletCount := headerCount

	pagelets := make([][]string, pageletCount)
	for i := range pagelets {
		pagelets[i] = make([]string, pageletSize)

		if rowNumberWidth > 0 {
			for j := 1; j <= pageletSize; j++ {
				rowString := fmt.Sprintf("%d", j)
				for n := 0; n <= rowNumberWidth-len(rowString); n++ {
					rowString = " " + rowString
				}

				pagelets[i][j-1] = rowString + t.spacing
			}
		}
	}

	if rowLimit < 0 {
		rowLimit = t.terminalHeight
	}

	output := []string{}

	// Now select rows.
	for rx, r := range t.rows {
		if rx < t.startingRow {
			continue
		}

		if rx >= t.startingRow+rowLimit {
			break
		}

		// Loop over the elements of the row. Generate pre- or post-spacing as
		// appropriate for the requested alignment, and any intra-column spacing.
		for cx, n := range t.columnOrder {
			px := columnMap[cx] % pageletCount

			text := AlignText(r[n], t.maxWidth[n], t.alignment[n]) + t.spacing
			pagelets[px][rx] = pagelets[px][rx] + text
		}
	}

	// Calculate how many lines are in each pagelet print block. Make sure that if this
	// goes to just a single row, turn off pagination entirely for this output because the
	// display is just too darn small.
	printBlockSize := ((t.terminalHeight - (headerCount * pageletCount)) / pageletCount) - 1
	if printBlockSize <= headerCount {
		printBlockSize = rowCount
	}

	ui.Log(ui.AppLogger, "There are %d pagelets", pageletCount)
	ui.Log(ui.AppLogger, "There are %d lines in each pagelet", pageletSize)
	ui.Log(ui.AppLogger, "Each print block is %d lines", printBlockSize)

	// @tomcole need to rethink this loop. Really probably needs to scan by
	// lines count in a pagelet, and then append into the output as needed.

	// reassemble into a page buffer.
	for px, p := range pagelets {
		// Get the header (and optionally, underline) for this pagelet
		hx := px
		if t.showUnderlines {
			hx = hx * 2
		}

		output = append(output, headers[hx].String())
		if t.showUnderlines {
			output = append(output, headers[hx+1].String())
		}

		// Add the rows for this pagelet
		for _, r := range p {
			if r != "" {
				output = append(output, r)
			}
		}

		// Add a blank between pagelets
		output = append(output, "")
	}

	return output
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
