package tables

import (
	"fmt"

	"github.com/tucats/ego/errors"
)

// AddRow appends a row to the table. The row slice must have exactly as many
// elements as there are columns; if the lengths differ, ErrColumnCount is
// returned and the table is unchanged.
//
// After a successful add, AddRow updates the per-column maxWidth so that
// FormatText can produce correctly padded output. The width is measured in
// Unicode rune count, not bytes, so multibyte characters (e.g. "Ä", "中")
// contribute one unit of width each, matching terminal display width for
// BMP characters.
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

// AddRowItems appends a row by converting each argument to a string with
// fmt.Sprintf("%v", item). The number of arguments must equal the column
// count; ErrColumnCount is returned otherwise.
//
// Type conversions applied by fmt.Sprintf:
//   - bool      → "true" or "false"
//   - int/float → decimal notation
//   - string    → the string itself (no quotes)
//   - nil       → "<nil>"
//
// After conversion the row is passed to AddRow, which updates maxWidth.
func (t *Table) AddRowItems(items ...any) error {
	if len(items) != t.columnCount {
		return errors.ErrColumnCount.Context(len(items))
	}

	row := make([]string, t.columnCount)

	for n, item := range items {
		row[n] = fmt.Sprintf("%v", item)
	}

	return t.AddRow(row)
}

// GetRow returns the data row at the given zero-based index. Returns
// ErrInvalidRange when index is negative or >= the number of rows.
// The returned slice is the table's internal storage; callers must not
// modify it.
func (t *Table) GetRow(index int) ([]string, error) {
	if index < 0 || index >= len(t.rows) {
		return nil, errors.ErrInvalidRange.Context(index)
	}

	return t.rows[index], nil
}
