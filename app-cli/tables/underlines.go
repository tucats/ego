package tables

import "strings"

// buildUnderlines generates an underline row for the table headers.
//
// It appends an underline row to the headers slice if showHeadings and showUnderlines are true.
// The underline row is constructed by iterating over the columnOrder slice starting from columnIndex,
// and for each column, it writes an equal sign ('=') for the number of times equal to the maxWidth of that column.
// It also writes the spacing between columns.
//
// If rowNumberWidth is greater than 0, it writes an equal sign for the number of times equal to rowNumberWidth,
// followed by the spacing.
//
// The function returns the updated headers slice.
func buildUnderlines(t *Table, headers []strings.Builder, headerIndex int, rowNumberWidth int, columnIndex int) []strings.Builder {
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
	return headers
}
