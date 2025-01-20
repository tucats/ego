package tables

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
)

// paginateText will output a table with column folding and pagination.
func (t *Table) paginateText() []string {
	var (
		headers     []strings.Builder
		headerCount int
		headerIndex int
		columnIndex int
		rowCount    = len(t.rows)
	)

	if t.startingRow > 0 {
		rowCount = rowCount - t.startingRow
	}

	if t.rowLimit > 0 {
		rowCount = rowCount - (len(t.rows) - t.rowLimit)
	}

	columnMap := make([]int, len(t.columnOrder))
	headers = make([]strings.Builder, 1)

	// Turn off vertical pagination for now.
	savedTerminalHeight := t.terminalHeight
	defer func() {
		t.terminalHeight = savedTerminalHeight
	}()

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
	headers, headerIndex, headerCount, columnIndex = buildHeaderMap(t, headers, availableWidth, columnIndex, rowNumberWidth, columnMap, first)

	// If we're outputting underlines, then we have to set up the final row of
	// underlines from the last set of headers.
	headers = buildUnderlines(t, headers, headerIndex, rowNumberWidth, columnIndex)

	// Scan over the pagelets and form each page of data.
	rowLimit := t.rowLimit
	headerCount++

	pageletSize, pageletCount, pagelets := makePagelets(rowCount, headerCount, rowNumberWidth, t)

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

	ui.Log(ui.AppLogger, "app.pager.pagelets",
		"count", pageletCount)
	ui.Log(ui.AppLogger, "app.pager.lines",
		"count", pageletSize)
	ui.Log(ui.AppLogger, "app.pager.blocksize",
		"count", printBlockSize)

	// reassemble into a page buffer.
	return pageletToText(pagelets, t, output, headers)
}

func pageletToText(pagelets [][]string, t *Table, output []string, headers []strings.Builder) []string {
	for px, p := range pagelets {
		hx := px
		if t.showUnderlines {
			hx = hx * 2
		}

		// Get the header (and optionally, underline) for this pagelet
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

func makePagelets(rowCount int, headerCount int, rowNumberWidth int, t *Table) (int, int, [][]string) {
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

	return pageletSize, pageletCount, pagelets
}
