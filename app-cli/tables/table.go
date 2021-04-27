package tables

import "github.com/tucats/ego/errors"

const (
	// AlignmentLeft aligns the column to the left.
	AlignmentLeft = -1

	// AlignmentRight aligns the column to the right.
	AlignmentRight = 1

	// AlignmentCenter aligns the column to the center.
	AlignmentCenter = 0
)

// Table is the wrapper object around a table to be printed.
type Table struct {
	rows           [][]string
	columns        []string
	alignment      []int
	maxWidth       []int
	columnOrder    []int
	spacing        string
	indent         string
	where          string
	rowLimit       int
	startingRow    int
	columnCount    int
	rowCount       int
	orderBy        int
	ascending      bool
	showUnderlines bool
	showHeadings   bool
	showRowNumbers bool
}

// New creates a new table object, given a list of headings.
func New(headings []string) (*Table, *errors.EgoError) {
	t := &Table{}

	if len(headings) == 0 {
		return t, errors.New(errors.ErrEmptyColumnList)
	}

	t.rowLimit = -1
	t.columnCount = len(headings)
	t.columns = headings
	t.maxWidth = make([]int, t.columnCount)
	t.alignment = make([]int, t.columnCount)
	t.columnOrder = make([]int, t.columnCount)
	t.spacing = "    "
	t.indent = ""
	t.rows = make([][]string, 0)
	t.orderBy = -1
	t.ascending = true
	t.showUnderlines = true
	t.showHeadings = true

	for n, h := range headings {
		t.maxWidth[n] = len(h)
		t.columns[n] = h
		t.alignment[n] = AlignmentLeft
		t.columnOrder[n] = n
	}

	return t, nil
}

// SetWhere sets an expression to be used as a "where" clause
// to select table rows.
func (t *Table) SetWhere(clause string) *Table {
	t.where = clause

	return t
}
