package scripting

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/http/tables/parsing"
)

func doSelect(sessionID int, user string, db *sql.DB, tx *sql.Tx, task txOperation, id int, syms *symbolTable) (int, int, error) {
	var err error

	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return 0, http.StatusBadRequest, errors.NewError(err)
	}

	tableName, _ := parsing.FullName(user, task.Table)
	count := 0

	fakeURL, _ := url.Parse("http://localhost/tables/" + task.Table + "/rows?limit=1")

	q := parsing.FormSelectorDeleteQuery(fakeURL, task.Filters, strings.Join(task.Columns, ","), tableName, user, selectVerb)
	if p := strings.Index(q, parsing.SyntaxErrorPrefix); p >= 0 {
		return count, http.StatusBadRequest, errors.NewMessage(filterErrorMessage(q))
	}

	ui.Log(ui.SQLLogger, "[%d] Query: %s", sessionID, q)

	var status int

	count, status, err = readTxRowData(db, tx, q, sessionID, syms, task.EmptyError)
	if err == nil {
		return count, status, nil
	}

	ui.Log(ui.TableLogger, "[%d] Error reading table, %v", sessionID, err)

	return 0, status, errors.NewError(err)
}

func readTxRowData(db *sql.DB, tx *sql.Tx, q string, sessionID int, syms *symbolTable, emptyResultError bool) (int, int, error) {
	var rows *sql.Rows

	var err error

	rowCount := 0
	status := http.StatusOK

	if syms == nil || len(syms.symbols) == 0 {
		*syms = symbolTable{symbols: map[string]interface{}{}}
	}

	rows, err = db.Query(q)
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

			// Get the next row values. Note we only incorporate them into the symbol
			// table on the first row (rowCount of zero), the rest are ignored. An error
			// will be thrown later.
			err = rows.Scan(rowptrs...)
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

				ui.Log(ui.TableLogger, "[%d] Read table to set symbols: %s", sessionID, msg.String())

				rowCount++
			}
		}

		if rowCount == 0 && emptyResultError {
			status = http.StatusNotFound
			err = errors.NewMessage("SELECT task did not return any row data")
		} else if rowCount > 1 {
			err = errors.NewMessage("SELECT task did not return a unique row")
			status = http.StatusBadRequest

			ui.Log(ui.TableLogger, "[%d] Invalid read of %d rows ", sessionID, rowCount)
		} else {
			ui.Log(ui.TableLogger, "[%d] Read %d rows of %d columns", sessionID, rowCount, columnCount)
		}
	} else {
		status = http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			status = http.StatusNotFound
		}
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return rowCount, status, err
}
