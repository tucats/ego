package scripting

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
)

func doDrop(sessionID int, user string, db *database.Database, task txOperation, id int, syms *symbolTable) (int, error) {
	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return http.StatusBadRequest, errors.New(err)
	}

	table, _ := parsing.FullName(user, task.Table)

	if len(task.Filters) > 0 {
		return http.StatusBadRequest, errors.ErrTaskDropUnsupported.Context("filters")
	}

	if len(task.Columns) > 0 {
		return http.StatusBadRequest, errors.ErrTaskDropUnsupported.Context("columns")
	}

	q := "DROP TABLE ?"
	_, err := db.Exec(q, table)

	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError

		if strings.Contains(err.Error(), "no such") || strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}

		err = errors.New(err)
	}

	return status, err
}
