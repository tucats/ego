package scripting

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/database"
)

// doSQL handles the "sql" opcode. It executes the raw, non-SELECT SQL
// statement in task.SQL (e.g. CREATE TABLE, ALTER TABLE, arbitrary DML) and
// returns the number of rows affected.
//
// Validation rules enforced before execution:
//   - task.Columns must be empty — raw SQL has no separate column list.
//   - task.Filters must be empty — filters are for opcode-driven queries only.
//   - task.SQL must be non-empty (after trimming whitespace).
//   - task.Table must be empty — the table name (if any) must be embedded
//     directly in the SQL string.
//
// Note: SELECT statements are detected in Handler before dispatch and promoted
// to the "readrows" opcode, so doSQL never receives a SELECT.
//
// If task.EmptyError is true and no rows were affected, returns 404.
// Returns (rowsAffected, httpStatus, error).
func doSQL(sessionID int, db *database.Database, task defs.TXOperation, id int, syms *symbolTable) (int, int, error) {
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
