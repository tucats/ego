package tables

import (
	"strings"

	"github.com/tucats/ego/errors"
)

// AddColumn appends a new column at the right of the table. The heading must
// be a non-empty string; otherwise ErrInvalidColumnName is returned.
//
// Every existing row gains an empty string placeholder in the new column
// position so that the row width stays consistent with the column count.
//
// The initial maxWidth for the new column is set to the rune count of its
// heading (matching the behaviour of New()), so the heading is never
// truncated in text output even before any row data is added.
func (t *Table) AddColumn(heading string) error {
	if len(heading) == 0 {
		return errors.ErrInvalidColumnName
	}

	headingWidth := 0
	for range heading {
		headingWidth++
	}

	t.columnCount++
	t.names = append(t.names, heading)
	t.alignment = append(t.alignment, AlignmentLeft)
	t.maxWidth = append(t.maxWidth, headingWidth)
	t.columnOrder = append(t.columnOrder, t.columnCount-1)

	for i := range t.rows {
		t.rows[i] = append(t.rows[i], "")
	}

	return nil
}

// AddColumns appends multiple columns to the table in the order they are
// listed. Passing no arguments is a no-op and returns nil.
//
// Validation is performed in a first pass before any column is added:
//   - Any empty heading causes ErrInvalidColumnName to be returned.
//   - Any heading that already exists in the table (case-insensitive)
//     causes ErrDuplicateColumnName to be returned.
//
// If validation passes, each column is added via AddColumn. Because the
// validation and insertion are separate passes, the table is never left in
// a partially modified state when an error occurs.
func (t *Table) AddColumns(headings ...string) error {
	if len(headings) == 0 {
		return nil
	}

	for _, heading := range headings {
		if len(heading) == 0 {
			return errors.ErrInvalidColumnName
		}

		if _, exists := t.Column(heading); exists {
			return errors.ErrDuplicateColumnName.Context(heading)
		}
	}

	for _, heading := range headings {
		if err := t.AddColumn(heading); err != nil {
			return err
		}
	}

	return nil
}

// Column looks up a column by name and returns its zero-based index. The
// search is case-insensitive so "Name", "name", and "NAME" all resolve to
// the same column. Returns (-1, false) when no column with that name exists.
func (t *Table) Column(name string) (int, bool) {
	for n, v := range t.names {
		if strings.EqualFold(v, name) {
			return n, true
		}
	}

	return -1, false
}

// GetHeadings returns the slice of column names in definition order (not
// display order). The returned slice is the table's internal slice, so
// callers must not modify it. Use it for read-only operations such as
// validating that a name is a known column.
func (t *Table) GetHeadings() []string {
	return t.names
}

// SetColumnOrder sets the display order of columns for text and JSON output.
// The order slice contains 1-based column positions (not zero-based), so
// column 1 is the first column defined in New(). ErrEmptyColumnList is
// returned when order is empty; ErrInvalidColumnNumber is returned when any
// value is less than 1 or greater than the column count.
//
// Example — print the third column first, then the first, then the second:
//
//	err := t.SetColumnOrder([]int{3, 1, 2})
//
// The order slice may be shorter than the total number of columns; only the
// listed columns are printed.
func (t *Table) SetColumnOrder(order []int) error {
	if len(order) == 0 {
		return errors.ErrEmptyColumnList
	}

	newOrder := make([]int, len(order))

	for n, v := range order {
		if v < 1 || v > t.columnCount {
			return errors.ErrInvalidColumnNumber.Context(v)
		}

		newOrder[n] = v - 1
	}

	t.columnOrder = newOrder

	return nil
}

// SetColumnOrderByName sets the display order of columns using column names
// rather than numeric positions. Names are matched case-insensitively.
// ErrEmptyColumnList is returned when order is empty; ErrInvalidColumnName
// is returned when any name is not found in the table.
//
// Example — print "city" before "name":
//
//	err := t.SetColumnOrderByName([]string{"city", "name", "age"})
func (t *Table) SetColumnOrderByName(order []string) error {
	if len(order) == 0 {
		return errors.ErrEmptyColumnList
	}

	newOrder := make([]int, len(order))

	for n, name := range order {
		v, found := t.Column(name)
		if !found {
			return errors.ErrInvalidColumnName.Context(name)
		}

		newOrder[n] = v
	}

	t.columnOrder = newOrder

	return nil
}
