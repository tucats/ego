package runtime

import (
	"errors"
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/gopackages/app-cli/tables"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"
)

// TableNew implements the New() table package function. This accepts a list
// of column names (as individual arguements or an array of strings) and allocates
// a new table. Additionally, the column names can contain alignment information;
// a name with a leading ":" is left-aligned, and a trailing ":" is right-
// aligned. In either case the ":" is removed from the name.
func TableNew(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	if len(args) == 0 {
		return nil, errors.New(defs.IncorrectArgumentCount)
	}

	// Fetch the arguments as column headings. If the value is passed by array,
	// go ahead and extract each array member as a column name.
	headings := []string{}
	for _, h := range args {
		if list, ok := h.([]interface{}); ok {
			for _, hh := range list {
				headings = append(headings, util.GetString(hh))
			}
		} else {
			headings = append(headings, util.GetString(h))
		}
	}

	// Scan over the heading strings and look for alignment cues. If found,
	// remove the ":" cue character, and record the specified (or default)
	// alignmnt for each column.
	align := make([]int, len(headings))
	for i := 0; i < len(headings); i = i + 1 {
		h := headings[i]
		if strings.HasPrefix(h, ":") && strings.HasSuffix(h, ":") {
			align[i] = tables.AlignmentCenter
			headings[i] = strings.TrimPrefix(strings.TrimSuffix(h, ":"), ":")
		} else {
			if strings.HasPrefix(h, ":") {
				align[i] = tables.AlignmentLeft
				headings[i] = strings.TrimPrefix(h, ":")
			} else {
				if strings.HasSuffix(h, ":") {
					align[i] = tables.AlignmentRight
					headings[i] = strings.TrimSuffix(h, ":")
				} else {
					align[i] = tables.AlignmentLeft
				}
			}
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

	headingsArray := make([]interface{}, len(headings))
	for i, h := range headings {
		headingsArray[i] = h
	}

	return map[string]interface{}{
		"table":      &t,
		"AddRow":     TableAddRow,
		"Close":      TableClose,
		"Sort":       TableSort,
		"Print":      TablePrint,
		"Format":     TableFormat,
		"headings":   headingsArray,
		"__readonly": true,
		"__type":     "TableHandle",
	}, nil
}

// TableClose closes the table handle, and releases any memory resources
// being held by the table.
func TableClose(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	_, err := getTable(s)
	if err != nil {
		return nil, err
	}
	this := getThis(s)
	this["table"] = nil
	this["AddRow"] = tableReleased
	this["Sort"] = tableReleased
	this["Print"] = tableReleased
	this["Format"] = tableReleased

	return true, err
}

// TableAddRow adds a row to the table. This can either be a list of values, or
// a struct. When it's a struct, each column name must match a struct member
// name, and the associated value is used as the table cell value. If a list of
// values is given, they are stored in the row in the same order that the columns
// were defined when the table was created.
func TableAddRow(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t, err := getTable(s)
	if err == nil {
		if len(args) > 0 {
			if m, ok := args[0].(map[string]interface{}); ok {
				if len(args) > 1 {
					err = errors.New(defs.IncorrectArgumentCount)
				} else {
					values := make([]string, len(m))
					for k, v := range m {
						p, ok := t.FindColumn(k)
						if ok {
							values[p] = util.GetString(v)
						}
					}
					err = t.AddRow(values)
				}
			} else {
				if m, ok := args[0].([]interface{}); ok {
					if len(args) > 1 {
						err = errors.New(defs.IncorrectArgumentCount)
						return err, err
					}
					err = t.AddRowItems(m...)
				} else {
					err = t.AddRowItems(args...)
				}
			}
		}
	}
	return err, err
}

// TableSort sorts the rows of the table. If you specify multiple arguments
// (column names) the sort is performed in the reverse order specified; that
// is the least-significant sort is performed first, then the next-most-
// significant sort, etc. until the first argument, which is the most
// significant sort. The column names can start wiht a tilde ("~") character
// to reverse the sort order from it's default value of ascending to descending.
func TableSort(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t, err := getTable(s)
	if err == nil {
		for i := len(args) - 1; i >= 0; i = i - 1 {
			v := args[i]
			heading := util.GetString(v)
			ascending := true
			if strings.HasPrefix(heading, "~") {
				ascending = false
				heading = heading[1:]
			}
			pos, found := t.FindColumn(heading)
			if !found {
				err = errors.New("Invalid column name:" + heading)
			} else {
				err = t.SortRows(pos, ascending)
			}
		}
	}
	return err, err
}

// TableFormat specifies the headings format. It accepts two values, which
// are both booleans. The first indicates if a headings row is to be printed
// in the output. The second is examined only if the headings value is true;
// it controls whether an underline string is printed under the column names.
func TableFormat(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 2 {
		err := errors.New(defs.IncorrectArgumentCount)
		return err, err
	}
	t, err := getTable(s)
	if err == nil {
		headings := true
		lines := true
		if len(args) > 0 {
			headings = util.GetBool(args[0])
			lines = headings
		}
		if len(args) > 1 {
			lines = util.GetBool(args[1])
		}
		t.ShowHeadings(headings)
		t.ShowUnderlines(lines)
	}
	return err, err
}

// TablePrint prints a table to the default output, in the default --output-format
// type (text or json)
func TablePrint(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t, err := getTable(s)
	if err == nil {
		err = t.Print(ui.OutputFormat)
	}
	return err, err
}

// getTable searches the symbol table for the client receiver ("_this")
// variable, validates that it contains a table object, and returns the
// native table object
func getTable(symbols *symbols.SymbolTable) (*tables.Table, error) {
	if g, ok := symbols.Get("_this"); ok {
		if gc, ok := g.(map[string]interface{}); ok {
			if tbl, ok := gc["table"]; ok {
				if tp, ok := tbl.(*tables.Table); ok {
					if tp == nil {
						return nil, errors.New("table was closed")
					}
					return tp, nil
				}
			}
		}
	}
	return nil, errors.New(defs.NoFunctionReceiver)
}

// Utility function that becomes the table handle function pointer for a closed
// table handle
func tableReleased(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return nil, errors.New("table closed")
}
