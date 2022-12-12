package runtime

import (
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// FormatSymbols implements the util.symbols() function. We skip over the current
// symbol table, which was created just for this function call and will always be
// empty.
func FormatSymbols(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	selectedScope := -1
	json := false

	if len(args) > 0 {
		json = strings.EqualFold(datatypes.GetString(args[0]), "json")
	}

	if len(args) > 1 {
		selectedScope = datatypes.GetInt(args[1])
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

// Get retrieves a value from the package structure by name. It returns the value and
// a boolean value indicating if it was found. The flag is true if the package has been
// initialized, the hash map is initialized, and the named value is found in the hashmap.
func GetPackageSymbols(p *datatypes.EgoPackage) *symbols.SymbolTable {
	if p == nil {
		return nil
	}

	symV, found := p.Get(datatypes.SymbolsMDKey)
	if found {
		if syms, ok := symV.(*symbols.SymbolTable); ok {
			return syms
		} else {
			ui.Debug(ui.DebugLogger, "Package symbol table was of wrong type: %#v", symV)

			return nil
		}
	} else {
		return symbols.NewSymbolTable("package " + p.Name())
	}
}
