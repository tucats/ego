package tables

import (
	"strings"

	"github.com/tucats/ego/errors"
)

// RowLimit caps the number of data rows included in any output method.
// A value of 0 or any negative number disables the limit (all rows are
// included). Returns the receiver for method chaining.
func (t *Table) RowLimit(limit int) *Table {
	if limit <= 0 {
		t.rowLimit = -1
	} else {
		t.rowLimit = limit
	}

	return t
}

// ShowUnderlines controls whether a row of "===" characters is printed
// immediately below the column headings. Enabled by default. Returns the
// receiver for method chaining.
func (t *Table) ShowUnderlines(flag bool) *Table {
	t.showUnderlines = flag

	return t
}

// ShowHeadings controls whether the column name row (and underline row) is
// printed. When false, data rows are printed without any header. Enabled
// by default. Returns the receiver for method chaining.
func (t *Table) ShowHeadings(flag bool) *Table {
	t.showHeadings = flag

	return t
}

// ShowRowNumbers controls whether a leading row-number column is prepended to
// each data row in text output. Row numbers are 1-based. Disabled by default.
// Returns the receiver for method chaining.
func (t *Table) ShowRowNumbers(flag bool) *Table {
	t.showRowNumbers = flag

	return t
}

// SetMinimumWidth ensures that column n is at least w characters wide in text
// output. Column n is zero-based. If the column's current maxWidth is already
// at least w, this is a no-op. Negative column indices and negative widths
// return an error; zero width is allowed (no-op since maxWidth is always >= 0).
func (t *Table) SetMinimumWidth(n int, w int) error {
	if n < 0 || n >= t.columnCount {
		return errors.ErrInvalidColumnNumber.Context(n)
	}

	if w < 0 {
		return errors.ErrInvalidColumnWidth.Context(w)
	}

	if w > t.maxWidth[n] {
		t.maxWidth[n] = w
	}

	return nil
}

// SetStartingRow specifies the first row to include in output, using a
// 1-based row number. Calling SetStartingRow(1) means "start at the first
// row" (no rows are skipped). Values less than 1 return ErrInvalidRowNumber.
// Internally the value is stored as a zero-based index (s-1).
func (t *Table) SetStartingRow(s int) error {
	if s < 1 {
		return errors.ErrInvalidRowNumber.Context(s)
	}

	t.startingRow = s - 1

	return nil
}

// SetSpacing sets the number of space characters inserted between each pair
// of adjacent columns in text output. The default is 4. Zero is valid and
// produces no inter-column gap. Negative values return ErrInvalidSpacing.
func (t *Table) SetSpacing(s int) error {
	if s < 0 {
		return errors.ErrInvalidSpacing.Context(s)
	}

	var buffer strings.Builder

	for i := 0; i < s; i++ {
		buffer.WriteRune(' ')
	}

	t.spacing = buffer.String()

	return nil
}

// SetIndent sets the number of space characters prepended to every output
// line (headings, underlines, and data rows) in text output. This is useful
// when embedding a table inside a larger structured output. Zero is valid
// (no indent). Negative values return ErrInvalidSpacing.
func (t *Table) SetIndent(s int) error {
	var buffer strings.Builder

	if s < 0 {
		return errors.ErrInvalidSpacing.Context(s)
	}

	for i := 0; i < s; i++ {
		buffer.WriteRune(' ')
	}

	t.indent = buffer.String()

	return nil
}

// SetAlignment sets the text alignment for column (zero-based). The valid
// alignment constants are:
//
//	AlignmentLeft   (-1) — pad on the right (default for all columns)
//	AlignmentCenter  (0) — equal padding on both sides
//	AlignmentRight   (1) — pad on the left
//
// Returns ErrInvalidColumnNumber for an out-of-range column and
// ErrAlignment for any other alignment value.
func (t *Table) SetAlignment(column int, alignment int) error {
	if column < 0 || column >= t.columnCount {
		return errors.ErrInvalidColumnNumber.Context(column)
	}

	switch alignment {
	case AlignmentLeft:
		t.alignment[column] = AlignmentLeft

	case AlignmentRight:
		t.alignment[column] = AlignmentRight

	case AlignmentCenter:
		t.alignment[column] = AlignmentCenter

	default:
		return errors.ErrAlignment.Context(alignment)
	}

	return nil
}
