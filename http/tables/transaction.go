package tables

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/expressions"
	syms "github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

type TXError struct {
	Condition string `json:"condition"`
	Status    int    `json:"status"`
	Message   string `json:"msg"`
}

// This defines a single operation performed as part of a transaction.
type TxOperation struct {
	Opcode     string                 `json:"operation"`
	Table      string                 `json:"table,omitempty"`
	Filters    []string               `json:"filters,omitempty"`
	Columns    []string               `json:"columns,omitempty"`
	EmptyError bool                   `json:"emptyError,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Errors     []TXError              `json:"errors,omitempty"`
	SQL        string                 `json:"sql,omitempty"`
}

type symbolTable struct {
	symbols map[string]interface{}
}

const (
	symbolsOpcode = "symbols"
	insertOpcode  = "insert"
	deleteOpcode  = "delete"
	updateOpcode  = "update"
	dropOpCode    = "drop"
	selectOpcode  = "select"
	rowsOpcode    = "readrows"
	sqlOpcode     = "sql"

	resultSetSymbolName = "$$RESULT$$SET$$"
)

// DeleteRows deletes rows from a table. If no filter is provided, then all rows are
// deleted and the tale is empty. If filter(s) are applied, only the matching rows
// are deleted. The function returns the number of rows deleted.
func Transaction(user string, isAdmin bool, sessionID int32, w http.ResponseWriter, r *http.Request) {
	if err := util.AcceptedMediaType(r, []string{defs.RowCountMediaType}); err != nil {
		util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)

		return
	}

	// Verify that the parameters are valid, if given.
	if err := util.ValidateParameters(r.URL, map[string]string{
		defs.FilterParameterName: defs.Any,
		defs.UserParameterName:   datatypes.StringTypeName,
	}); err != nil {
		util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)

		return
	}

	// Validate the transaction payload.
	tasks := []TxOperation{}

	e := json.NewDecoder(r.Body).Decode(&tasks)
	if e != nil {
		util.ErrorResponse(w, sessionID, "transaction request decode error; "+e.Error(), http.StatusBadRequest)

		return
	}

	ui.Debug(ui.ServerLogger, "[%d] Transaction request with %d operations", sessionID, len(tasks))

	if p := parameterString(r); p != "" {
		ui.Debug(ui.ServerLogger, "[%d] request parameters:  %s", sessionID, p)
	}

	if len(tasks) == 0 {
		ui.Debug(ui.ServerLogger, "[%d] no tasks in transaction", sessionID)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("no tasks in transaction"))

		return
	}

	for n, task := range tasks {
		// Is the opcode missing, but there's a SQL string? If so, assume a "sql" operation
		if task.Opcode == "" && task.SQL != "" {
			tasks[n].Opcode = "sql"
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
			util.ErrorResponse(w, sessionID, msg, http.StatusBadRequest)

			return
		}
	}

	// Access the database and execute the transaction operations
	rowsAffected := 0
	httpStatus := http.StatusOK
	symbols := symbolTable{symbols: map[string]interface{}{}}

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {
		defer db.Close()

		tx, err := db.Begin()
		if err != nil {
			util.ErrorResponse(w, sessionID, "unable to start transaction; "+err.Error(), http.StatusInternalServerError)

			return
		}

		for n, task := range tasks {
			var operationErr error

			count := 0
			tableName, _ := fullName(user, task.Table)

			if ui.IsActive(ui.TableLogger) {
				if util.InList(strings.ToLower(task.Opcode), symbolsOpcode, sqlOpcode, rowsOpcode) {
					ui.Debug(ui.TableLogger, "[%d] Operation %s", sessionID, strings.ToUpper(task.Opcode))
				} else {
					ui.Debug(ui.TableLogger, "[%d] Operation %s on table %s", sessionID, strings.ToUpper(task.Opcode), tableName)
				}
			}

			// IF this is a SQL opcode that is a select operation, convert it to
			// a rows opcode
			if strings.EqualFold(task.Opcode, sqlOpcode) {
				if strings.HasPrefix(strings.TrimSpace(strings.ToLower(task.SQL)), "select ") {
					task.Opcode = rowsOpcode
				}
			}

			switch strings.ToLower(task.Opcode) {
			case sqlOpcode:
				count, httpStatus, operationErr = txSQL(sessionID, user, tx, task, n+1, &symbols)
				rowsAffected += count

			case symbolsOpcode:
				httpStatus, operationErr = txSymbols(sessionID, task, n+1, &symbols)

			case selectOpcode:
				count, httpStatus, operationErr = txSelect(sessionID, user, db, tx, task, n+1, &symbols)
				rowsAffected += count

			case rowsOpcode:
				count, httpStatus, operationErr = txRows(sessionID, user, db, tx, task, n+1, &symbols)
				rowsAffected += count

			case updateOpcode:
				count, httpStatus, operationErr = txUpdate(sessionID, user, db, tx, task, n+1, &symbols)
				rowsAffected += count

			case deleteOpcode:
				count, httpStatus, operationErr = txDelete(sessionID, user, tx, task, n+1, &symbols)
				rowsAffected += count

			case insertOpcode:
				httpStatus, operationErr = txInsert(sessionID, user, db, tx, task, n+1, &symbols)
				rowsAffected++

			case dropOpCode:
				httpStatus, operationErr = txDrop(sessionID, user, db, task, n+1, &symbols)
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
					condition := formCondition(errorCondition.Condition)

					ui.Debug(ui.TableLogger, "[%d] Evaluate error condition: %s", sessionID, condition)

					// Build a temporary symbol table for the expression evaluator. Fill it with the symbols
					// being managed for this transaction.
					evalSymbols := syms.NewRootSymbolTable("transaction task condition")

					for k, v := range symbols.symbols {
						evalSymbols.SetAlways(k, v)
					}

					evalSymbols.SetAlways("_rows_", count)
					evalSymbols.SetAlways("_all_rows_", rowsAffected)

					result, err := expressions.New().WithText(condition).Eval(evalSymbols)
					if err != nil {
						msg := fmt.Sprintf("Invalid error condition in task %d, %v", n+1, err.Error())

						util.ErrorResponse(w, sessionID, msg, http.StatusBadRequest)

						return
					}

					if datatypes.Bool(result) {
						_ = tx.Rollback()

						ui.Debug(ui.TableLogger, "[%d] Transaction rolled back at task %d", sessionID, n+1)

						msg := fmt.Sprintf("Error condition %d aborts transaction at operation %d", errorNumber+1, n+1)
						httpStatus = http.StatusInternalServerError

						if errorCondition.Status > 0 {
							httpStatus = errorCondition.Status
						}

						if errorCondition.Message != "" {
							msg = errorCondition.Message
						}

						util.ErrorResponse(w, sessionID, msg, httpStatus)

						return
					}
				}
			}

			if operationErr != nil {
				_ = tx.Rollback()

				msg := fmt.Sprintf("transaction rollback at operation %d; %s", n+1, operationErr.Error())

				util.ErrorResponse(w, sessionID, msg, httpStatus)

				return
			}
		}

		err = tx.Commit()
		if err != nil {
			util.ErrorResponse(w, sessionID, "transaction commit error; "+err.Error(), httpStatus)

			return
		}

		// Was there a result set in the symbol table? If so, we're returning
		// a rowset type.
		if result, ok := symbols.symbols[resultSetSymbolName]; ok {
			if rows, ok := result.([]map[string]interface{}); ok {
				r := defs.DBRowSet{
					ServerInfo: util.MakeServerInfo(sessionID),
					Rows:       rows,
					Count:      len(rows),
				}

				b, _ := json.MarshalIndent(r, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

				ui.Debug(ui.TableLogger, "[%d] %s",
					sessionID,
					fmt.Sprintf("completed %d operations in transaction, updated and/or read %d rows, returning %d rows",
						len(tasks), rowsAffected, len(rows)))

				w.Header().Add("Content-Type", defs.RowSetMediaType)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(b)

				return
			}
		}

		r := defs.DBRowCount{
			ServerInfo: util.MakeServerInfo(sessionID),
			Count:      rowsAffected,
		}

		b, _ := json.MarshalIndent(r, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		ui.Debug(ui.TableLogger, "[%d] %s",
			sessionID,
			fmt.Sprintf("completed %d operations in transaction, updated %d rows", len(tasks), rowsAffected))

		w.Header().Add("Content-Type", defs.RowCountMediaType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)

		return
	}
}

// Add all the items in the "data" dictionary to the symbol table, which is initialized if needed.
func txSymbols(sessionID int32, task TxOperation, id int, symbols *symbolTable) (int, error) {
	if err := applySymbolsToTask(sessionID, &task, id, symbols); err != nil {
		return http.StatusBadRequest, errors.EgoError(err)
	}

	if len(task.Filters) > 0 {
		return http.StatusBadRequest, errors.NewMessage("filters not supported for SYMBOLS task")
	}

	if len(task.Columns) > 0 {
		return http.StatusBadRequest, errors.NewMessage("columns not supported for SYMBOLS task")
	}

	if task.Table != "" {
		return http.StatusBadRequest, errors.NewMessage("table name not supported for SYMBOLS task")
	}

	msg := strings.Builder{}

	for key, value := range task.Data {
		if symbols.symbols == nil {
			symbols.symbols = map[string]interface{}{}
		}

		symbols.symbols[key] = value

		msg.WriteString(key)
		msg.WriteString(": ")
		msg.WriteString(datatypes.String(value))
	}

	ui.Debug(ui.TableLogger, "[%d] Defined new symbols; %s", sessionID, msg.String())

	return http.StatusOK, nil
}

func txRows(sessionID int32, user string, db *sql.DB, tx *sql.Tx, task TxOperation, id int, syms *symbolTable) (int, int, error) {
	var err error

	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return 0, http.StatusBadRequest, err
	}

	tableName, _ := fullName(user, task.Table)
	count := 0

	fakeURL, _ := url.Parse("http://localhost:8080/tables/" + task.Table + "/rows?limit=1")

	q := task.SQL
	if q == "" {
		q = formSelectorDeleteQuery(fakeURL, task.Filters, strings.Join(task.Columns, ","), tableName, user, selectVerb)
		if p := strings.Index(q, syntaxErrorPrefix); p >= 0 {
			return count, http.StatusBadRequest, errors.NewMessage(filterErrorMessage(q))
		}
	}

	ui.Debug(ui.SQLLogger, "[%d] Query: %s", sessionID, q)

	var status int

	count, status, err = readTxRowResultSet(db, tx, q, sessionID, syms, task.EmptyError)
	if err == nil {
		return count, status, nil
	}

	ui.Debug(ui.TableLogger, "[%d] Error reading table, %v", sessionID, err)

	return 0, status, errors.EgoError(err)
}

func readTxRowResultSet(db *sql.DB, tx *sql.Tx, q string, sessionID int32, syms *symbolTable, emptyResultError bool) (int, int, error) {
	// If the symbol table doesn't exist, create it. If it does, delete any
	// previous result set (to quote the Highlander, "there can be only one.")
	if syms == nil || len(syms.symbols) == 0 {
		*syms = symbolTable{symbols: map[string]interface{}{}}
	} else {
		delete(syms.symbols, resultSetSymbolName)
	}

	var rows *sql.Rows

	var err error

	result := []map[string]interface{}{}
	rowCount := 0
	status := http.StatusOK

	rows, err = tx.Query(q)
	if err == nil {
		defer rows.Close()

		columnNames, _ := rows.Columns()
		columnCount := len(columnNames)

		for rows.Next() {
			row := make([]interface{}, columnCount)
			rowptrs := make([]interface{}, columnCount)

			for i := range row {
				rowptrs[i] = &row[i]
			}

			err = rows.Scan(rowptrs...)
			if err == nil {
				newRow := map[string]interface{}{}
				for i, v := range row {
					newRow[columnNames[i]] = v
				}

				result = append(result, newRow)
				rowCount++
			}
		}

		syms.symbols[resultSetSymbolName] = result

		ui.Debug(ui.TableLogger, "[%d] Read %d rows of %d columns; %d", sessionID, rowCount, columnCount, status)
	} else {
		status = http.StatusBadRequest
	}

	if err != nil {
		err = errors.EgoError(err)
	}

	return rowCount, status, err
}

func txSelect(sessionID int32, user string, db *sql.DB, tx *sql.Tx, task TxOperation, id int, syms *symbolTable) (int, int, error) {
	var err error

	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return 0, http.StatusBadRequest, errors.EgoError(err)
	}

	tableName, _ := fullName(user, task.Table)
	count := 0

	fakeURL, _ := url.Parse("http://localhost:8080/tables/" + task.Table + "/rows?limit=1")

	q := formSelectorDeleteQuery(fakeURL, task.Filters, strings.Join(task.Columns, ","), tableName, user, selectVerb)
	if p := strings.Index(q, syntaxErrorPrefix); p >= 0 {
		return count, http.StatusBadRequest, errors.NewMessage(filterErrorMessage(q))
	}

	ui.Debug(ui.SQLLogger, "[%d] Query: %s", sessionID, q)

	var status int

	count, status, err = readTxRowData(db, tx, q, sessionID, syms, task.EmptyError)
	if err == nil {
		return count, status, nil
	}

	ui.Debug(ui.TableLogger, "[%d] Error reading table, %v", sessionID, err)

	return 0, status, errors.EgoError(err)
}

func readTxRowData(db *sql.DB, tx *sql.Tx, q string, sessionID int32, syms *symbolTable, emptyResultError bool) (int, int, error) {
	var rows *sql.Rows

	var err error

	rowCount := 0
	status := http.StatusOK

	if syms == nil || len(syms.symbols) == 0 {
		*syms = symbolTable{symbols: map[string]interface{}{}}
	}

	rows, err = db.Query(q)
	if err == nil {
		defer rows.Close()

		columnNames, _ := rows.Columns()
		columnCount := len(columnNames)

		for rows.Next() {
			row := make([]interface{}, columnCount)
			rowptrs := make([]interface{}, columnCount)

			for i := range row {
				rowptrs[i] = &row[i]
			}

			// Get the next row values. Note we only incorporate them into the symbol
			// table on the first row (rowCount of zero), the rest are ignored. An error
			// will be thrown later.
			err = rows.Scan(rowptrs...)
			if err == nil && rowCount == 0 {
				msg := strings.Builder{}

				for i, v := range row {
					syms.symbols[columnNames[i]] = v

					if msg.Len() > 0 {
						msg.WriteString(", ")
					}

					msg.WriteString(columnNames[i])
					msg.WriteString("=")
					msg.WriteString(datatypes.String(v))
				}

				ui.Debug(ui.TableLogger, "[%d] Read table to set symbols: %s", sessionID, msg.String())

				rowCount++
			}
		}

		if rowCount == 0 && emptyResultError {
			status = http.StatusNotFound
			err = errors.NewMessage("SELECT task did not return any row data")
		} else if rowCount > 1 {
			err = errors.NewMessage("SELECT task did not return a unique row")
			status = http.StatusBadRequest

			ui.Debug(ui.TableLogger, "[%d] Invalid read of %d rows ", sessionID, rowCount)
		} else {
			ui.Debug(ui.TableLogger, "[%d] Read %d rows of %d columns", sessionID, rowCount, columnCount)
		}
	} else {
		status = http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			status = http.StatusNotFound
		}
	}

	if err != nil {
		err = errors.EgoError(err)
	}

	return rowCount, status, err
}

func txUpdate(sessionID int32, user string, db *sql.DB, tx *sql.Tx, task TxOperation, id int, syms *symbolTable) (int, int, error) {
	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return 0, http.StatusBadRequest, errors.EgoError(err)
	}

	tableName, _ := fullName(user, task.Table)

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
	if filter := whereClause(task.Filters); filter != "" {
		if p := strings.Index(filter, syntaxErrorPrefix); p >= 0 {
			return 0, http.StatusBadRequest, errors.NewMessage(filterErrorMessage(filter))
		}

		result.WriteString(filter)
	} else if settings.GetBool(defs.TablesServerEmptyFilterError) {
		return 0, http.StatusBadRequest, errors.NewMessage("update without filter is not allowed")
	}

	ui.Debug(ui.SQLLogger, "[%d] Exec: %s", sessionID, result.String())

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
		updateErr = errors.EgoError(updateErr)
		status = http.StatusBadRequest
		if strings.Contains(updateErr.Error(), "constraint") {
			status = http.StatusConflict
		}
	}

	return int(count), status, updateErr
}

func txDelete(sessionID int32, user string, tx *sql.Tx, task TxOperation, id int, syms *symbolTable) (int, int, error) {
	if e := applySymbolsToTask(sessionID, &task, id, syms); e != nil {
		return 0, http.StatusBadRequest, errors.EgoError(e)
	}

	tableName, _ := fullName(user, task.Table)

	if len(task.Columns) > 0 {
		return 0, http.StatusBadRequest, errors.NewMessage("columns not supported for DELETE task")
	}

	if where := whereClause(task.Filters); where == "" {
		if settings.GetBool(defs.TablesServerEmptyFilterError) {
			return 0, http.StatusBadRequest, errors.NewMessage("operation invalid with empty filter")
		}
	}

	fakeURL, _ := url.Parse(fmt.Sprintf("http://localhost:8080/tables/%s/rows", task.Table))

	q := formSelectorDeleteQuery(fakeURL, task.Filters, "", tableName, user, deleteVerb)
	if p := strings.Index(q, syntaxErrorPrefix); p >= 0 {
		return 0, http.StatusBadRequest, errors.NewMessage(filterErrorMessage(q))
	}

	ui.Debug(ui.SQLLogger, "[%d] Exec: %s", sessionID, q)

	rows, err := tx.Exec(q)
	if err == nil {
		count, _ := rows.RowsAffected()

		if count == 0 && task.EmptyError {
			return 0, http.StatusNotFound, errors.NewMessage("delete did not modify any rows")
		}

		ui.Debug(ui.TableLogger, "[%d] Deleted %d rows; %d", sessionID, count, 200)

		return int(count), http.StatusOK, nil
	}

	return 0, http.StatusBadRequest, errors.EgoError(err)
}

func txSQL(sessionID int32, user string, tx *sql.Tx, task TxOperation, id int, syms *symbolTable) (int, int, error) {
	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return 0, http.StatusBadRequest, errors.EgoError(err)
	}

	if len(task.Columns) > 0 {
		return 0, http.StatusBadRequest, errors.NewMessage("columns not supported for SQL task")
	}

	if len(task.Filters) > 0 {
		return 0, http.StatusBadRequest, errors.NewMessage("filters not supported for SQL task")
	}

	if len(strings.TrimSpace(task.SQL)) == 0 {
		return 0, http.StatusBadRequest, errors.NewMessage("missing SQL command for SQL task")
	}

	if len(strings.TrimSpace(task.Table)) != 0 {
		return 0, http.StatusBadRequest, errors.NewMessage("table name not supported for SQL task")
	}

	q := task.SQL

	ui.Debug(ui.SQLLogger, "[%d] Exec: %s", sessionID, q)

	rows, err := tx.Exec(q)
	if err == nil {
		count, _ := rows.RowsAffected()

		if count == 0 && task.EmptyError {
			return 0, http.StatusNotFound, errors.NewMessage("sql did not modify any rows")
		}

		ui.Debug(ui.TableLogger, "[%d] Affeccted %d rows; %d", sessionID, count, 200)

		return int(count), http.StatusOK, nil
	}

	return 0, http.StatusBadRequest, errors.EgoError(err)
}

func txDrop(sessionID int32, user string, db *sql.DB, task TxOperation, id int, syms *symbolTable) (int, error) {
	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return http.StatusBadRequest, errors.EgoError(err)
	}

	table, _ := fullName(user, task.Table)

	if len(task.Filters) > 0 {
		return http.StatusBadRequest, errors.NewMessage("filters not supported for DROP task")
	}

	if len(task.Columns) > 0 {
		return http.StatusBadRequest, errors.NewMessage("columns not supported for DROP task")
	}

	q := "DROP TABLE ?"
	_, err := db.Exec(q, table)

	status := http.StatusOK
	if err != nil {
		status = http.StatusInternalServerError

		if strings.Contains(err.Error(), "no such") || strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}

		err = errors.EgoError(err)
	}

	return status, err
}

func txInsert(sessionID int32, user string, db *sql.DB, tx *sql.Tx, task TxOperation, id int, syms *symbolTable) (int, error) {
	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return http.StatusBadRequest, errors.EgoError(err)
	}

	if len(task.Filters) > 0 {
		return http.StatusBadRequest, errors.NewMessage("filters not supported for INSERT task")
	}

	if len(task.Columns) > 0 {
		return http.StatusBadRequest, errors.NewMessage("columns not supported for INSERT task")
	}

	// Get the column metadata for the table we're insert into, so we can validate column info.
	tableName, _ := fullName(user, task.Table)

	columns, err := getColumnInfo(db, user, tableName, sessionID)
	if err != nil {
		return http.StatusBadRequest, errors.NewMessage("unable to read table metadata; " + err.Error())
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

			return http.StatusBadRequest, errors.NewMessage(msg)
		}

		// If it's one of the date/time values, make sure it is wrapped in single qutoes.
		if keywordMatch(column.Type, "time", "date", "timestamp") {
			text := strings.TrimPrefix(strings.TrimSuffix(datatypes.String(v), "\""), "\"")
			task.Data[column.Name] = "'" + strings.TrimPrefix(strings.TrimSuffix(text, "'"), "'") + "'"
			ui.Debug(ui.TableLogger, "[%d] Updated column %s value from %v to %v", sessionID, column.Name, v, task.Data[column.Name])
		}
	}

	q, values := formInsertQuery(task.Table, user, task.Data)
	ui.Debug(ui.TableLogger, "[%d] Exec: %s", sessionID, q)

	_, e := tx.Exec(q, values...)
	if e != nil {
		status := http.StatusBadRequest
		if strings.Contains(e.Error(), "constraint") {
			status = http.StatusConflict
		}

		return status, errors.NewMessage("error inserting row; " + e.Error())
	}

	ui.Debug(ui.TableLogger, "[%d] Successful INSERT to %s", sessionID, tableName)

	return http.StatusOK, nil
}
