package tables

import (
	"fmt"
	"strconv"

	"github.com/tucats/ego/errors"
)

// AddRow adds a row to an existing table using an array of string objects,
// where each object represents a column of the data.
func (t *Table) AddRow(row []string) *errors.EgoError {
	if len(row) != t.columnCount {
		return errors.New(errors.IncorrectColumnCountError).WithContext(len(row))
	}

	for n, h := range row {
		if len(h) > t.maxWidth[n] {
			t.maxWidth[n] = len(h)
		}
	}

	t.rows = append(t.rows, row)

	return nil
}

// AddRowItems adds a row to an existing table using individual parameters.
// Each parameter is converted to a string representation, and the set of all
// formatted values are added to the table as a row.
func (t *Table) AddRowItems(items ...interface{}) *errors.EgoError {
	if len(items) != t.columnCount {
		return errors.New(errors.IncorrectColumnCountError).WithContext(len(items))
	}

	row := make([]string, t.columnCount)
	buffer := ""

	for n, item := range items {
		switch v := item.(type) {
		case int:
			buffer = strconv.Itoa(v)

		case string:
			buffer = v

		case bool:
			if v {
				buffer = "true"
			} else {
				buffer = "false"
			}

		default:
			buffer = fmt.Sprintf("%v", item)
		}

		row[n] = buffer
	}

	return t.AddRow(row)
}
