package runtime

import (
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// FormatSymbols implements the util.symbols() function. We skip over the current
// symbol table, which was created just for this function call and will always be
// empty.
func FormatSymbols(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	selectedScope := -1
	json := false

	if len(args) > 0 {
		json = strings.EqualFold(util.GetString(args[0]), "json")
	}

	if len(args) > 1 {
		selectedScope = util.GetInt(args[1])
	}

	scopeLevel := 0
	syms := s.Parent

	scopeColumns := []string{}
	if selectedScope < 0 {
		scopeColumns = []string{"Scope", "Table"}
	}

	t, _ := tables.New(append(scopeColumns, []string{"Symbol", "Type", "Value"}...))

	_ = t.SetAlignment(0, tables.AlignmentCenter)

	for syms != nil {
		if selectedScope >= 0 && selectedScope != scopeLevel {
			syms = syms.Parent

			continue
		}

		name := syms.Name
		scope := strconv.Itoa(scopeLevel)

		rows := syms.FormattedData(false)
		if len(rows) > 0 {
			for _, row := range rows {
				// Escape the value column if needed
				if json {
					row[2] = strings.ReplaceAll(row[2], "\"", "\\\"")
				}

				if selectedScope >= 0 {
					_ = t.AddRow(row)
				} else {
					rowData := append([]string{scope, name}, row...)

					if !json {
						name = ""
						scope = ""
					}

					_ = t.AddRow(rowData)
				}
			}

			if !json {
				_ = t.AddRow([]string{"", "", "", "", ""})
			}
		} else if !json {
			_ = t.AddRow([]string{scope, name, "<no symbols>", "", ""})
			_ = t.AddRow([]string{"", "", "", "", ""})
		}

		scopeLevel++

		syms = syms.Parent
	}

	t.ShowHeadings(true).ShowUnderlines(true)

	if json {
		return t.FormatJSON(), nil
	}

	return strings.Join(t.FormatText(), "\n") + "\n", nil
}
