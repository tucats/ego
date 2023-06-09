package scripting

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/http/tables/parsing"
)

func doRows(sessionID int, user string, db *sql.DB, tx *sql.Tx, task txOperation, id int, syms *symbolTable, provider string) (int, int, error) {
	var err error

	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return 0, http.StatusBadRequest, err
	}

	tableName, _ := parsing.FullName(user, task.Table)
	count := 0

	fakeURL, _ := url.Parse("http://localhost/tables/" + task.Table + "/rows?limit=1")

	q := task.SQL
	if q == "" {
		q = parsing.FormSelectorDeleteQuery(fakeURL, task.Filters, strings.Join(task.Columns, ","), tableName, user, selectVerb, provider)
		if p := strings.Index(q, parsing.SyntaxErrorPrefix); p >= 0 {
			return count, http.StatusBadRequest, errors.NewMessage(filterErrorMessage(q))
		}
	}

	ui.Log(ui.SQLLogger, "[%d] Query: %s", sessionID, q)

	var status int

	count, status, err = readTxRowResultSet(db, tx, q, sessionID, syms, task.EmptyError)
	if err == nil {
		return count, status, nil
	}

	ui.Log(ui.TableLogger, "[%d] Error reading table, %v", sessionID, err)

	return 0, status, errors.NewError(err)
}

func readTxRowResultSet(db *sql.DB, tx *sql.Tx, q string, sessionID int, syms *symbolTable, emptyResultError bool) (int, int, error) {
	// If the symbol table doesn't exist, create it. If it does, delete any
	// previous result set (to quote the Highlander, "there can be only one.")
	if syms == nil || len(syms.symbols) == 0 {
		*syms = symbolTable{symbols: map[string]interface{}{}}
	} else {
		delete(syms.symbols, resultSetSymbolName)
	}

	var rows *sql.Rows

	var err error

	result := []map[string]interface{}{}
	rowCount := 0
	status := http.StatusOK

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

	if err != nil {
		err = errors.NewError(err)
	}

	return rowCount, status, err
}
