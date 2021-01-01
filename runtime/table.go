package runtime

import (
	"errors"
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/gopackages/app-cli/tables"
	"github.com/tucats/gopackages/app-cli/ui"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"

	_ "github.com/lib/pq"
)

// TableNew implements the New() table package function.
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
	t, err := tables.New(headings)
	if err != nil {
		return nil, err
	}
	for i, v := range align {
		_ = t.SetAlignment(i, v)
	}

	return map[string]interface{}{
		"table":      &t,
		"AddRow":     TableAddRow,
		"Close":      TableClose,
		"Sort":       TableSort,
		"Print":      TablePrint,
		"Format":     TableFormat,
		"headings":   headings,
		"__readonly": true,
		"__type":     "TableHandle",
	}, nil
}

// TableClose closes the table handle
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

// TableAddRow adds a row to the table.
func TableAddRow(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t, err := getTable(s)
	if err == nil {
		if len(args) > 0 {
			if m, ok := args[0].(map[string]interface{}); ok {
				values := make([]string, len(m))
				for k, v := range m {
					p, ok := t.FindColumn(k)
					if ok {
						values[p] = util.GetString(v)
					}
				}
				err = t.AddRow(values)
			} else {
				err = t.AddRowItems(args...)
			}
		}
	}
	return err, err
}

// TableSort sorts the rows of the table.
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

// TablePrint prints a table
func TableFormat(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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

// TablePrint prints a table
func TablePrint(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t, err := getTable(s)
	if err == nil {
		err = t.Print(ui.OutputFormat)
	}
	return err, err
}

// getTable searches the symbol table for the client receiver ("_this")
// variable, validates that it contains a table  object, and returns
// the native table object
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

// Utility function that becomes the db handle function pointer for a closed
// db connection handle
func tableReleased(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return nil, errors.New("table closed")
}
