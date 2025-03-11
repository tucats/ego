package scripting

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/expressions"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Handler executes a sequence of SQL operations as a single atomic
// transaction. This provides the basic scripting functionality of the
// tables service.
func Handler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Validate the transaction payload.
	tasks := []txOperation{}

	e := json.NewDecoder(r.Body).Decode(&tasks)
	if e != nil {
		return util.ErrorResponse(w, session.ID, "transaction request decode error; "+e.Error(), http.StatusBadRequest)
	}

	ui.Log(ui.TableLogger, "table.tx.count", ui.A{
		"session": session.ID,
		"count":   len(tasks)})

	if len(tasks) == 0 {
		text := "no tasks in transaction"
		session.ResponseLength += len(text)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(text))
		session.ResponseLength += len(text)

		return http.StatusOK
	}

	for n, task := range tasks {
		// Is the opcode missing, but there's a SQL string? If so, assume a "sql" operation
		if task.Opcode == "" && task.SQL != "" {
			tasks[n].Opcode = sqlOpcode
			task = tasks[n]
		}

		// Validate that the opcode is legit...
		opcode := strings.ToLower(task.Opcode)
		if !util.InList(opcode,
			deleteOpcode,
			updateOpcode,
			insertOpcode,
			dropOpCode,
			symbolsOpcode,
			selectOpcode,
			rowsOpcode,
			sqlOpcode,
		) {
			msg := fmt.Sprintf("transaction operation %d has invalid opcode: %s",
				n, opcode)

			return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
		}
	}

	// Access the database and execute the transaction operations
	rowsAffected := 0
	httpStatus := http.StatusOK
	dictionary := symbolTable{symbols: map[string]interface{}{}}

	db, err := database.Open(&session.User, data.String(session.URLParts["dsn"]), dsns.DSNWriteAction+dsns.DSNReadAction)
	if err == nil && db != nil {
		defer db.Close()

		tx, err := db.Begin()
		if err != nil {
			return util.ErrorResponse(w, session.ID, "unable to start transaction; "+err.Error(), http.StatusInternalServerError)
		}

		for n, task := range tasks {
			var operationErr error

			count := 0
			tableName, _ := parsing.FullName(session.User, task.Table)

			if ui.IsActive(ui.TableLogger) {
				if util.InList(strings.ToLower(task.Opcode), symbolsOpcode, sqlOpcode, rowsOpcode) {
					ui.WriteLog(ui.TableLogger, "table.op", ui.A{
						"session": session.ID,
						"op":      strings.ToUpper(task.Opcode)})
				} else {
					ui.WriteLog(ui.TableLogger, "table.op.table", ui.A{
						"session": session.ID,
						"op":      strings.ToUpper(task.Opcode),
						"table":   tableName})
				}
			}

			// If this is a SQL opcode that is a select operation, convert it to
			// a rows opcode
			if strings.EqualFold(task.Opcode, sqlOpcode) {
				if strings.HasPrefix(strings.TrimSpace(strings.ToLower(task.SQL)), "select ") {
					task.Opcode = rowsOpcode
				}
			}

			// Based on the opcode, dispatch the appropriate function to do the
			// specific task.
			switch strings.ToLower(task.Opcode) {
			case sqlOpcode:
				count, httpStatus, operationErr = doSQL(session.ID, tx, task, n+1, &dictionary)
				rowsAffected += count

			case symbolsOpcode:
				httpStatus, operationErr = doSymbols(session.ID, task, n+1, &dictionary)

			case selectOpcode:
				count, httpStatus, operationErr = doSelect(session.ID, session.User, db.Handle, tx, task, n+1, &dictionary, db.Provider)
				rowsAffected += count

			case rowsOpcode:
				count, httpStatus, operationErr = doRows(session.ID, session.User, tx, task, n+1, &dictionary, db.Provider)
				rowsAffected += count

			case updateOpcode:
				count, httpStatus, operationErr = doUpdate(session.ID, session.User, db, tx, task, n+1, &dictionary)
				rowsAffected += count

			case deleteOpcode:
				count, httpStatus, operationErr = doDelete(session.ID, session.User, tx, task, n+1, &dictionary, db.Provider)
				rowsAffected += count

			case insertOpcode:
				httpStatus, operationErr = doInsert(session.ID, session.User, db, tx, task, n+1, &dictionary)
				rowsAffected++

			case dropOpCode:
				httpStatus, operationErr = doDrop(session.ID, session.User, db.Handle, task, n+1, &dictionary)
			}

			// See if there are any error triggers we need to look at, assuming what
			// has already been done was successful.
			if operationErr == nil && task.Errors != nil {
				for errorNumber, errorCondition := range task.Errors {
					// Evaluation the condition. Skip if it it's empty.
					if strings.TrimSpace(errorCondition.Condition) == "" {
						continue
					}

					// Convert from filter syntax to Ego syntax.
					condition, err := parsing.FormCondition(errorCondition.Condition)
					if err != nil {
						_ = tx.Rollback()

						return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
					}

					// Build a temporary symbol table for the expression evaluator. Fill it with the symbols
					// being managed for this transaction.
					evalSymbols := symbols.NewRootSymbolTable("transaction task condition")

					for k, v := range dictionary.symbols {
						evalSymbols.SetAlways(k, v)
					}

					evalSymbols.SetAlways("_rows_", count)
					evalSymbols.SetAlways("_all_rows_", rowsAffected)

					result, err := expressions.New().WithText(condition).Eval(evalSymbols)
					if err != nil {
						msg := fmt.Sprintf("Invalid error condition in task %d, %v", n+1, err.Error())

						return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
					}

					if data.BoolOrFalse(result) {
						_ = tx.Rollback()

						ui.Log(ui.TableLogger, "table.tx.rollback", ui.A{
							"session": session.ID,
							"count":   n + 1})

						msg := fmt.Sprintf("Error condition %d aborts transaction at operation %d", errorNumber+1, n+1)
						httpStatus = http.StatusInternalServerError

						if errorCondition.Status > 0 {
							httpStatus = errorCondition.Status
						}

						if errorCondition.Message != "" {
							msg = errorCondition.Message
						}

						return util.ErrorResponse(w, session.ID, msg, httpStatus)
					}
				}
			}

			// After we've processed any error triggers that might change the
			// value, if we still have an error let's roll back our work and
			// report the error.
			if operationErr != nil {
				_ = tx.Rollback()

				msg := fmt.Sprintf("transaction rollback at operation %d; %s", n+1, operationErr.Error())

				return util.ErrorResponse(w, session.ID, msg, httpStatus)
			}
		}

		// No errors so far, let's commit the script as a transaction. If this fails,
		// then we bail out with an error.
		if err = tx.Commit(); err != nil {
			return util.ErrorResponse(w, session.ID, "transaction commit error; "+err.Error(), httpStatus)
		}

		// Was there a result set in the symbol table? If so, we're returning
		// a rowset type.
		if result, ok := dictionary.symbols[resultSetSymbolName]; ok {
			if rows, ok := result.([]map[string]interface{}); ok {
				r := defs.DBRowSet{
					ServerInfo: util.MakeServerInfo(session.ID),
					Rows:       rows,
					Count:      len(rows),
					Status:     http.StatusOK,
				}

				b, _ := json.MarshalIndent(r, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

				ui.Log(ui.TableLogger, "table.tx.done", ui.A{
					"session":    session.ID,
					"operations": len(tasks),
					"affected":   rowsAffected,
					"rows":       len(rows)})

				w.Header().Add(defs.ContentTypeHeader, defs.RowSetMediaType)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(b)
				session.ResponseLength += len(b)

				if ui.IsActive(ui.RestLogger) {
					ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
						"session": session.ID,
						"body":    string(b)})
				}

				return http.StatusOK
			}
		}

		// Otherwise, we're returning a row count object, based on the
		// cumulative number of rows affected by the script.
		r := defs.DBRowCount{
			ServerInfo: util.MakeServerInfo(session.ID),
			Count:      rowsAffected,
			Status:     http.StatusOK,
		}

		b, _ := json.MarshalIndent(r, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		ui.Log(ui.TableLogger, "table.tx.affected", ui.A{
			"session":    session.ID,
			"operations": len(tasks),
			"affected":   rowsAffected})

		w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
		session.ResponseLength += len(b)

		if ui.IsActive(ui.RestLogger) {
			ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
				"session": session.ID,
				"body":    string(b)})
		}
	}

	return http.StatusOK
}
