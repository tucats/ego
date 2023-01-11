package tables

import (
	"strings"

	"github.com/tucats/ego/errors"
)

// Column returns the column number for a named column. The boolean return
// value indicates if the value was found, if true then the integer result is a
// zero-based column number.
func (t *Table) Column(name string) (int, bool) {
	for n, v := range t.names {
		if strings.EqualFold(v, name) {
			return n, true
		}
	}

	return -1, false
}

// GetHeadings returns an array of the headings already stored
// in the table. This can be used to validate a name against
// the list of headings, for example.
func (t *Table) GetHeadings() []string {
	return t.names
}

// SetColumnOrder accepts a list of column positions and uses it
// to set the order in which columns of output are printed.
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

// SetColumnOrderByName accepts a list of column positions and uses it
// to set the order in which columns of output are printed.
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
