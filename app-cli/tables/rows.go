package tables

import (
	"fmt"

	"github.com/tucats/ego/errors"
)

// AddRow adds a row to an existing table using an array of string objects,
// where each object represents a column of the data.
func (t *Table) AddRow(row []string) error {
	if len(row) != t.columnCount {
		return errors.ErrColumnCount.Context(len(row))
	}

	// Update the maximum row width based on this new row info. Count
	// the runes explicitly since len() of a string is really the byte
	// count, not the character count.
	for n, h := range row {
		realLength := 0

		for range h {
			realLength++
		}

		if realLength > t.maxWidth[n] {
			t.maxWidth[n] = realLength
		}
	}

	t.rows = append(t.rows, row)

	return nil
}

// AddRowItems adds a row to an existing table using individual parameters.
// Each parameter is converted to a string representation, and the set of all
// formatted values are added to the table as a row.
func (t *Table) AddRowItems(items ...interface{}) error {
	if len(items) != t.columnCount {
		return errors.ErrColumnCount.Context(len(items))
	}

	row := make([]string, t.columnCount)

	for n, item := range items {
		row[n] = fmt.Sprintf("%v", item)
	}

	return t.AddRow(row)
}
