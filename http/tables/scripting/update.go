package scripting

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/http/tables/database"
	"github.com/tucats/ego/http/tables/parsing"
)

func doUpdate(sessionID int, user string, db *database.Database, tx *sql.Tx, task txOperation, id int, syms *symbolTable) (int, int, error) {
	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return 0, http.StatusBadRequest, errors.NewError(err)
	}

	tableName, _ := parsing.FullName(user, task.Table)

	validColumns, err := getColumnInfo(db, user, tableName, sessionID)
	if err != nil {
		msg := "Unable to read table metadata, " + err.Error()

		return 0, http.StatusBadRequest, errors.NewMessage(msg)
	}

	// Make sure none of the columns in the update are non-existent
	for k := range task.Data {
		valid := false

		for _, column := range validColumns {
			if column.Name == k {
				valid = true

				break
			}
		}

		if !valid {
			msg := "insert task references non-existent column: " + k

			return 0, http.StatusBadRequest, errors.NewMessage(msg)
		}
	}

	// Is there columns list for this task that should be used to determine
	// which parts of the payload to use?
	if len(task.Columns) > 0 {
		// Make sure none of the columns in the columns are non-existent
		for _, name := range task.Columns {
			valid := false

			for _, k := range validColumns {
				if name == k.Name {
					valid = true

					break
				}
			}

			if !valid {
				msg := "insert task references non-existent column: " + name

				return 0, http.StatusBadRequest, errors.NewMessage(msg)
			}
		}

		// The columns list is valid, so use it to thin out the task payload
		keepList := map[string]bool{}

		for k := range task.Data {
			keepList[k] = false
		}

		for _, columnName := range task.Columns {
			keepList[columnName] = true
		}

		for k, keep := range keepList {
			if !keep {
				delete(task.Data, k)
			}
		}
	}

	// Form the update query. We start with a list of the keys to update
	// in a predictable order
	var result strings.Builder

	var values []interface{}

	var keys []string

	for key := range task.Data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	result.WriteString("UPDATE ")
	result.WriteString(tableName)

	// Loop over the item names and add SET clauses for each one. We always
	// ignore the rowid value because you cannot update it on an UPDATE call;
	// it is only set on an insert.
	columnPosition := 0

	for _, key := range keys {
		if key == defs.RowIDName {
			continue
		}

		// Add the value to the list of values that will be passed to the Exec()
		// function later. These must be in the same order that the column names
		// are specified in the query text.
		values = append(values, task.Data[key])

		if columnPosition == 0 {
			result.WriteString(" SET ")
		} else {
			result.WriteString(", ")
		}

		columnPosition++

		result.WriteString("\"" + key + "\"")
		result.WriteString(fmt.Sprintf(" = $%d", columnPosition))
	}

	// If there is a filter, then add that as well. And fail if there
	// isn't a filter but must be
	if filter := parsing.WhereClause(task.Filters); filter != "" {
		if p := strings.Index(filter, parsing.SyntaxErrorPrefix); p >= 0 {
			return 0, http.StatusBadRequest, errors.NewMessage(filterErrorMessage(filter))
		}

		result.WriteString(filter)
	} else if settings.GetBool(defs.TablesServerEmptyFilterError) {
		return 0, http.StatusBadRequest, errors.NewMessage("update without filter is not allowed")
	}

	ui.Log(ui.SQLLogger, "[%d] Exec: %s", sessionID, result.String())

	status := http.StatusOK

	var count int64 = 0

	queryResult, updateErr := tx.Exec(result.String(), values...)
	if updateErr == nil {
		count, _ = queryResult.RowsAffected()
		if count == 0 && task.EmptyError {
			status = http.StatusNotFound
			updateErr = errors.NewMessage("update did not modify any rows")
		}
	} else {
		updateErr = errors.NewError(updateErr)
		status = http.StatusBadRequest
		if strings.Contains(updateErr.Error(), "constraint") {
			status = http.StatusConflict
		}
	}

	return int(count), status, updateErr
}
