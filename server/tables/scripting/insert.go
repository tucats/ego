package scripting

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
)

func doInsert(sessionID int, user string, db *database.Database, tx *sql.Tx, task txOperation, id int, syms *symbolTable) (int, error) {
	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return http.StatusBadRequest, errors.New(err)
	}

	if len(task.Filters) > 0 {
		return http.StatusBadRequest, errors.Message("filters not supported for INSERT task")
	}

	if len(task.Columns) > 0 {
		return http.StatusBadRequest, errors.Message("columns not supported for INSERT task")
	}

	// Get the column metadata for the table we're insert into, so we can validate column info.
	tableName, _ := parsing.FullName(user, task.Table)

	columns, err := getColumnInfo(db, user, tableName, sessionID)
	if err != nil {
		return http.StatusBadRequest, errors.Message("unable to read table metadata; " + err.Error())
	}

	// It's a new row, so assign a UUID now. This overrides any previous item in the payload
	// for _row_id_ or creates it if not found. Row IDs are always assigned on insert only.
	task.Data[defs.RowIDName] = uuid.New().String()

	for _, column := range columns {
		v, ok := task.Data[column.Name]
		if !ok && settings.GetBool(defs.TableServerPartialInsertError) {
			expectedList := make([]string, 0)
			for _, k := range columns {
				expectedList = append(expectedList, k.Name)
			}

			providedList := make([]string, 0)
			for k := range task.Data {
				providedList = append(providedList, k)
			}

			sort.Strings(expectedList)
			sort.Strings(providedList)

			msg := fmt.Sprintf("Payload did not include data for \"%s\"; expected %v but payload contained %v",
				column.Name, strings.Join(expectedList, ","), strings.Join(providedList, ","))

			return http.StatusBadRequest, errors.Message(msg)
		}

		// If it's one of the date/time values, make sure it is wrapped in single qutoes.
		if parsing.KeywordMatch(column.Type, "time", "date", "timestamp") {
			text := strings.TrimPrefix(strings.TrimSuffix(data.String(v), "\""), "\"")
			task.Data[column.Name] = "'" + strings.TrimPrefix(strings.TrimSuffix(text, "'"), "'") + "'"
			ui.Log(ui.TableLogger, "[%d] Updated column %s value from %v to %v", sessionID, column.Name, v, task.Data[column.Name])
		}
	}

	q, values := parsing.FormInsertQuery(task.Table, user, db.Provider, task.Data)
	ui.Log(ui.TableLogger, "[%d] Exec: %s", sessionID, q)

	_, e := tx.Exec(q, values...)
	if e != nil {
		status := http.StatusBadRequest
		if strings.Contains(e.Error(), "constraint") {
			status = http.StatusConflict
		}

		return status, errors.Message("error inserting row; " + e.Error())
	}

	ui.Log(ui.TableLogger, "[%d] Successful INSERT to %s", sessionID, tableName)

	return http.StatusOK, nil
}
