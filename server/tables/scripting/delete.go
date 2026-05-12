package scripting

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
)

// doDelete handles the "delete" opcode. It deletes every row from task.Table
// that matches the filter expressions in task.Filters.
//
// Validation rules enforced before the query runs:
//   - task.Columns must be empty — a DELETE has no column list.
//   - If the defs.TablesServerEmptyFilterError preference is set, an empty
//     filter (which would delete all rows) is rejected with 400. This is a
//     safety guard against accidental full-table wipes.
//
// If task.EmptyError is true and no rows were deleted, the operation returns
// 404 so the caller can detect a no-op delete.
//
// Returns (rowsDeleted, httpStatus, error).
func doDelete(sessionID int, user string, db *database.Database, task defs.TXOperation, id int, syms *symbolTable) (int, int, error) {
	if e := applySymbolsToTask(sessionID, &task, id, syms); e != nil {
		return 0, http.StatusBadRequest, errors.New(e)
	}

	if len(task.Columns) > 0 {
		return 0, http.StatusBadRequest, errors.ErrTaskDeleteUnsupported.Context("columns")
	}

	if where, err := parsing.WhereClause(task.Filters); where == "" {
		if settings.GetBool(defs.TablesServerEmptyFilterError) {
			return 0, http.StatusBadRequest, errors.ErrTaskFilterRequired
		}
	} else if err != nil {
		return 0, http.StatusBadRequest, errors.New(err)
	}

	fakeURL, _ := url.Parse(fmt.Sprintf("http://localhost/tables/%s/rows", task.Table))

	q, err := parsing.FormSelectorDeleteQuery(fakeURL, task.Filters, "", task.Table, user, deleteVerb, db.Provider)
	if err != nil {
		return 0, http.StatusBadRequest, errors.Message(filterErrorMessage(q))
	}

	rows, err := db.Exec(q)
	if err == nil {
		count, _ := rows.RowsAffected()

		if count == 0 && task.EmptyError {
			return 0, http.StatusNotFound, errors.ErrTableRowsNoChanges
		}

		ui.Log(ui.TableLogger, "table.deleted.rows", ui.A{
			"session": sessionID,
			"count":   count,
			"status":  http.StatusOK})

		return int(count), http.StatusOK, nil
	}

	return 0, http.StatusBadRequest, errors.New(err)
}
