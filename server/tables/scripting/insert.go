package scripting

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
)

// doInsert handles the "insert" opcode. It inserts a single new row into
// task.Table using the column→value pairs in task.Data.
//
// Steps performed:
//  1. Symbol substitution ({{name}} in keys and values).
//  2. Validation: task.Filters and task.Columns must be empty — INSERT does
//     not use a WHERE clause or a separate column-selector list.
//  3. Column metadata fetch (getColumnInfo) so that:
//     a. If defs.TableServerPartialInsertError is set, every table column must
//        have a corresponding entry in task.Data; missing columns are rejected.
//     b. Date/time column values are wrapped in single quotes so they are
//        passed to the database as string literals rather than bare identifiers.
//  4. Row-ID assignment: if the database uses row IDs (db.HasRowID), a new
//     UUID is generated and inserted into task.Data under defs.RowIDName,
//     overwriting any value the caller may have supplied.
//  5. Query construction via parsing.FormInsertQuery and execution.
//
// A constraint violation (e.g. duplicate primary key) returns 409 Conflict.
// Returns (httpStatus, error). Row count is always 1 on success; Handler
// increments rowsAffected directly rather than using the return value.
func doInsert(sessionID int, user string, db *database.Database, task defs.TXOperation, id int, syms *symbolTable) (int, error) {
	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return http.StatusBadRequest, errors.New(err)
	}

	if len(task.Filters) > 0 {
		return http.StatusBadRequest, errors.ErrTaskInsertUnsupported.Context("filters")
	}

	if len(task.Columns) > 0 {
		return http.StatusBadRequest, errors.ErrTaskInsertUnsupported.Context("columns")
	}

	// Get the column metadata for the table we're inserting into.
	columns, err := getColumnInfo(db, user, task.Table)
	if err != nil {
		return http.StatusBadRequest, errors.Message("unable to read table metadata; " + err.Error())
	}

	// Assign a row ID UUID only if the table actually has that column. Tables created via
	// the Ego API always have _row_id_, but tables created with a raw SQL "CREATE TABLE"
	// (e.g. via the @transaction sql opcode) may not.
	if db.HasRowID {
		for _, col := range columns {
			if col.Name == defs.RowIDName {
				task.Data[defs.RowIDName] = egostrings.Gibberish(uuid.New())

				break
			}
		}
	}

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

			msg := fmt.Sprintf("Payload did not include data for %s; expected %v but payload contained %v",
				strconv.Quote(column.Name), strings.Join(expectedList, ","), strings.Join(providedList, ","))

			return http.StatusBadRequest, errors.Message(msg)
		}

		// If it's one of the date/time values, make sure it is wrapped in single quotes.
		if parsing.KeywordMatch(column.Type, "time", "date", "timestamp") {
			text := strings.TrimPrefix(strings.TrimSuffix(data.String(v), "\""), "\"")
			task.Data[column.Name] = "'" + strings.TrimPrefix(strings.TrimSuffix(text, "'"), "'") + "'"
		}
	}

	q, values, err := parsing.FormInsertQuery(task.Table, user, db.Provider, columns, task.Data)
	if err != nil {
		_ = db.Rollback()

		return 0, errors.New(err)
	}

	_, e := db.Exec(q, values...)
	if e != nil {
		status := http.StatusBadRequest
		if strings.Contains(e.Error(), "constraint") {
			status = http.StatusConflict
		}

		return status, e
	}

	return http.StatusOK, nil
}
