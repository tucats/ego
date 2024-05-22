package tables

import (
	"strings"

	"github.com/tucats/ego/i18n"
)

// buildHeaderMap generates headers for the table based on the column order, maximum width,
// alignment, and other table settings. It also handles column folding and pagination.
//
// Parameters:
// - t: A pointer to the Table struct containing the table data and settings.
// - headers: A slice of strings.Builder that holds the headers for each pagelet.
// - availableWidth: An integer representing the available width for the headers.
// - columnIndex: An integer representing the current index in the column order.
// - rowNumberWidth: An integer representing the width of the row number column.
// - columnMap: A slice of integers that maps column indexes to header indexes.
// - first: A boolean indicating whether this is the first iteration of the loop.
//
// Returns:
// - headers: A slice of strings.Builder containing the updated headers.
// - headerIndex: An integer representing the current index in the headers slice.
// - headerCount: An integer representing the total number of headers generated.
// - columnIndex: An integer representing the updated index in the column order.
func buildHeaderMap(t *Table, headers []strings.Builder, availableWidth int, columnIndex int, rowNumberWidth int, columnMap []int, first bool) ([]strings.Builder, int, int, int) {
	var (
		headerIndex int
		headerCount int
	)

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

	return headers, headerIndex, headerCount, columnIndex
}
