package tables

import (
	"strconv"
	"strings"
)

// RenderPagelets renders the table in pagelets based on terminal width and height. This is
// the "new" paginator. This currently does not appear to handle width correctly, but does
// break output by height into pages correctly.
func (t *Table) RenderPagelets() []string {
	// If no rows, return empty pagelet
	if len(t.rows) == 0 {
		return []string{""}
	}

	// If we were called and the output dimensions are undefined, assume old-school terminal window sizes.
	if t.terminalWidth <= 0 {
		t.terminalWidth = 80
	}

	if t.terminalHeight <= 0 {
		t.terminalHeight = 24
	}

	// Make a saved copy of all the table data, because we're going to mess with in in-place to
	// format the pagelets, and then we need to be able to put it back, thank-you-very-much.
	savedRows := [][]string{}

	savedColumnCount := t.columnCount
	savedNames := make([]string, len(t.names))
	copy(savedNames, t.names)

	savedAlignments := make([]int, len(t.alignment))
	copy(savedAlignments, t.alignment)

	for i, row := range t.rows {
		savedRows = append(savedRows, []string{})
		for _, column := range row {
			savedRows[i] = append(savedRows[i], column)
		}
	}

	// When we're done, restore the data we damanged during formatting...
	defer func() {
		t.rows = savedRows
		t.names = savedNames
		t.alignment = savedAlignments
		t.columnCount = savedColumnCount
	}()

	// Calculate column widths and apply padding and alignment to the column data
	colWidths := t.normalizeWidths()

	var pagelets []string

	// Calculate available width for content (accounting for spacing, indentation, etc.)
	availableWidth := t.terminalWidth - len(t.indent) - len(t.spacing) // for padding

	// Determine which columns fit in available width
	nextColumnStarts := 0

	for {
		fittingColumns := t.determineFittingColumns(nextColumnStarts, colWidths, availableWidth)

		// Split rows into pagelets based on terminal height
		var currentPageRows [][]string

		currentPageHeight := 0

		// Process rows in chunks based on terminal height
		for _, row := range t.rows {
			// Calculate row height based on content and column widths
			rowHeight := t.calculateRowHeight(row, colWidths, fittingColumns)

			// Check if adding this row would exceed terminal height
			if len(currentPageRows) > 0 && currentPageHeight+rowHeight > t.terminalHeight-1 {
				// Render current pagelet
				pagelets = append(pagelets, t.renderPagelet(currentPageRows, colWidths, fittingColumns))

				// Start new pagelet with current row
				currentPageRows = [][]string{row}
				currentPageHeight = 0

				// Add the height of this row to the new pagelet
				currentPageHeight += rowHeight
			} else {
				// Add row to current pagelet
				currentPageRows = append(currentPageRows, row)
				currentPageHeight += rowHeight
			}
		}

		// Render remaining rows as last pagelet
		if len(currentPageRows) > 0 {
			pagelets = append(pagelets, t.renderPagelet(currentPageRows, colWidths, fittingColumns))
		}

		pos := len(fittingColumns) - 1
		if pos < 1 {
			break
		}

		lastOne := fittingColumns[pos]
		if lastOne >= t.columnCount {
			break
		}

		nextColumnStarts = lastOne + 1
	}

	return pagelets
}

// determineFittingColumns determines which columns can fit within terminal width, starting at
// the specified column number (0 for the first one).
func (t *Table) determineFittingColumns(starting int, colWidths []int, availableWidth int) []int {
	var fittingColumns []int

	currentWidth := 0

	// Add columns one by one until we exceed available width
	for i, width := range colWidths {
		if i < starting {
			continue
		}
		// Add spacing between columns
		if i > 0 {
			currentWidth += len(t.spacing)
		}

		// Add column width and padding (assuming 1 space on each side)
		currentWidth += width + len(t.spacing)

		if currentWidth <= availableWidth {
			fittingColumns = append(fittingColumns, i)
		} else {
			break
		}
	}

	return fittingColumns
}

// calculateRowHeight calculates how many lines a row will take up when rendered.
func (t *Table) calculateRowHeight(row []string, colWidths []int, fittingColumns []int) int {
	maxLines := 1

	// For each fitting column, calculate how many lines it will take up
	for _, colIndex := range fittingColumns {
		if colIndex < len(row) {
			content := row[colIndex]
			// Calculate how many lines this column's content will take up
			if len(content) > colWidths[colIndex] {
				// Assuming each line is limited by column width
				lines := (len(content) + colWidths[colIndex] - 1) / colWidths[colIndex]
				if lines > maxLines {
					maxLines = lines
				}
			}
		}
	}

	return maxLines
}

// renderPagelet renders a single pagelet with the specified columns.
func (t *Table) renderPagelet(pageRows [][]string, widths []int, fittingColumns []int) string {
	var result strings.Builder

	// Add indentation to each line
	indent := t.indent

	if t.showHeadings && len(t.names) > 0 {
		// Render headers
		headerLine := t.renderRow(t.names, widths, fittingColumns)
		result.WriteString(indent + headerLine + "\n")

		// Add underline if requested
		if t.showUnderlines {
			underline := t.renderUnderline(widths, fittingColumns)
			result.WriteString(indent + underline + "\n")
		}
	}

	// Render rows
	for _, row := range pageRows {
		line := t.renderRow(row, widths, fittingColumns)
		result.WriteString(indent + line + "\n")
	}

	return result.String()
}

// renderRow renders a single row with only the fitting columns.
func (t *Table) renderRow(row []string, widths, fittingColumns []int) string {
	var result strings.Builder

	for i, colIndex := range fittingColumns {
		if i > 0 {
			result.WriteString("  ") // Two spaces between columns
		}

		if colIndex < len(row) {
			content := AlignText(row[colIndex], widths[i], t.alignment[i])
			result.WriteString(content)
		} else {
			result.WriteString("")
		}
	}

	return result.String()
}

// renderUnderline renders the underline for headers.
func (t *Table) renderUnderline(widths, fittingColumns []int) string {
	var result strings.Builder

	for i, colIndex := range fittingColumns {
		if i > 0 {
			result.WriteString("  ")
		}

		// Create underline with dashes
		if colIndex < len(t.names) {
			length := widths[colIndex]
			underline := strings.Repeat("=", length)
			result.WriteString(underline)
		} else {
			result.WriteString("")
		}
	}

	return result.String()
}

func (t *Table) normalizeWidths() []int {
	if len(t.rows) == 0 {
		return make([]int, t.columnCount)
	}

	// If we are adding in the row count, update the headers and rows first.
	if t.showRowNumbers {
		t.names = append([]string{"Row"}, t.names...)
		t.alignment = append([]int{AlignmentRight}, t.alignment...)

		for i, row := range t.rows {
			t.rows[i] = append([]string{strconv.Itoa(i + 1)}, row...)
		}
	}

	widths := make([]int, len(t.names))

	// Initialize the width values with the widths of the columns.
	for i, name := range t.names {
		widths[i] = len(name)
	}

	// Loop over the columns and see if the data elements update the width value.
	for _, row := range t.rows {
		for j, column := range row {
			if widths[j] < len(column) {
				widths[j] = len(column)
			}
		}
	}

	// Now pad and align each value based on the column widths
	for i, row := range t.rows {
		for j, column := range row {
			t.rows[i][j] = AlignText(column, widths[j], t.alignment[j])
		}
	}

	return widths
}
