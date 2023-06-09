package scripting

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/http/tables/parsing"
)

func doDelete(sessionID int, user string, tx *sql.Tx, task txOperation, id int, syms *symbolTable, provider string) (int, int, error) {
	if e := applySymbolsToTask(sessionID, &task, id, syms); e != nil {
		return 0, http.StatusBadRequest, errors.NewError(e)
	}

	tableName, _ := parsing.FullName(user, task.Table)

	if len(task.Columns) > 0 {
		return 0, http.StatusBadRequest, errors.NewMessage("columns not supported for DELETE task")
	}

	if where := parsing.WhereClause(task.Filters); where == "" {
		if settings.GetBool(defs.TablesServerEmptyFilterError) {
			return 0, http.StatusBadRequest, errors.NewMessage("operation invalid with empty filter")
		}
	}

	fakeURL, _ := url.Parse(fmt.Sprintf("http://localhost/tables/%s/rows", task.Table))

	q := parsing.FormSelectorDeleteQuery(fakeURL, task.Filters, "", tableName, user, deleteVerb, provider)
	if p := strings.Index(q, parsing.SyntaxErrorPrefix); p >= 0 {
		return 0, http.StatusBadRequest, errors.NewMessage(filterErrorMessage(q))
	}

	ui.Log(ui.SQLLogger, "[%d] Exec: %s", sessionID, q)

	rows, err := tx.Exec(q)
	if err == nil {
		count, _ := rows.RowsAffected()

		if count == 0 && task.EmptyError {
			return 0, http.StatusNotFound, errors.NewMessage("delete did not modify any rows")
		}

		ui.Log(ui.TableLogger, "[%d] Deleted %d rows; %d", sessionID, count, 200)

		return int(count), http.StatusOK, nil
	}

	return 0, http.StatusBadRequest, errors.NewError(err)
}
