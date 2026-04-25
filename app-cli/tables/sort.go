package tables

import (
	"sort"
	"strings"

	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
)

// SortRows sorts all data rows by the values in column (zero-based). Returns
// ErrInvalidColumnNumber when column is out of range.
//
// Sort behaviour:
//   - If both compared cells parse as integers (via egostrings.Atoi), the
//     comparison is numeric, so "9" < "10" rather than "10" < "9".
//   - Otherwise the comparison is lexicographic string order.
//   - Mixed columns (some cells numeric, some not) fall back to string
//     comparison for rows where at least one cell is non-numeric.
//
// sort.SliceStable is used, so rows with equal sort keys retain their
// original relative order.
func (t *Table) SortRows(column int, ascending bool) error {
	if column < 0 || column >= t.columnCount {
		return errors.ErrInvalidColumnNumber.Context(column)
	}

	sort.SliceStable(t.rows, func(i, j int) bool {
		// If both values are numeric, sort numerically
		if v1, err := egostrings.Atoi(t.rows[i][column]); err == nil {
			if v2, err := egostrings.Atoi(t.rows[j][column]); err == nil {
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

// SetOrderBy stores the name of the column to sort by when Print or String is
// called. The sort is applied lazily — rows are not reordered immediately.
//
// Prefix the name with "~" to request descending order:
//
//	t.SetOrderBy("age")   // ascending sort by "age"
//	t.SetOrderBy("~age")  // descending sort by "age"
//
// Name matching is case-insensitive. Returns ErrInvalidColumnName when name
// is empty or does not match any column.
func (t *Table) SetOrderBy(name string) error {
	if len(name) == 0 {
		return errors.ErrInvalidColumnName
	}

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
