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

func doDelete(sessionID int, user string, db *database.Database, task txOperation, id int, syms *symbolTable) (int, int, error) {
	if e := applySymbolsToTask(sessionID, &task, id, syms); e != nil {
		return 0, http.StatusBadRequest, errors.New(e)
	}

	tableName, _ := parsing.FullName(user, task.Table)

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

	q, err := parsing.FormSelectorDeleteQuery(fakeURL, task.Filters, "", tableName, user, deleteVerb, db.Provider)
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
