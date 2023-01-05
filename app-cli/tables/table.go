package tables

import (
	"github.com/tucats/ego/errors"
	"golang.org/x/term"
)

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
	names          []string
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
	terminalWidth  int
	terminalHeight int
	ascending      bool
	showUnderlines bool
	showHeadings   bool
	showRowNumbers bool
}

// New creates a new table object, given a list of headings.
func New(headings []string) (*Table, error) {
	t := &Table{}

	if len(headings) == 0 {
		return t, errors.EgoError(errors.ErrEmptyColumnList)
	}

	t.rowLimit = -1
	t.columnCount = len(headings)
	t.names = headings
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
		realLen := 0

		for range h {
			realLen++
		}

		t.maxWidth[n] = realLen
		t.names[n] = h
		t.alignment[n] = AlignmentLeft
		t.columnOrder[n] = n
	}

	// For pagination, if there is a terminal with width and height,
	// add that to the table definition. Zero values mean no pagination
	// or column folding will be done.
	if term.IsTerminal(0) {
		width, height, err := term.GetSize(0)
		if err != nil {
			return nil, errors.EgoError(err)
		}

		t.terminalWidth = width
		t.terminalHeight = height
	}

	return t, nil
}

// SetWhere sets an expression to be used as a "where" clause
// to select table rows.
func (t *Table) SetWhere(clause string) *Table {
	t.where = clause

	return t
}
