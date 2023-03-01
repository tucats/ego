package tables

import (
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// newTable implements the newTable() table package function. This accepts a list
// of column names (as individual arguments or an array of strings) and allocates
// a new table. Additionally, the column names can contain alignment information;
// a name with a leading ":" is left-aligned, and a trailing":" is right-
// aligned. In either case the ":" is removed from the name.
func newTable(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	// Fetch the arguments as column headings. If the value is passed by array,
	// extract each array member as a column name.
	headings := []string{}

	for _, h := range args.Elements() {
		if list, ok := h.(*data.Array); ok {
			for idx := 0; idx < list.Len(); idx++ {
				str, _ := list.Get(idx)
				headings = append(headings, data.String(str))
			}
		} else if list, ok := h.([]interface{}); ok {
			for _, hh := range list {
				headings = append(headings, data.String(hh))
			}
		} else {
			headings = append(headings, data.String(h))
		}
	}

	// Scan over the heading strings and look for alignment cues. If found,
	// remove the":" cue character, and record the specified (or default)
	// alignment for each column.
	align := make([]int, len(headings))

	for i := 0; i < len(headings); i = i + 1 {
		h := headings[i]
		if strings.HasPrefix(h, ":") && strings.HasSuffix(h, ":") {
			align[i] = tables.AlignmentCenter
			headings[i] = strings.TrimPrefix(strings.TrimSuffix(h, ":"), ":")
		} else if strings.HasPrefix(h, ":") {
			align[i] = tables.AlignmentLeft
			headings[i] = strings.TrimPrefix(h, ":")
		} else if strings.HasSuffix(h, ":") {
			align[i] = tables.AlignmentRight
			headings[i] = strings.TrimSuffix(h, ":")
		} else {
			align[i] = tables.AlignmentLeft
		}
	}

	// Create the new table object, and set the alignment for each column heading now.
	t, err := tables.New(headings)
	if err != nil {
		return nil, err
	}

	for i, v := range align {
		_ = t.SetAlignment(i, v)
	}

	// Turn off pagination by default.
	t.SetPagination(0, 0)

	// Move the string array of headings into a native array type, which can
	// be read by the caller.
	headingsArray := data.NewArray(data.StringType, len(headings))

	for i, h := range headings {
		_ = headingsArray.Set(i, h)
	}

	result := data.NewStruct(tableTypeDef).FromBuiltinPackage()
	result.SetAlways(tableFieldName, t)
	result.SetAlways(headingsFieldName, headingsArray)
	result.SetReadonly(true)

	return result, nil
}

// closeTable closes the table handle, and releases any memory resources
// being held by the table.
func closeTable(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	_, err := getTable(s)
	if err != nil {
		return nil, err
	}

	this := getThisStruct(s)
	this.SetAlways(tableFieldName, nil)

	return true, err
}

// addRow adds a row to the table. This can either be a list of values, or
// a struct. When it's a struct, each column name must match a struct member
// name, and the associated value is used as the table cell value. If a list of
// values is given, they are stored in the row in the same order that the columns
// were defined when the table was created.
func addRow(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTable(s)
	if err == nil {
		if args.Len() > 0 {
			if m, ok := args.Get(0).(*data.Struct); ok {
				if args.Len() > 1 {
					err = errors.ErrArgumentCount
				} else {
					values := make([]string, len(m.FieldNames(false)))

					for _, k := range m.FieldNames(false) {
						v := m.GetAlways(k)
						if v == nil {
							return nil, errors.ErrInvalidField.Context(k)
						}

						p, ok := t.Column(k)
						if ok {
							values[p] = data.String(v)
						}
					}

					err = t.AddRow(values)
				}
			} else {
				if m, ok := args.Get(0).([]interface{}); ok {
					if args.Len() > 1 {
						err = errors.ErrArgumentCount

						return err, err
					}
					err = t.AddRowItems(m...)
				} else {
					err = t.AddRowItems(args.Elements()...)
				}
			}
		}
	}

	return err, err
}

// sortTable sorts the rows of the table. If you specify multiple arguments
// (column names) the sort is performed in the reverse order specified; that
// is the least-significant sort is performed first, then the next-most-
// significant sort, etc. until the first argument, which is the most
// significant sort. The column names can start with a tilde ("~") character
// to reverse the sort order from it's default value of ascending to descending.
func sortTable(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	t, err := getTable(s)
	if err == nil {
		for i := args.Len() - 1; i >= 0; i = i - 1 {
			v := args.Get(i)
			ascending := true

			heading := data.String(v)
			if strings.HasPrefix(heading, "~") {
				ascending = false
				heading = heading[1:]
			}

			pos, found := t.Column(heading)
			if !found {
				err = errors.ErrInvalidColumnName.Context(heading)
			} else {
				err = t.SortRows(pos, ascending)
			}
		}
	}

	return err, err
}

// getTable searches the symbol table for the client receiver (defs.ThisVariable)
// variable, validates that it contains a table object, and returns the
// native table object.
func getTable(symbols *symbols.SymbolTable) (*tables.Table, error) {
	if value, ok := symbols.Get(defs.ThisVariable); ok {
		if structValue, ok := value.(*data.Struct); ok {
			if tableValue := structValue.GetAlways(tableFieldName); tableValue != nil {
				if table, ok := tableValue.(*tables.Table); ok {
					if table == nil {
						return nil, errors.ErrTableClosed
					}

					return table, nil
				}
			}
		}
	}

	return nil, errors.ErrNoFunctionReceiver
}

// getThis returns a map for the "this" object in the current
// symbol table.
func getThisStruct(s *symbols.SymbolTable) *data.Struct {
	t, ok := s.Get(defs.ThisVariable)
	if !ok {
		return nil
	}

	this, ok := t.(*data.Struct)
	if !ok {
		return nil
	}

	return this
}

// Pad the formatted value of a given object to the specified number
// of characters. Negative numbers are right-aligned, positive numbers
// are left-aligned.
func Pad(v interface{}, w int) string {
	s := data.FormatUnquoted(v)
	count := w

	if count < 0 {
		count = -count
	}

	padString := ""
	if count > len(s) {
		padString = strings.Repeat(" ", count-len(s))
	}

	var r string

	if w < 0 {
		r = padString + s
	} else {
		r = s + padString
	}

	if len(r) > count {
		r = r[:count]
	}

	return r
}
