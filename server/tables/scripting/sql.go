package scripting

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

func doSQL(sessionID int, user string, tx *sql.Tx, task txOperation, id int, syms *symbolTable) (int, int, error) {
	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return 0, http.StatusBadRequest, errors.New(err)
	}

	if len(task.Columns) > 0 {
		return 0, http.StatusBadRequest, errors.Message("columns not supported for SQL task")
	}

	if len(task.Filters) > 0 {
		return 0, http.StatusBadRequest, errors.Message("filters not supported for SQL task")
	}

	if len(strings.TrimSpace(task.SQL)) == 0 {
		return 0, http.StatusBadRequest, errors.Message("missing SQL command for SQL task")
	}

	if len(strings.TrimSpace(task.Table)) != 0 {
		return 0, http.StatusBadRequest, errors.Message("table name not supported for SQL task")
	}

	q := task.SQL

	ui.Log(ui.SQLLogger, "[%d] Exec: %s", sessionID, q)

	rows, err := tx.Exec(q)
	if err == nil {
		count, _ := rows.RowsAffected()

		if count == 0 && task.EmptyError {
			return 0, http.StatusNotFound, errors.Message("sql did not modify any rows")
		}

		ui.Log(ui.TableLogger, "[%d] Affected %d rows; %d", sessionID, count, http.StatusOK)

		return int(count), http.StatusOK, nil
	}

	return 0, http.StatusBadRequest, errors.New(err)
}
