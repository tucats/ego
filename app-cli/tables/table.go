// Package tables provides text table formatting for the Ego CLI. A table
// is a rectangular grid of string cells organized into named columns and
// anonymous rows.
//
// Typical usage:
//
//	t, err := tables.New([]string{"Name", "Age", "City"})
//	_ = t.AddRow([]string{"Alice", "30", "Boston"})
//	_ = t.AddRow([]string{"Bob",   "25", "Denver"})
//	_ = t.SortRows(1, true)             // sort by column 1 ascending
//	_ = t.Print(ui.TextFormat)          // print to stdout
//
// The table can also produce JSON output suitable for REST API responses:
//
//	json := t.FormatJSON()              // compact JSON array of objects
//	indented := t.FormatIndented()      // pretty-printed JSON
//
// Column values are always stored as strings internally. FormatJSON infers
// types at output time: bare integers and the literals "true"/"false" are
// emitted without quotes; everything else becomes a JSON string.
//
// Three output format constants (defined in the ui package) are accepted by
// Print and String:
//
//	ui.TextFormat         — human-readable fixed-width text (default)
//	ui.JSONFormat         — compact JSON array
//	ui.JSONIndentedFormat — indented JSON array
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

// New creates a new Table with the given column headings. An empty slice is
// allowed and produces a zero-column table.
//
// Default settings:
//
//	Alignment:     AlignmentLeft for every column
//	Spacing:       four spaces between columns
//	Indent:        none (empty string)
//	Row limit:     -1 (unlimited)
//	Sort column:   none (-1, unsorted)
//	Show headings:    true
//	Show underlines:  true
//	Show row numbers: false
//
// The initial maxWidth for each column is set to the rune count of its
// heading so that the heading itself is never truncated in text output.
//
// If the process is running in an interactive terminal, New reads the
// terminal width and height so that FormatText can fold columns across
// multiple header blocks (pagination). An error is returned only when
// terminal-size detection fails, which never occurs in non-terminal
// environments.
func New(headings []string) (*Table, error) {
	t := &Table{}

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
			return nil, errors.New(err)
		}

		t.terminalWidth = width
		t.terminalHeight = height
	}

	return t, nil
}

// SetWhere stores an arbitrary filter expression for the table. The expression
// is not evaluated by this package; it is provided as metadata for callers
// that build queries from a Table (for example, a REST handler that maps
// the expression into a SQL WHERE clause). Passing an empty string clears
// any previously set clause. Returns the receiver for method chaining.
func (t *Table) SetWhere(clause string) *Table {
	t.where = clause

	return t
}

// Len returns the number of data rows currently stored in the table.
// It does not count the heading row or the underline row.
func (t *Table) Len() int {
	return len(t.rows)
}

// Width returns the number of columns defined for the table. This equals
// the number of headings passed to New plus any columns added via AddColumn
// or AddColumns.
func (t *Table) Width() int {
	return len(t.names)
}
