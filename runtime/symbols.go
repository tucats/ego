package runtime

import (
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/compiler"
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
		selectedScope = datatypes.GetInt(args[0])
	}

	if len(args) > 1 {
		json = strings.EqualFold(datatypes.GetString(args[1]), "json")
	}

	// We start counting scope one level above the scope created just for
	// the function call (which will always be empty).
	scopeLevel := 0
	syms := s.Parent

	// Prepare the column names. If a specific scope was NOT requested, we add
	// columns for the scope and table names in the output.
	scopeColumns := []string{}
	if selectedScope < 0 {
		scopeColumns = []string{"Scope", "Table"}
	}

	t, _ := tables.New(append(scopeColumns, []string{"Symbol", "Type", "Readonly", "Value"}...))

	_ = t.SetAlignment(0, tables.AlignmentCenter)

	for syms != nil {
		// If a specific scope was requested and we never found it,
		// time to bail out. Otherwise, keep crawling up the tree.
		if selectedScope >= 0 && selectedScope != scopeLevel {
			if syms.IsRoot() && selectedScope > scopeLevel {
				return nil, errors.New(errors.ErrInvalidScopeLevel).Context(selectedScope)
			}

			syms = syms.Parent
			scopeLevel++

			continue
		}

		name := syms.Name
		scope := strconv.Itoa(scopeLevel)

		// Get the sets of rows for this table. If the table is empty,
		// we don't print it out.
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

		// If we were only doing a specific scope, bail out now.
		if selectedScope >= 0 {
			break
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

func SymbolTables(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	// This function doesn't take any parameters.
	if len(args) > 0 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	// Compile the type definition for the structure we're going to return.
	t, e := compiler.CompileTypeSpec(`
	type SymbolTable struct{
		depth int
		name string
		id string
		root bool
		size int
		}`)

	if !errors.Nil(e) {
		return nil, e
	}

	result := datatypes.NewArray(t, 0)
	depth := 0
	p := s.Parent

	for p != nil {
		item := datatypes.NewStructFromMap(map[string]interface{}{
			"depth": depth,
			"name":  p.Name,
			"id":    p.ID.String(),
			"root":  p.IsRoot(),
			"size":  len(p.Symbols),
		})

		result.Append(item)

		depth++

		p = p.Parent
	}

	return result, nil
}
