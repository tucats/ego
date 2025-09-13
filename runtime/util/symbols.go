package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/symbols"
)

// formatSymbols implements the util.symbols() function. We skip over the current
// symbol table, which was created just for this function call and will always be
// empty.
func formatSymbols(s *symbols.SymbolTable, args data.List) (any, error) {
	var err error

	selectedScope := -1
	json := false
	allItems := false

	if args.Len() > 0 {
		selectedScope, err = data.Int(args.Get(0))
		if err != nil {
			return nil, errors.New(err).In("symbols")
		}
	}

	if args.Len() > 1 {
		json = strings.EqualFold(data.String(args.Get(1)), "json")
	}

	if args.Len() > 2 {
		allItems, err = data.Bool(args.Get(2))
		if err != nil {
			return nil, errors.New(err).In("Symbols")
		}
	}

	// We start counting scope one level above the scope created just for
	// the function call (which will always be empty).
	scopeLevel := 0
	syms := s

	// Prepare the column names. If a specific scope was NOT requested, we add
	// columns for the scope and table names in the output.
	scopeColumns := []string{}
	if selectedScope < 0 {
		scopeColumns = []string{i18n.L("Scope"), i18n.L("Table"), i18n.L("Boundary")}
	}

	t, _ := tables.New(append(scopeColumns, []string{"Symbol", i18n.L("Type"), i18n.L("Readonly"), i18n.L("Value")}...))

	if index, found := t.Column("Scope"); found {
		_ = t.SetAlignment(index, tables.AlignmentCenter)
	}

	tableName := ""

	for syms != nil {
		if selectedScope >= 0 {
			tableName = syms.Name
		}

		// If a specific scope was requested and we never found it,
		// time to bail out. Otherwise, keep crawling up the tree.
		if selectedScope >= 0 && selectedScope != scopeLevel {
			if syms.IsRoot() && selectedScope > scopeLevel {
				return nil, errors.ErrInvalidScopeLevel.Context(selectedScope)
			}

			syms = syms.Parent()
			scopeLevel++

			continue
		}

		name := " " + syms.Name
		if syms.IsShared() {
			name = "*" + syms.Name
		}

		boundary := strconv.FormatBool(syms.IsBoundary())

		scope := strconv.Itoa(scopeLevel)

		// Get the sets of rows for this table. If the table is empty,
		// we don't print it out.
		rows := syms.FormattedData(allItems)
		if len(rows) > 0 {
			for _, row := range rows {
				// Escape the value column if needed
				if json {
					row[2] = strings.ReplaceAll(row[2], "\"", "\\\"")
				}

				if selectedScope >= 0 {
					_ = t.AddRow(row)
				} else {
					rowData := append([]string{scope, name, boundary}, row...)

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
			_ = t.AddRow([]string{scope, name, boundary, "<no symbols>", "", "", ""})
			_ = t.AddRow([]string{"", "", "", "", "", "", ""})
		}

		// If we were only doing a specific scope, bail out now.
		if selectedScope >= 0 {
			break
		}

		scopeLevel++

		syms = syms.Parent()
	}

	t.ShowHeadings(true).ShowUnderlines(true)
	t.SetPagination(0, 0)

	if json {
		return t.FormatJSON(), nil
	}

	if selectedScope >= 0 {
		fmt.Printf("\nSymbol table: %s\n\n", strconv.Quote(tableName))
	}

	return strings.Join(t.FormatText(), "\n") + "\n", nil
}

func formatTables(s *symbols.SymbolTable, args data.List) (any, error) {
	t := UtilSymbolTableType

	result := data.NewArray(t, 0)
	depth := 0
	p := s.Parent()

	for p != nil {
		item := data.NewStructOfTypeFromMap(t, map[string]any{
			"depth":        depth,
			"name":         p.Name,
			"id":           p.ID().String(),
			"root":         p.IsRoot(),
			"size":         p.Size(),
			"shared":       p.IsShared(),
			data.TypeMDKey: t,
		})

		result.Append(item)

		depth++

		p = p.Parent()
	}

	return result, nil
}
