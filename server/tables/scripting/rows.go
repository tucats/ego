package scripting

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
)

// doRows handles the "readrows" opcode (and SQL SELECT statements promoted from
// the "sql" opcode in Handler). It runs a SELECT query and stores the complete
// result set — all matching rows — as a single symbol under the key
// resultSetSymbolName. After the transaction commits, Handler detects that key
// and returns its value as the HTTP response body (a defs.DBRowSet), rather
// than returning the plain row-count response.
//
// The SQL query is taken from task.SQL when present (e.g. for a raw "select …"
// statement). Otherwise it is built from task.Table, task.Filters, and
// task.Columns just like doSelect.
//
// Unlike doSelect, any number of rows is valid. If task.EmptyError is true and
// zero rows are returned, the operation fails with 404.
func doRows(sessionID int, user string, db *database.Database, task defs.TXOperation, id int, syms *symbolTable) (int, int, error) {
	var (
		err    error
		count  int
		status int
		q      = task.SQL
	)

	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return 0, http.StatusBadRequest, err
	}

	tableName, _ := parsing.FullName(user, task.Table)
	fakeURL, _ := url.Parse("http://localhost/tables/" + task.Table + "/rows?limit=1")

	if q == "" {
		q, err = parsing.FormSelectorDeleteQuery(fakeURL, task.Filters, strings.Join(task.Columns, ","), tableName, user, selectVerb, db.Provider)
		if err != nil {
			return count, http.StatusBadRequest, errors.Message(filterErrorMessage(q))
		}
	}

	count, status, err = readTxRowResultSet(db, q, sessionID, syms, task.EmptyError)
	if err == nil {
		return count, status, nil
	}

	ui.Log(ui.TableLogger, "table.read.error", ui.A{
		"session": sessionID,
		"sql":     q,
		"error":   err})

	return 0, status, errors.New(err)
}

// readTxRowResultSet executes query q, collects every row into a
// []map[string]any slice, and stores the slice under resultSetSymbolName in
// the symbol table. Each map represents one row, keyed by column name.
//
// Any previous result set stored under resultSetSymbolName is deleted before
// the new query runs — there can be at most one result set per transaction.
//
// emptyResultError controls whether zero rows is treated as an error:
//   - true  → 404 + ErrTableNoRows
//   - false → success; the stored slice is empty but present
//
// If the query itself fails, status is set to 400 and the error is wrapped and
// returned; in that case the result set is not stored.
func readTxRowResultSet(db *database.Database, q string, sessionID int, syms *symbolTable, emptyResultError bool) (int, int, error) {
	var (
		rows     *sql.Rows
		err      error
		rowCount int
		result   = []map[string]any{}
		status   = http.StatusOK
	)

	// If the symbol table doesn't exist, create it. If it does, delete any
	// previous result set (to quote the Highlander, "there can be only one.")
	if syms == nil || len(syms.symbols) == 0 {
		*syms = symbolTable{symbols: map[string]any{}}
	} else {
		delete(syms.symbols, resultSetSymbolName)
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

			err = rows.Scan(rowPointers...)
			if err == nil {
				newRow := map[string]any{}
				for i, v := range row {
					newRow[columnNames[i]] = v
				}

				result = append(result, newRow)
				rowCount++
			}
		}

		syms.symbols[resultSetSymbolName] = result

		ui.Log(ui.TableLogger, "table.read", ui.A{
			"session": sessionID,
			"rows":    rowCount,
			"columns": columnCount,
			"status":  status})
	} else {
		status = http.StatusBadRequest
	}

	if rowCount == 0 && emptyResultError {
		return rowCount, http.StatusNotFound, errors.ErrTableNoRows
	}

	if err != nil {
		err = errors.New(err)
	}

	return rowCount, status, err
}
