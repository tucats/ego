package scripting

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/database"
)

func doSQL(sessionID int, db *database.Database, task txOperation, id int, syms *symbolTable) (int, int, error) {
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

	rows, err := db.Exec(q)
	if err == nil {
		if affectedCount, err := rows.RowsAffected(); err == nil {
			count = int(affectedCount)
		}

		if count == 0 && task.EmptyError {
			return count, http.StatusNotFound, errors.ErrTableRowsNoChanges
		}

		ui.Log(ui.TableLogger, "table.affected", ui.A{
			"session": sessionID,
			"count":   count,
			"status":  http.StatusOK})

		return count, http.StatusOK, nil
	}

	return count, http.StatusBadRequest, errors.New(err)
}
