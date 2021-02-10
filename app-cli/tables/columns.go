package tables

import (
	"strings"

	"github.com/tucats/ego/errors"
)

// FindColumn returns the column number for a named column. The boolean return
// value indicates if the value was found, if true then the integer result is a
// zero-based column number.
func (t *Table) FindColumn(name string) (int, bool) {
	for n, v := range t.columns {
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
	return t.columns
}

// SetColumnOrder accepts a list of column positions and uses it
// to set the order in which columns of output are printed.
func (t *Table) SetColumnOrder(order []int) *errors.EgoError {
	if len(order) == 0 {
		return NewTableErr(EmptyColumnListError)
	}

	newOrder := make([]int, len(order))

	for n, v := range order {
		if v < 1 || v > t.columnCount {
			return NewTableErr(InvalidColumnNumberError, v)
		}

		newOrder[n] = v - 1
	}

	t.columnOrder = newOrder

	return nil
}

// SetColumnOrderByName accepts a list of column positions and uses it
// to set the order in which columns of output are printed.
func (t *Table) SetColumnOrderByName(order []string) *errors.EgoError {
	if len(order) == 0 {
		return NewTableErr(EmptyColumnListError)
	}

	newOrder := make([]int, len(order))

	for n, name := range order {
		v, found := t.FindColumn(name)
		if !found {
			return NewTableErr(InvalidColumnNameError, name)
		}

		newOrder[n] = v
	}

	t.columnOrder = newOrder

	return nil
}
