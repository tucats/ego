package scripting

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/parsing"
)

func doDrop(sessionID int, user string, db *sql.DB, task txOperation, id int, syms *symbolTable) (int, error) {
	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return http.StatusBadRequest, errors.NewError(err)
	}

	table, _ := parsing.FullName(user, task.Table)

	if len(task.Filters) > 0 {
		return http.StatusBadRequest, errors.NewMessage("filters not supported for DROP task")
	}

	if len(task.Columns) > 0 {
		return http.StatusBadRequest, errors.NewMessage("columns not supported for DROP task")
	}

	q := "DROP TABLE ?"
	_, err := db.Exec(q, table)

	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError

		if strings.Contains(err.Error(), "no such") || strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}

		err = errors.NewError(err)
	}

	return status, err
}
