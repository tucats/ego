package tables

import (
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// setPagination sets the page width and height for paginated output. Set the
// values both to zero to disable pagination support.
func setPagination(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	h := data.Int(args[0])
	w := data.Int(args[1])

	t, err := getTable(s)
	if err != nil {
		return nil, err
	}

	t.SetPagination(h, w)

	return true, err
}

// setFormat specifies the headings format. It accepts two values, which
// are both booleans. The first indicates if a headings row is to be printed
// in the output. The second is examined only if the headings value is true;
// it controls whether an underline string is printed under the column names.
func setFormat(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t, err := getTable(s)
	if err == nil {
		headings := true
		lines := true

		if len(args) > 0 {
			headings = data.Bool(args[0])
			lines = headings
		}

		if len(args) > 1 {
			lines = data.Bool(args[1])
		}

		t.ShowHeadings(headings)
		t.ShowUnderlines(lines)
	}

	return err, err
}

// setAlignment specifies alignment for a given column.
func setAlignment(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	t, err := getTable(s)
	if err == nil {
		column := 0

		if columnName, ok := args[0].(string); ok {
			column, ok = t.Column(columnName)
			if !ok {
				err = errors.ErrInvalidColumnName.Context(columnName)

				return err, err
			}
		} else {
			column = data.Int(args[0])
		}

		mode := tables.AlignmentLeft

		if modeName, ok := args[1].(string); ok {
			switch strings.ToLower(modeName) {
			case "left":
				mode = tables.AlignmentLeft

			case "right":
				mode = tables.AlignmentRight

			case "center":
				mode = tables.AlignmentCenter

			default:
				err = errors.ErrAlignment.Context(modeName)

				return err, err
			}
		}

		err = t.SetAlignment(column, mode)
	}

	return err, err
}

// printTable prints a table to the default output, in the default --output-format
// type (text or json).
func printTable(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	fmt := ui.OutputFormat

	if len(args) > 0 {
		fmt = data.String(args[0])
	}

	t, err := getTable(s)
	if err == nil {
		err = t.Print(fmt)
	}

	return err, err
}

// toString formats a table as a string in the default output.
func toString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	fmt := ui.OutputFormat

	if len(args) > 0 {
		fmt = data.String(args[0])
	}

	t, err := getTable(s)
	if err == nil {
		return t.String(fmt)
	}

	return nil, err
}