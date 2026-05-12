package scripting

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/expressions"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Handler executes a sequence of SQL operations as a single atomic database
// transaction. It is the HTTP handler for the tables-service transaction endpoint.
//
// The request body must be a JSON array of defs.TXOperation objects. Each object
// specifies one operation to perform (insert, update, delete, select, etc.). All
// operations share a per-request symbol table that allows earlier operations to
// pass results to later ones via {{name}} substitution.
//
// If any operation fails, or if any user-defined error condition evaluates to true,
// the entire transaction is rolled back and an error response is returned.
// On success, the response is either a row-set (if a "readrows" operation populated
// the result-set symbol) or a simple row-count.
func Handler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var needCacheFlush bool

	// Decode the JSON array of operations from the request body.
	tasks := []defs.TXOperation{}

	e := json.NewDecoder(r.Body).Decode(&tasks)
	if e != nil {
		return util.ErrorResponse(w, session.ID, "transaction request decode error; "+e.Error(), http.StatusBadRequest)
	}

	ui.Log(ui.TableLogger, "table.tx.count", ui.A{
		"session": session.ID,
		"count":   len(tasks)})

	// An empty task list is legal but there is nothing to do.
	if len(tasks) == 0 {
		text := i18n.T("msg.table.tx.empty")

		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(text))
		session.ResponseLength += len(text)

		return http.StatusOK
	}

	// First pass: validate all opcodes before touching the database.
	// This avoids starting a transaction that we know will fail immediately.
	for n, task := range tasks {
		// If the opcode is missing but a raw SQL string is present, treat it
		// as a "sql" operation — a convenience for callers that just want to
		// send a bare SQL statement.
		if task.Opcode == "" && task.SQL != "" {
			tasks[n].Opcode = sqlOpcode
			task = tasks[n]
		}

		// Reject any unrecognized opcode before we open the database.
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

	// Open the database connection (read+write access required) and begin
	// a transaction so that all operations below are atomic.
	rowsAffected := 0
	httpStatus := http.StatusOK
	dictionary := symbolTable{symbols: map[string]any{}}

	db, err := database.Open(session, data.String(session.URLParts["dsn"]), dsns.DSNWriteAction+dsns.DSNReadAction)
	if err == nil && db != nil {
		defer db.Close()

		err = db.Begin()
		if err != nil {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
		}

		// Second pass: execute each operation in order.
		for n, task := range tasks {
			var operationErr error

			count := 0

			tableName := ""
			if task.Table != "" {
				tableName, _ = parsing.FullName(db.Provider, session.User, task.Table)
			}

			// Log which operation we are about to run (if table logging is active).
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

			// A "sql" opcode whose SQL text starts with SELECT is really a
			// "readrows" operation — promote it so the correct handler runs.
			if strings.EqualFold(task.Opcode, sqlOpcode) {
				if strings.HasPrefix(strings.TrimSpace(strings.ToLower(task.SQL)), "select ") {
					task.Opcode = rowsOpcode
				}
			}

			var cacheFlush bool

			// Dispatch to the operation-specific handler. Each handler:
			//   1. Expands {{symbol}} references in the task fields (applySymbolsToTask).
			//   2. Validates the task fields for that opcode.
			//   3. Builds and executes the SQL query.
			//   4. Returns (rowCount, httpStatus, error).
			switch strings.ToLower(task.Opcode) {
			case sqlOpcode:
				// Arbitrary non-SELECT SQL. Returns the affected-row count.
				count, httpStatus, cacheFlush, operationErr = doSQL(session.ID, db, task, n+1, &dictionary)
				if operationErr == nil {
					rowsAffected += count

					if cacheFlush {
						needCacheFlush = true
					}
				}

			case symbolsOpcode:
				// Load the task's Data map into the per-transaction symbol table.
				// No database access.
				httpStatus, operationErr = doSymbols(session.ID, task, n+1, &dictionary)

			case selectOpcode:
				// SELECT that reads exactly one row and stores each column as a
				// symbol (e.g. syms["age"] = 42). Used to feed values into later
				// operations.
				count, httpStatus, operationErr = doSelect(session.ID, session.User, db, task, n+1, &dictionary)
				rowsAffected += count

			case rowsOpcode:
				// SELECT that reads all matching rows and stores the entire result
				// as a single symbol (resultSetSymbolName). When the transaction
				// commits, this symbol becomes the HTTP response body.
				count, httpStatus, operationErr = doRows(session.ID, session.User, db, task, n+1, &dictionary)
				rowsAffected += count

			case updateOpcode:
				count, httpStatus, operationErr = doUpdate(session.ID, session.User, db, task, n+1, &dictionary)
				rowsAffected += count

			case deleteOpcode:
				count, httpStatus, operationErr = doDelete(session.ID, session.User, db, task, n+1, &dictionary)
				rowsAffected += count

			case insertOpcode:
				// Insert always affects exactly one row, so we increment by 1
				// rather than using the return value.
				httpStatus, operationErr = doInsert(session.ID, session.User, db, task, n+1, &dictionary)
				count = 1
				rowsAffected++

			case dropOpCode:
				httpStatus, operationErr = doDrop(session.ID, session.User, db, task, n+1, &dictionary)
				if operationErr == nil {
					needCacheFlush = true
				}
			}

			// Check user-defined error conditions (task.Errors). Each condition is
			// an Ego expression that is evaluated against the current symbol table.
			// If any condition is true, the transaction is rolled back and the
			// caller receives the associated status code and message.
			// This block only runs when the operation itself succeeded (operationErr == nil).
			if operationErr == nil && task.Errors != nil {
				for errorNumber, errorCondition := range task.Errors {
					// An empty condition string means "always trigger", so skip it
					// to avoid unintentional rollbacks.
					if strings.TrimSpace(errorCondition.Condition) == "" {
						continue
					}

					// Convert the filter-style condition to an Ego expression string
					// (e.g. "EQ(count,0)" becomes "count == 0").
					condition, err := parsing.FormCondition(errorCondition.Condition)
					if err != nil {
						_ = db.Rollback()

						return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
					}

					// Build a temporary Ego symbol table for the expression evaluator,
					// seeded with the current transaction symbols plus two synthetic
					// variables:
					//   _rows_     – number of rows affected by this operation
					//   _all_rows_ – cumulative rows affected so far in the transaction
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

					// If the condition evaluated to true, roll back and report.
					if data.BoolOrFalse(result) {
						_ = db.Rollback()

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

			// If the operation itself returned an error (after error-condition
			// processing), roll back everything and report the failure.
			if operationErr != nil {
				_ = db.Rollback()

				msg := fmt.Sprintf("transaction rollback at operation %d; %s", n+1, operationErr.Error())

				return util.ErrorResponse(w, session.ID, msg, httpStatus)
			}
		}

		// All operations succeeded — commit the transaction.
		if err = db.Commit(); err != nil {
			return util.ErrorResponse(w, session.ID, "transaction commit error; "+err.Error(), httpStatus)
		}

		// If the commit succeeded and one or more SQL operations altered the table metadata, flush the
		// cached table schema info so it can be reloaded from the database with updated information.
		if needCacheFlush {
			caches.Purge(caches.SchemaCache)
		}

		// Determine the response type. If a "readrows" operation stored a result
		// set, return it as a DBRowSet (Content-Type: rowset). Otherwise return a
		// plain DBRowCount with the cumulative affected-row count.
		if result, ok := dictionary.symbols[resultSetSymbolName]; ok {
			if rows, ok := result.([]map[string]any); ok {
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

		// No result set — return the cumulative row count.
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
