package scripting

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

func doSQL(sessionID int, tx *sql.Tx, task txOperation, id int, syms *symbolTable) (int, int, error) {
	var (
		err   error
		count int
	)

	if err = applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return count, http.StatusBadRequest, errors.New(err)
	}

	if len(task.Columns) > 0 {
		return count, http.StatusBadRequest, errors.ErrTaskSQLUnsupported.Context("columns")
	}

	if len(task.Filters) > 0 {
		return count, http.StatusBadRequest, errors.ErrTaskSQLUnsupported.Context("filters")
	}

	if len(strings.TrimSpace(task.SQL)) == 0 {
		return count, http.StatusBadRequest, errors.ErrTaskSQLMissing
	}

	if len(strings.TrimSpace(task.Table)) != 0 {
		return count, http.StatusBadRequest, errors.ErrTaskSQLUnsupported.Context("table name")
	}

	q := task.SQL

	ui.Log(ui.SQLLogger, "sql.exec", ui.A{
		"session": sessionID,
		"sql":     q})

	rows, err := tx.Exec(q)
	if err == nil {
		if affectedCount, err := rows.RowsAffected(); err == nil {
			count = int(affectedCount)
		}

		if count == 0 && task.EmptyError {
			return count, http.StatusNotFound, errors.Message("sql did not modify any rows")
		}

		ui.Log(ui.TableLogger, "table.affected", ui.A{
			"session": sessionID,
			"count":   count,
			"status":  http.StatusOK})

		return count, http.StatusOK, nil
	}

	return count, http.StatusBadRequest, errors.New(err)
}
