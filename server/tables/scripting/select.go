package scripting

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
)

// doSelect handles the "select" opcode. It runs a SELECT query and stores the
// column values of the first matching row directly into the per-transaction
// symbol table — e.g. if the row has columns "age" and "name", afterwards
// syms["age"] and syms["name"] hold those values. Later operations in the same
// transaction can reference them via {{age}} or {{name}} substitution.
//
// Exactly one row is expected:
//   - Zero rows: if task.EmptyError is true, returns 404; otherwise succeeds
//     with a count of 0 (symbols unchanged).
//   - More than one row: always returns an error — the caller must add filters
//     that narrow the result to a single row.
//
// The query is built from task.Table, task.Filters (WHERE clause), and
// task.Columns (SELECT list). A "limit=1" hint is embedded in the fake URL
// passed to the query builder as a safety net.
func doSelect(sessionID int, user string, db *database.Database, task defs.TXOperation, id int, syms *symbolTable) (int, int, error) {
	var (
		err    error
		count  int
		status int
	)

	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return 0, http.StatusBadRequest, errors.New(err)
	}

	fakeURL, _ := url.Parse("http://localhost/tables/" + task.Table + "/rows?limit=1")

	q, err := parsing.FormSelectorDeleteQuery(fakeURL, task.Filters, strings.Join(task.Columns, ","), task.Table, user, selectVerb, db.Provider)
	if err != nil {
		return count, http.StatusBadRequest, errors.Message(filterErrorMessage(q))
	}

	count, status, err = readTxRowData(db, q, sessionID, syms, task.EmptyError)
	if err == nil {
		return count, status, nil
	}

	ui.Log(ui.TableLogger, "table.read.error", ui.A{
		"session": sessionID,
		"sql":     q,
		"error":   err})

	return 0, status, errors.New(err)
}

// readTxRowData executes query q, expects exactly one row back, and stores each
// column value into the symbol table under the column's name.
//
// Only the first row is stored; subsequent rows are counted but otherwise
// ignored. If more than one row is returned an error is returned (the caller
// should use filters to guarantee uniqueness).
//
// emptyResultError controls whether zero rows is treated as an error:
//   - true  → 404 + ErrTableSelectNone
//   - false → success with rowCount == 0 and no symbols written
//
// If the query itself fails with a message that looks like "does not exist"
// (e.g. an unknown table), the status is promoted from 400 to 404.
func readTxRowData(db *database.Database, q string, sessionID int, syms *symbolTable, emptyResultError bool) (int, int, error) {
	var (
		rows     *sql.Rows
		err      error
		rowCount int
		status   = http.StatusOK
	)

	if syms == nil || len(syms.symbols) == 0 {
		*syms = symbolTable{symbols: map[string]any{}}
	}

	rows, err = db.Query(q)
	if err == nil {
		defer rows.Close()

		columnNames, _ := rows.Columns()
		columnCount := len(columnNames)

		for rows.Next() {
			row := make([]any, columnCount)
			rowPointers := make([]any, columnCount)

			for i := range row {
				rowPointers[i] = &row[i]
			}

			// Get the next row values. Note we only incorporate them into the symbol
			// table on the first row (rowCount of zero), the rest are ignored. An error
			// will be thrown later.
			err = rows.Scan(rowPointers...)
			if err == nil && rowCount == 0 {
				msg := strings.Builder{}

				for i, v := range row {
					syms.symbols[columnNames[i]] = v

					if msg.Len() > 0 {
						msg.WriteString(", ")
					}

					msg.WriteString(columnNames[i])
					msg.WriteString("=")
					msg.WriteString(data.String(v))
				}

				rowCount++
			}
		}

		if rowCount == 0 && emptyResultError {
			status = http.StatusNotFound
			err = errors.ErrTableSelectNone
		} else if rowCount > 1 {
			err = errors.ErrTableSelectUnique
			status = http.StatusBadRequest
		} else {
			ui.Log(ui.TableLogger, "table.read", ui.A{
				"session": sessionID,
				"rows":    rowCount,
				"columns": columnCount,
				"status":  status})
		}
	} else {
		status = http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			status = http.StatusNotFound
		}
	}

	if err != nil {
		err = errors.New(err)
	}

	return rowCount, status, err
}
