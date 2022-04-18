package dbtables

import (
	"encoding/json"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/util"
)

const (
	symbolPrefix = "{{"
	symbolSuffix = "}}"
)

// For a given task, apply the symbols to the various fields and data values
// in the task.
func applySymbolsToTask(sessionID int32, task *TxOperation, syms *symbolTable) {
	// Process any substittions to filters, column names, or data values
	if syms != nil && len(syms.Symbols) > 0 {
		// Allow substitutions in the table name
		task.Table = applySymbolsToString(sessionID, task.Table, syms, "Table name")

		// Allow subsitutions in the filter list
		for n := 0; n < len(task.Filters); n++ {
			task.Filters[n] = applySymbolsToString(sessionID, task.Filters[n], syms, "Filter")
		}

		// Allow substitutions in the column list
		for n := 0; n < len(task.Columns); n++ {
			task.Columns[n] = applySymbolsToString(sessionID, task.Columns[n], syms, "Column selector")
		}

		// Make a list of the keys we will scan in the data object
		keys := make([]string, 0)
		for key := range task.Data {
			keys = append(keys, key)
		}

		// Allow subtitutions of the key names as well as the values
		for _, key := range keys {
			oldKey := key
			newKey := applySymbolsToString(sessionID, key, syms, "Column name")
			value := task.Data[key]

			if oldKey != newKey {
				delete(task.Data, oldKey)
			}

			task.Data[newKey] = applySymbolsToItem(sessionID, value, syms, "Column value")
		}
	}

	if ui.LoggerIsActive(ui.RestLogger) {
		b, _ := json.MarshalIndent(task, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.Debug(ui.RestLogger, "[%d] Transaction task payload:\n%s", sessionID, util.SessionLog(sessionID, string(b)))
	}
}

// If the item passed is a string of the form {{name}} then the symbol with
// the matching name is substituted for this value, if found.
func applySymbolsToItem(sessionID int32, input interface{}, symbols *symbolTable, label string) interface{} {
	if symbols == nil || symbols.Symbols == nil {
		return input
	}

	stringRepresentation := datatypes.GetString(input)
	if strings.HasPrefix(stringRepresentation, symbolPrefix) && strings.HasSuffix(stringRepresentation, symbolSuffix) {
		key := strings.TrimPrefix(strings.TrimSuffix(stringRepresentation, symbolSuffix), symbolPrefix)

		if value, ok := symbols.Symbols[key]; ok {
			input = value
			ui.Debug(ui.TableLogger, "[%d] %s symbol substitution, %s = %v", sessionID, label, key, value)
		} else {
			input = "No symbol: " + key
		}
	}

	return input
}

// applySymbolsToString searches a string for any symbol references, and replaces
// the refernence with the symbol's value, expressed as a string. So an input string
// of "GE(age, {{target}})" and a symbol value for "target" of 25, will result in an
// output string of "GE(age, 25)". Note that the symbol value is replaced exactly,
// so if the symbol is a string, the input string may still need to include quotes
// around the target to ensure that it is still represented as a string value in
// a filter expresion, for example.
func applySymbolsToString(sessionID int32, input string, syms *symbolTable, label string) string {
	if syms == nil || len(syms.Symbols) == 0 {
		return input
	}

	for k, v := range syms.Symbols {
		search := symbolPrefix + k + symbolSuffix
		replace := datatypes.GetString(v)
		oldInput := input
		input = strings.ReplaceAll(input, search, replace)

		if oldInput != input {
			ui.Debug(ui.TableLogger, "[%d] %s symbol substitution, %s = %v", sessionID, label, k, replace)
		}
	}

	return input
}
