package scripting

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/parsing"
)

func doRows(sessionID int, user string, tx *sql.Tx, task txOperation, id int, syms *symbolTable, provider string) (int, int, error) {
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
		q, err = parsing.FormSelectorDeleteQuery(fakeURL, task.Filters, strings.Join(task.Columns, ","), tableName, user, selectVerb, provider)
		if err != nil {
			return count, http.StatusBadRequest, errors.Message(filterErrorMessage(q))
		}
	}

	ui.Log(ui.SQLLogger, "[%d] Query: %s", sessionID, q)

	count, status, err = readTxRowResultSet(tx, q, sessionID, syms, task.EmptyError)
	if err == nil {
		return count, status, nil
	}

	ui.Log(ui.TableLogger, "[%d] Error reading table, %v", sessionID, err)

	return 0, status, errors.New(err)
}

func readTxRowResultSet(tx *sql.Tx, q string, sessionID int, syms *symbolTable, emptyResultError bool) (int, int, error) {
	var (
		rows     *sql.Rows
		err      error
		rowCount int
		result   = []map[string]interface{}{}
		status   = http.StatusOK
	)

	// If the symbol table doesn't exist, create it. If it does, delete any
	// previous result set (to quote the Highlander, "there can be only one.")
	if syms == nil || len(syms.symbols) == 0 {
		*syms = symbolTable{symbols: map[string]interface{}{}}
	} else {
		delete(syms.symbols, resultSetSymbolName)
	}

	rows, err = tx.Query(q)
	if err == nil {
		defer rows.Close()

		columnNames, _ := rows.Columns()
		columnCount := len(columnNames)

		for rows.Next() {
			row := make([]interface{}, columnCount)
			rowptrs := make([]interface{}, columnCount)

			for i := range row {
				rowptrs[i] = &row[i]
			}

			err = rows.Scan(rowptrs...)
			if err == nil {
				newRow := map[string]interface{}{}
				for i, v := range row {
					newRow[columnNames[i]] = v
				}

				result = append(result, newRow)
				rowCount++
			}
		}

		syms.symbols[resultSetSymbolName] = result

		ui.Log(ui.TableLogger, "[%d] Read %d rows of %d columns; %d", sessionID, rowCount, columnCount, status)
	} else {
		status = http.StatusBadRequest
	}

	if rowCount == 0 && emptyResultError {
		return rowCount, http.StatusNotFound, errors.Message("sql did not return any rows")
	}

	if err != nil {
		err = errors.New(err)
	}

	return rowCount, status, err
}
