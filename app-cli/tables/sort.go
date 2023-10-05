package tables

import (
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/ego/errors"
)

// SortRows sorts the existing table rows. The column to sort by is specified by
// ordinal position (zero-based). The ascending flag is true if the sort is to be
// in ascending order, and false if a descending sort is required.
func (t *Table) SortRows(column int, ascending bool) error {
	if column < 0 || column >= t.columnCount {
		return errors.ErrInvalidColumnNumber.Context(column)
	}

	sort.SliceStable(t.rows, func(i, j int) bool {
		// If both values are numeric, sort numerically
		if v1, err := strconv.Atoi(t.rows[i][column]); err == nil {
			if v2, err := strconv.Atoi(t.rows[j][column]); err == nil {
				if ascending {
					return v1 < v2
				}

				return v2 < v1
			}
		}

		if ascending {
			return t.rows[i][column] < t.rows[j][column]
		}

		return t.rows[i][column] > t.rows[j][column]
	})

	return nil
}

// SetOrderBy sets the name of the column that should be used for
// sorting the output data.
func (t *Table) SetOrderBy(name string) error {
	ascending := true

	if name[0] == '~' {
		name = name[1:]
		ascending = false
	}

	for n, v := range t.GetHeadings() {
		if strings.EqualFold(name, v) {
			t.orderBy = n
			t.ascending = ascending

			return nil
		}
	}

	return errors.ErrInvalidColumnName.Context(name)
}
