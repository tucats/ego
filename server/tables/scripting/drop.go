package scripting

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
)

// doDrop handles the "drop" opcode. It executes DROP TABLE for task.Table,
// permanently removing the table and all its data from the database.
//
// Validation rules enforced before the statement runs:
//   - task.Filters must be empty — a DROP TABLE has no WHERE clause.
//   - task.Columns must be empty — a DROP TABLE has no column list.
//
// If the database reports that the table does not exist (error message contains
// "no such" or "not found"), the HTTP status is 404 rather than 500 so the
// caller can distinguish "table was already gone" from a genuine server fault.
//
// Returns (httpStatus, error). Unlike the other handlers there is no row count
// because DROP TABLE does not affect individual rows.
func doDrop(sessionID int, user string, db *database.Database, task defs.TXOperation, id int, syms *symbolTable) (int, error) {
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
