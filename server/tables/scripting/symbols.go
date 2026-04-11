package scripting

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

const (
	// symbolPrefix and symbolSuffix delimit a symbol reference inside a string
	// field. For example, the string "Hello, {{name}}!" contains a reference to
	// the symbol "name". Both delimiters must be present for a substitution to occur.
	symbolPrefix = "{{"
	symbolSuffix = "}}"
)

// applySymbolsToTask expands all {{name}} references in every string field of
// the given task before the operation is executed. It mutates the task in place.
//
// Fields processed: Table, Filters, Columns, SQL, Data keys, Data values,
// and all error-condition strings.
//
// If a reference names a symbol that does not exist in syms, an error is returned
// and the operation is aborted (the caller will roll back the transaction).
func applySymbolsToTask(sessionID int, task *defs.TXOperation, id int, syms *symbolTable) error {
	var err error

	// Log the raw task payload before substitution when REST logging is active.
	// This makes it easy to see exactly what the client sent.
	if ui.IsActive(ui.RestLogger) {
		b, _ := json.MarshalIndent(task, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		ui.WriteLog(ui.RestLogger, "table.tx.payload", ui.A{
			"session": sessionID,
			"id":      id,
			"body":    string(b)})
	}

	// Only bother scanning if there are symbols to substitute.
	if syms != nil && len(syms.symbols) > 0 {
		// Substitute in the table name (e.g. table: "{{target_table}}").
		task.Table, err = applySymbolsToString(sessionID, task.Table, syms, "Table name")
		if err != nil {
			return err
		}

		// Substitute in each filter string (e.g. "EQ(age, {{min_age}})").
		for n := 0; n < len(task.Filters); n++ {
			task.Filters[n], err = applySymbolsToString(sessionID, task.Filters[n], syms, "Filter")
			if err != nil {
				return err
			}
		}

		// Substitute in each column selector string.
		for n := 0; n < len(task.Columns); n++ {
			task.Columns[n], err = applySymbolsToString(sessionID, task.Columns[n], syms, "Column selector")
			if err != nil {
				return err
			}
		}

		// Substitute in the raw SQL string (used by the "sql" opcode).
		task.SQL, err = applySymbolsToString(sessionID, task.SQL, syms, "SQL statement")
		if err != nil {
			return err
		}

		// For the Data map (column name → value), we need to handle both the
		// keys and the values. We snapshot the key list first because we may
		// rename keys during the loop.
		keys := make([]string, 0)
		for key := range task.Data {
			keys = append(keys, key)
		}

		for _, key := range keys {
			oldKey := key

			// A key might itself be a symbol reference, e.g. "{{col_name}}".
			newKey, err := applySymbolsToString(sessionID, key, syms, "Column name")
			if err != nil {
				return err
			}

			value := task.Data[key]

			// If the key name changed, remove the old entry so the map stays clean.
			if oldKey != newKey {
				delete(task.Data, oldKey)
			}

			// applySymbolsToItem handles the case where the value itself is a bare
			// symbol reference (e.g. value "{{user_id}}"), replacing the whole value
			// rather than doing a string substitution inside it.
			task.Data[newKey], err = applySymbolsToItem(sessionID, value, syms, "Column value")
			if err != nil {
				return err
			}
		}
	}

	// Substitute in the error-condition strings even when the symbol table is
	// empty, because the conditions are always evaluated (they may reference
	// _rows_ or _all_rows_ which are injected separately in Handler).
	for n := 0; n < len(task.Errors); n++ {
		newConditionString, err := applySymbolsToString(sessionID, task.Errors[n].Condition, syms, "Condition")
		if err != nil {
			return err
		}

		if newConditionString != task.Errors[n].Condition {
			task.Errors[n].Condition = newConditionString
		}
	}

	return nil
}

// applySymbolsToItem handles the case where a data value is itself a bare symbol
// reference — a string of the exact form "{{name}}" with nothing else around it.
// In that case the symbol's stored value (which may be any type, not just a string)
// replaces the entire input value. This preserves the original type (e.g. an integer
// stays an integer rather than becoming its string representation).
//
// If the input is not a bare symbol reference, it is returned unchanged.
// If it is a reference but the symbol is not found, an error is returned.
func applySymbolsToItem(sessionID int, input any, symbols *symbolTable, label string) (any, error) {
	if symbols == nil || symbols.symbols == nil {
		return input, nil
	}

	stringRepresentation := data.String(input)
	if strings.HasPrefix(stringRepresentation, symbolPrefix) && strings.HasSuffix(stringRepresentation, symbolSuffix) {
		// Strip the {{ and }} delimiters to get the symbol name.
		key := strings.TrimPrefix(strings.TrimSuffix(stringRepresentation, symbolSuffix), symbolPrefix)

		if value, ok := symbols.symbols[key]; ok {
			oldInput := input
			input = value
			ui.Log(ui.TableLogger, "table.symbol", ui.A{
				"session": sessionID,
				"label":   label,
				"value":   input,
				"name":    oldInput,
			})
		} else {
			return "", errors.ErrNoSuchTXSymbol.Context(key)
		}
	}

	return input, nil
}

// applySymbolsToString replaces every {{name}} occurrence inside a string with
// the corresponding symbol value (converted to a string). Multiple references in
// the same string are all expanded in a single pass.
//
// Example: input "GE(age, {{target}})" with symbol target=25 → "GE(age, 25)".
//
// Note: the replacement is a plain string conversion. If the symbol value is a
// string that needs to appear quoted inside a filter expression (e.g. a name
// comparison), the caller must include the quotes in the input string around the
// {{name}} reference: "EQ(name, '{{user}}')" → "EQ(name, 'alice')".
//
// After all substitutions, any remaining {{...}} sequences indicate a reference
// to an undefined symbol and cause an error to be returned.
func applySymbolsToString(sessionID int, input string, syms *symbolTable, label string) (string, error) {
	if syms == nil || len(syms.symbols) == 0 {
		return input, nil
	}

	for k, v := range syms.symbols {
		search := symbolPrefix + k + symbolSuffix
		replace := data.String(v)
		oldInput := input
		input = strings.ReplaceAll(input, search, replace)

		ui.Log(ui.TableLogger, "table.symbol", ui.A{
			"session": sessionID,
			"label":   label,
			"value":   input,
			"name":    oldInput,
		})
	}

	// After substitution, scan for any remaining {{ }} pairs. Their presence
	// means a symbol reference was not resolved — report it as an error.
	p1 := strings.Index(input, symbolPrefix)
	p2 := strings.Index(input, symbolSuffix)

	if p1 >= 0 && p2 >= 0 {
		// Extract the unresolved symbol name for a helpful error message.
		key := ""
		if p1 < p2 {
			key = input[p1+2 : p2]
		}

		if key != "" {
			return "", errors.ErrNoSuchTXSymbol.Context(key)
		}

		return "", errors.ErrNoSuchTXSymbol
	}

	return input, nil
}

// doSymbols handles the "symbols" opcode. It copies every key/value pair from
// the task's Data map into the per-transaction symbol table so that later
// operations can reference those values via {{name}} substitution.
//
// The "symbols" opcode does not touch the database at all. It is purely a
// way for the client to inject named values at the start of a transaction
// (or at any point in the sequence).
//
// Filters, Columns, and Table are rejected with an error because they have no
// meaning for a symbol-loading operation.
func doSymbols(sessionID int, task defs.TXOperation, id int, symbols *symbolTable) (int, error) {
	// Expand any {{name}} references in the task fields before processing.
	if err := applySymbolsToTask(sessionID, &task, id, symbols); err != nil {
		return http.StatusBadRequest, errors.New(err)
	}

	if len(task.Filters) > 0 {
		return http.StatusBadRequest, errors.ErrTaskSymbolsUnsupported.Context("filters")
	}

	if len(task.Columns) > 0 {
		return http.StatusBadRequest, errors.ErrTaskSymbolsUnsupported.Context("columns")
	}

	if task.Table != "" {
		return http.StatusBadRequest, errors.ErrTaskSymbolsUnsupported.Context("table name")
	}

	msg := strings.Builder{}

	// Copy each data item into the symbol table, initializing it if needed.
	for key, value := range task.Data {
		if symbols.symbols == nil {
			symbols.symbols = map[string]any{}
		}

		symbols.symbols[key] = value

		msg.WriteString(key)
		msg.WriteString(": ")
		msg.WriteString(data.String(value))
	}

	return http.StatusOK, nil
}
