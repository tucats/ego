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

	// Make a saved copy of all the table data, because we're going to mess with it in-place to
	// format the pagelets, and then we need to be able to put it back, thank-you-very-much.
	savedRows := make([][]string, len(t.rows))
	for i, row := range t.rows {
		savedRows[i] = append([]string{}, row...)
	}

	savedColumnCount := t.columnCount
	savedNames := append([]string{}, t.names...)
	savedAlignments := append([]int{}, t.alignment...)

	// When we're done, restore the data we damaged during formatting.
	defer func() {
		t.rows = savedRows
		t.names = savedNames
		t.alignment = savedAlignments
		t.columnCount = savedColumnCount
	}()

	// normalizeWidths pads/aligns every cell in t.rows in-place and returns
	// the maximum character width for each column.
	colWidths := t.normalizeWidths()

	var pagelets []string

	// availableWidth is the number of characters we can fill per line,
	// after subtracting the left-side indentation.
	availableWidth := t.terminalWidth - len(t.indent) - len(t.spacing) // for padding

	// nextColumnStarts is the index of the first column that has not yet
	// been placed on a width-slice. We keep looping, advancing this index,
	// until we have emitted every column.
	nextColumnStarts := 0

	for {
		// Figure out which contiguous run of columns (starting at nextColumnStarts)
		// fits within the available terminal width.
		fittingColumns := t.determineFittingColumns(nextColumnStarts, colWidths, availableWidth)

		// Split rows into pagelets based on terminal height.
		var currentPageRows [][]string

		// Start the height counter at the space already consumed by the
		// column headings and the underline row (0, 1, or 2 lines depending
		// on the table's showHeadings / showUnderlines flags).
		currentPageHeight := t.startingRowOnPage()

		// Walk every data row and decide whether it fits on the current page
		// or needs to start a new one.
		for _, row := range t.rows {
			// Determine how many terminal lines this row occupies. Most rows
			// are exactly 1 line, but very long cell values can wrap.
			rowHeight := t.calculateRowHeight(row, colWidths, fittingColumns)

			// If we already have rows collected AND adding this row would push
			// the page past the terminal height, flush the current page and
			// start a new one.
			if len(currentPageRows) > 0 && currentPageHeight+rowHeight > t.terminalHeight-1 {
				// Render the accumulated rows as a single pagelet string.
				pagelets = append(pagelets, t.renderPagelet(currentPageRows, colWidths, fittingColumns))

				// Start a fresh page. The new page will also show headings, so
				// initialize its height counter the same way we did at the top.
				currentPageRows = [][]string{row}
				currentPageHeight = t.startingRowOnPage()

				// Count the first row on the new page.
				currentPageHeight += rowHeight
			} else {
				// Row fits on the current page — just add it.
				currentPageRows = append(currentPageRows, row)
				currentPageHeight += rowHeight
			}
		}

		// Render whatever rows are still pending as the final pagelet for
		// this width-slice.
		if len(currentPageRows) > 0 {
			pagelets = append(pagelets, t.renderPagelet(currentPageRows, colWidths, fittingColumns))
		}

		// Decide whether there are more columns to display in a subsequent
		// width-slice. If the last column we showed is at or beyond the final
		// column index, we are done.
		pos := len(fittingColumns) - 1
		if pos < 0 {
			// No columns fit at all — nothing left to render.
			break
		}

		lastOne := fittingColumns[pos]
		if lastOne >= t.columnCount-1 {
			// The last displayed column is the final column in the table.
			break
		}

		// Advance past the columns we just rendered.
		nextColumnStarts = lastOne + 1
	}

	return pagelets
}

// startingRowOnPage returns the number of lines that the heading block
// (column names + optional underline) will consume at the top of every
// pagelet. This lets the height-budget calculation reserve those lines
// before counting data rows.
func (t *Table) startingRowOnPage() int {
	currentPageHeight := 0

	if t.showHeadings {
		currentPageHeight++

		// Underlines are only emitted when headings are also on (see renderPagelet).
		// Counting them when headings are off would over-reserve the height budget.
		if t.showUnderlines {
			currentPageHeight++
		}
	}

	return currentPageHeight
}

// determineFittingColumns determines which columns can fit within terminal width, starting at
// the specified column number (0 for the first one).
func (t *Table) determineFittingColumns(starting int, colWidths []int, availableWidth int) []int {
	var fittingColumns []int

	currentWidth := 0

	// Walk the columns in order, accumulating their rendered widths.
	// We stop as soon as adding the next column would exceed the terminal width.
	for i, width := range colWidths {
		// Skip columns that belong to an earlier width-slice.
		if i < starting {
			continue
		}

		// Add the inter-column spacing gap, but only between columns — not
		// before the very first column in this slice.
		if len(fittingColumns) > 0 {
			currentWidth += len(t.spacing)
		}

		// Accumulate the column's data width.
		currentWidth += width

		if currentWidth <= availableWidth {
			fittingColumns = append(fittingColumns, i)
		} else {
			break
		}
	}

	return fittingColumns
}

// calculateRowHeight calculates how many terminal lines a single data row
// will occupy when rendered. If no cell in the row wraps, this returns 1.
// Wrapping happens when the cell's content is wider than the column's
// display width (the data is broken across multiple lines).
func (t *Table) calculateRowHeight(row []string, colWidths []int, fittingColumns []int) int {
	maxLines := 1

	// For each fitting column, calculate how many lines it will take up.
	for _, colIndex := range fittingColumns {
		if colIndex < len(row) {
			content := row[colIndex]
			// If the content is wider than the column, calculate how many
			// lines are needed using ceiling division.
			if len(content) > colWidths[colIndex] {
				lines := (len(content) + colWidths[colIndex] - 1) / colWidths[colIndex]
				if lines > maxLines {
					maxLines = lines
				}
			}
		}
	}

	return maxLines
}

// renderPagelet renders a single pagelet — one screen's worth of rows for
// one width-slice of columns — as a single string. It always starts with
// the column headings (and optional underline) so that every pagelet is
// self-contained and readable on its own.
func (t *Table) renderPagelet(pageRows [][]string, widths []int, fittingColumns []int) string {
	var result strings.Builder

	indent := t.indent

	if t.showHeadings && len(t.names) > 0 {
		// Render the column-name header row.
		headerLine := t.renderRow(t.names, widths, fittingColumns)
		result.WriteString(indent + headerLine + "\n")

		// Render the underline row (e.g., "======  ======") if requested.
		if t.showUnderlines {
			underline := t.renderUnderline(widths, fittingColumns)
			result.WriteString(indent + underline + "\n")
		}
	}

	// Render each data row.
	for _, row := range pageRows {
		line := t.renderRow(row, widths, fittingColumns)
		result.WriteString(indent + line + "\n")
	}

	return result.String()
}

// renderRow renders a single row (header or data) by emitting only the
// columns listed in fittingColumns, separated by the table's spacing string.
// widths[colIndex] is the display width of column colIndex; alignment is
// likewise keyed by column index.
func (t *Table) renderRow(row []string, widths, fittingColumns []int) string {
	var result strings.Builder

	for i, colIndex := range fittingColumns {
		// Write the inter-column gap before every column except the first.
		// Use t.spacing so the rendered gap matches the width budget computed
		// by determineFittingColumns (both use len(t.spacing)).
		if i > 0 {
			result.WriteString(t.spacing)
		}

		if colIndex < len(row) {
			// AlignText pads (or truncates) the cell to exactly widths[colIndex]
			// characters, applying left/center/right alignment.
			content := AlignText(row[colIndex], widths[colIndex], t.alignment[colIndex])
			result.WriteString(content)
		}
	}

	return result.String()
}

// renderUnderline renders the underline row that appears beneath the column
// headings. Each column gets a run of "=" characters equal to its display
// width, with the same inter-column spacing used by renderRow.
func (t *Table) renderUnderline(widths, fittingColumns []int) string {
	var result strings.Builder

	for i, colIndex := range fittingColumns {
		// Use t.spacing for the same reason as renderRow: consistency with
		// the width budget computed in determineFittingColumns.
		if i > 0 {
			result.WriteString(t.spacing)
		}

		// Draw a row of "=" characters as wide as the column.
		if colIndex < len(t.names) {
			underline := strings.Repeat("=", widths[colIndex])
			result.WriteString(underline)
		}
	}

	return result.String()
}

// normalizeWidths computes the maximum display width for every column
// (considering both the heading text and all cell values), then pads and
// aligns every cell in t.rows in-place so that renderRow can concatenate
// cells directly without any per-cell width arithmetic.
func (t *Table) normalizeWidths() []int {
	if len(t.rows) == 0 {
		return make([]int, t.columnCount)
	}

	// If we are adding in the row count, prepend a synthetic "Row" column
	// to both the header slice and every data row before measuring widths.
	if t.showRowNumbers {
		t.names = append([]string{"Row"}, t.names...)
		t.alignment = append([]int{AlignmentRight}, t.alignment...)

		for i, row := range t.rows {
			t.rows[i] = append([]string{strconv.Itoa(i + 1)}, row...)
		}
	}

	widths := make([]int, len(t.names))

	// Seed each column's width with the length of its heading so that the
	// heading is never truncated.
	for i, name := range t.names {
		widths[i] = len(name)
	}

	// Widen each column if any data cell is longer than the heading.
	for _, row := range t.rows {
		for j, column := range row {
			if widths[j] < len(column) {
				widths[j] = len(column)
			}
		}
	}

	// Pad and align every cell so all cells in a column are the same width.
	// After this step, len(t.rows[i][j]) == widths[j] for all i, j.
	for i, row := range t.rows {
		for j, column := range row {
			t.rows[i][j] = AlignText(column, widths[j], t.alignment[j])
		}
	}

	return widths
}
