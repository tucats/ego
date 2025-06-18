package tables

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	data "github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
	"github.com/tucats/ego/util"
)

const insertErrorPrefix = "insert error: "

// DeleteRows deletes rows from a table. If no filter is provided, then all rows are
// deleted and the tale is empty. If filter(s) are applied, only the matching rows
// are deleted. The function returns the number of rows deleted.
func DeleteRows(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	tableName := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])

	db, err := database.Open(&session.User, dsnName, dsns.DSNWriteAction)
	if err == nil && db != nil {
		defer db.Close()

		// If we're not using sqlite for this connection, amend any table name
		// with the user schema name.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(session.User, tableName)
		}

		if !session.Admin && dsnName == "" && !Authorized(session.ID, db.Handle, session.User, tableName, deleteOperation) {
			return util.ErrorResponse(w, session.ID, "User does not have delete permission", http.StatusForbidden)
		}

		if where, err := parsing.WhereClause(parsing.FiltersFromURL(r.URL)); where == "" {
			if settings.GetBool(defs.TablesServerEmptyFilterError) {
				return util.ErrorResponse(w, session.ID, "operation invalid with empty filter", http.StatusBadRequest)
			}
		} else if err != nil {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}

		columns := parsing.ColumnsFromURL(r.URL)
		filters := parsing.FiltersFromURL(r.URL)

		q, err := parsing.FormSelectorDeleteQuery(r.URL, filters, columns, tableName, session.User, deleteVerb, db.Provider)
		if err != nil {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}

		if p := strings.Index(q, syntaxErrorPrefix); p >= 0 {
			return util.ErrorResponse(w, session.ID, filterErrorMessage(q), http.StatusBadRequest)
		}

		ui.Log(ui.SQLLogger, "sq.exec", ui.A{
			"session": session.ID,
			"sql":     q})

		rows, err := db.Exec(q)
		if err == nil {
			rowCount, _ := rows.RowsAffected()

			if rowCount == 0 && settings.GetBool(defs.TablesServerEmptyRowsetError) {
				return util.ErrorResponse(w, session.ID, "no matching rows found", http.StatusNotFound)
			}

			resp := defs.DBRowCount{
				ServerInfo: util.MakeServerInfo(session.ID),
				Count:      int(rowCount),
				Status:     http.StatusOK,
			}

			w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

			b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
			_, _ = w.Write(b)
			session.ResponseLength += len(b)

			if ui.IsActive(ui.RestLogger) {
				ui.Log(ui.RestLogger, "rest.response.payload", ui.A{
					"session": session.ID,
					"body":    string(b)})
			}

			ui.Log(ui.TableLogger, "table.deleted.rows", ui.A{
				"session": session.ID,
				"count":   rowCount,
				"status":  resp.Status})

			return resp.Status
		}

		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	return http.StatusOK
}

// InsertRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func InsertRows(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var columns []defs.DBColumn

	tableName := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])

	if useAbstract(r) {
		return InsertAbstractRows(session.User, session.Admin, tableName, session, w, r)
	}

	db, err := database.Open(&session.User, dsnName, dsns.DSNWriteAction)
	if err == nil && db != nil && db.Handle != nil {
		defer db.Close()

		// If we're not using sqlite for this connection, amend any table name
		// with the user schema name.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(session.User, tableName)
		}

		if !session.Admin && dsnName == "" && !Authorized(session.ID, db.Handle, session.User, tableName, updateOperation) {
			return util.ErrorResponse(w, session.ID, "User does not have update permission", http.StatusForbidden)
		}

		// Get the column metadata for the table we're insert into, so we can validate column info.

		tableName, _ = parsing.FullName(session.User, tableName)

		columns, err = getColumnInfo(db, session.User, tableName, session.ID)
		if err != nil {
			return util.ErrorResponse(w, session.ID, "Unable to read table metadata, "+err.Error(), http.StatusBadRequest)
		}

		buf := new(strings.Builder)
		_, _ = io.Copy(buf, r.Body)
		rawPayload := buf.String()

		ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    rawPayload})

		// Lets get the rows we are to insert from the request payload.. This is either a rowset, an array of rows,
		// or a single row. In this case, a row is modeled as a map of column name to value.
		rowSet, httpStatus := getRowSet(rawPayload, session, w)
		if httpStatus > http.StatusOK {
			return httpStatus
		}

		// For any object in the payload, we must assign a UUID now. This overrides any previous
		// item in the set for _row_id_ or creates it if not found. Row IDs are always assigned
		// on input only. Note that the UUID is recoded as base-32 to make a shorter string value.
		for n := 0; n < len(rowSet.Rows); n++ {
			rowSet.Rows[n][defs.RowIDName] = egostrings.Gibberish(uuid.New())
		}

		// Start a transaction, and then lets loop over the rows in the rowset. Note this might
		// be just one row.
		tx, _ := db.Begin()
		count := 0

		count, httpStatus = insertRowSet(rowSet, columns, w, session, r, db, count, tx)
		if httpStatus > http.StatusOK {
			return httpStatus
		}

		if count == 0 && settings.GetBool(defs.TablesServerEmptyRowsetError) {
			return util.ErrorResponse(w, session.ID, "no matching rows found", http.StatusNotFound)
		}

		result := defs.DBRowCount{
			ServerInfo: util.MakeServerInfo(session.ID),
			Count:      count,
			Status:     http.StatusOK,
		}

		w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

		b, _ := json.MarshalIndent(result, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		_, _ = w.Write(b)
		session.ResponseLength += len(b)

		if ui.IsActive(ui.RestLogger) {
			ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
				"session": session.ID,
				"body":    string(b)})
		}

		err = tx.Commit()
		if err == nil {
			status := http.StatusOK
			ui.Log(ui.TableLogger, "table.inserted.rows", ui.A{
				"session": session.ID,
				"count":   count,
				"status":  status})

			return status
		}

		_ = tx.Rollback()

		return util.ErrorResponse(w, session.ID, insertErrorPrefix+err.Error(), http.StatusBadRequest)
	}

	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "no privilege") {
			status = http.StatusForbidden
		}

		return util.ErrorResponse(w, session.ID, insertErrorPrefix+err.Error(), status)
	}

	return http.StatusOK
}

// insertRowSet does the actual work of inserting the rows from the row set object into the database, and reporting any errors. The
// result is the count of rows inserted, and the HTTP status code if an error occurred.
func insertRowSet(rowSet defs.DBRowSet, columns []defs.DBColumn, w http.ResponseWriter, session *server.Session, r *http.Request, db *database.Database, count int, tx *sql.Tx) (int, int) {
	for _, row := range rowSet.Rows {
		for _, column := range columns {
			v, ok := row[column.Name]
			if !ok && settings.GetBool(defs.TableServerPartialInsertError) {
				expectedList := make([]string, 0)
				for _, k := range columns {
					expectedList = append(expectedList, k.Name)
				}

				providedList := make([]string, 0)
				for k := range row {
					providedList = append(providedList, k)
				}

				sort.Strings(expectedList)
				sort.Strings(providedList)

				msg := fmt.Sprintf("Payload did not include data for %s; expected %v but payload contained %v",
					strconv.Quote(column.Name), strings.Join(expectedList, ","), strings.Join(providedList, ","))

				return 0, util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
			}

			// If it's one of the date/time values, make sure it is wrapped in single quotes.
			if parsing.KeywordMatch(column.Type, "time", "date", "timestamp") {
				text := strings.TrimPrefix(strings.TrimSuffix(data.String(v), "\""), "\"")
				row[column.Name] = "'" + strings.TrimPrefix(strings.TrimSuffix(text, "'"), "'") + "'"
				ui.Log(ui.TableLogger, "table.update.column", ui.A{
					"session": session.ID,
					"column":  column.Name,
					"from":    v,
					"to":      row[column.Name]})
			}
		}

		tableName, e := parsing.TableNameFromRequest(r)
		if e != nil {
			return 0, util.ErrorResponse(w, session.ID, e.Error(), http.StatusBadRequest)
		}

		q, values, err := parsing.FormInsertQuery(tableName, session.User, db.Provider, columns, row)
		if err != nil {
			_ = tx.Rollback()

			return 0, util.ErrorResponse(w, session.ID, err.Error(), http.StatusConflict)
		}

		ui.Log(ui.SQLLogger, "sql.exec", ui.A{
			"session": session.ID,
			"sql":     q})

		_, err = db.Exec(q, values...)
		if err == nil {
			count++
		} else {
			_ = tx.Rollback()

			return 0, util.ErrorResponse(w, session.ID, err.Error(), http.StatusConflict)
		}
	}

	return count, http.StatusOK
}

// getRowSet extracts the row set from the raw payload that is to be applied to the database.
func getRowSet(rawPayload string, session *server.Session, w http.ResponseWriter) (defs.DBRowSet, int) {
	var err error

	rowSet := defs.DBRowSet{}

	err = json.Unmarshal([]byte(rawPayload), &rowSet)
	if err != nil || len(rowSet.Rows) == 0 {
		// Not a valid row set, but might be an array of items
		err = json.Unmarshal([]byte(rawPayload), &rowSet.Rows)
		if err == nil {
			rowSet.Count = len(rowSet.Rows)
		} else {
			// Not an array of rows, but might be a single item
			item := map[string]interface{}{}

			err = json.Unmarshal([]byte(rawPayload), &item)
			if err != nil {
				return defs.DBRowSet{}, util.ErrorResponse(w, session.ID, "Invalid INSERT payload: "+err.Error(), http.StatusBadRequest)
			} else {
				rowSet.Count = 1
				rowSet.Rows = make([]map[string]interface{}, 1)
				rowSet.Rows[0] = item
			}
		}
	}

	// If at this point we have an empty row set, then just bail out now. Return a success
	// status but an indicator that nothing was done.
	if len(rowSet.Rows) == 0 {
		return rowSet, util.ErrorResponse(w, session.ID, errors.ErrTableNoRows.Error(), http.StatusNoContent)
	}

	return rowSet, http.StatusOK
}

// ReadRows reads the data for a given table, and returns it as an array
// of structs for each row, with the struct tag being the column name. The
// query can also specify filter, sort, and column query parameters to refine
// the read operation.
func ReadRows(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	tableName := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])

	if useAbstract(r) {
		return ReadAbstractRows(session.User, session.Admin, tableName, session, w, r)
	}

	db, err := database.Open(&session.User, dsnName, dsns.DSNReadAction)
	if err == nil && db != nil {
		var queryText string

		defer db.Close()

		// If we're not using sqlite for this connection, amend any table name
		// with the user schema name.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(session.User, tableName)
		}

		if !session.Admin && dsnName == "" && !Authorized(session.ID, db.Handle, session.User, tableName, readOperation) {
			return util.ErrorResponse(w, session.ID, "User does not have read permission", http.StatusForbidden)
		}

		queryText, err = parsing.FormSelectorDeleteQuery(r.URL, parsing.FiltersFromURL(r.URL), parsing.ColumnsFromURL(r.URL), tableName, session.User, selectVerb, db.Provider)
		if err != nil {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}

		if p := strings.Index(queryText, syntaxErrorPrefix); p >= 0 {
			return util.ErrorResponse(w, session.ID, filterErrorMessage(queryText), http.StatusBadRequest)
		}

		ui.Log(ui.SQLLogger, "sql.query", ui.A{
			"session": session.ID,
			"sql":     queryText})

		if err = readRowData(db.Handle, queryText, session, w); err == nil {
			return http.StatusOK
		}
	}

	ui.Log(ui.TableLogger, "table.read.error", ui.A{
		"session": session.ID,
		"error":   err.Error()})

	return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
}

func readRowData(db *sql.DB, q string, session *server.Session, w http.ResponseWriter) error {
	var (
		rows     *sql.Rows
		err      error
		rowCount int
		result   = []map[string]interface{}{}
	)

	rows, err = db.Query(q)
	if err == nil {
		defer rows.Close()

		columnNames, _ := rows.Columns()
		columnCount := len(columnNames)

		for rows.Next() {
			row := make([]interface{}, columnCount)
			rowPointers := make([]interface{}, columnCount)

			for i := range row {
				rowPointers[i] = &row[i]
			}

			err = rows.Scan(rowPointers...)
			if err == nil {
				newRow := map[string]interface{}{}
				for i, v := range row {
					newRow[columnNames[i]] = v
				}

				result = append(result, newRow)
				rowCount++
			}
		}

		resp := defs.DBRowSet{
			ServerInfo: util.MakeServerInfo(session.ID),
			Rows:       result,
			Count:      len(result),
			Status:     http.StatusOK,
		}

		status := http.StatusOK

		w.Header().Add(defs.ContentTypeHeader, defs.RowSetMediaType)
		w.WriteHeader(status)

		b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		_, _ = w.Write(b)
		session.ResponseLength += len(b)

		ui.Log(ui.TableLogger, "table.read", ui.A{
			"session": session.ID,
			"rows":    rowCount,
			"columns": columnCount,
			"status":  status})

		if ui.IsActive(ui.RestLogger) {
			ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
				"session": session.ID,
				"body":    string(b)})
		}
	}

	return err
}

// UpdateRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func UpdateRows(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		db     *database.Database
		err    error
		rowSet defs.DBRowSet
	)

	tableName := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])
	count := 0

	if useAbstract(r) {
		return UpdateAbstractRows(session.User, session.Admin, tableName, session, w, r)
	}

	ui.Log(ui.TableLogger, "table.update.table", ui.A{
		"session": session.ID,
		"table":   tableName})

	if p := parameterString(r); p != "" {
		ui.Log(ui.TableLogger, "table.parms", ui.A{
			"session": session.ID,
			"params":  p})
	}

	db, err = database.Open(&session.User, dsnName, dsns.DSNWriteAction)
	if err == nil && db != nil {
		defer db.Close()

		// If we're not using sqlite for this connection, amend any table name
		// with the user schema name.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(session.User, tableName)
		}

		if !session.Admin && dsnName == "" && !Authorized(session.ID, db.Handle, session.User, tableName, updateOperation) {
			return util.ErrorResponse(w, session.ID, "User does not have update permission", http.StatusForbidden)
		}

		excludeList, httpStatus := getExcludeList(r, db, session, tableName, w)
		if httpStatus > http.StatusOK {
			return httpStatus
		}

		// Get the rowset specification from the payload for what is to be updated.
		rowSet, err, httpStatus = getUpdateRows(r, session, err, w, excludeList)
		if httpStatus > http.StatusOK {
			return httpStatus
		}

		// Start a transaction to ensure atomicity of the entire update
		tx, _ := db.Begin()

		// Loop over the row set doing the update

		count, httpStatus = updateRowSet(rowSet, excludeList, session, r, db, tx, w, count)
		if httpStatus > http.StatusOK {
			return httpStatus
		}

		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
	}

	if err == nil {
		if count == 0 && settings.GetBool(defs.TablesServerEmptyRowsetError) {
			return util.ErrorResponse(w, session.ID, "no matching rows found", http.StatusNotFound)
		}

		result := defs.DBRowCount{
			ServerInfo: util.MakeServerInfo(session.ID),
			Count:      count,
			Status:     http.StatusOK,
		}

		status := http.StatusOK

		w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)
		w.WriteHeader(status)

		b, _ := json.MarshalIndent(result, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		_, _ = w.Write(b)
		session.ResponseLength += len(b)

		if ui.IsActive(ui.RestLogger) {
			ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
				"session": session.ID,
				"body":    string(b)})
		}

		ui.Log(ui.TableLogger, "table.updated.rows", ui.A{
			"session": session.ID,
			"count":   count,
			"status":  status})
	} else {
		return util.ErrorResponse(w, session.ID, "Error updating table, "+err.Error(), http.StatusInternalServerError)
	}

	return http.StatusOK
}

func updateRowSet(rowSet defs.DBRowSet, excludeList map[string]bool, session *server.Session, r *http.Request, db *database.Database, tx *sql.Tx, w http.ResponseWriter, count int) (int, int) {
	for _, rowData := range rowSet.Rows {
		hasRowID := false

		if v, found := rowData[defs.RowIDName]; found {
			if data.String(v) != "" {
				hasRowID = true
			}
		}

		for key, excluded := range excludeList {
			if key == defs.RowIDName && hasRowID {
				continue
			}

			if excluded {
				delete(rowData, key)
			}
		}

		ui.Log(ui.TableLogger, "table.values", ui.A{
			"session": session.ID,
			"data":    rowData})

		q, values, err := parsing.FormUpdateQuery(r.URL, session.User, db.Provider, rowData)
		if err != nil {
			ui.Log(ui.SQLLogger, "sql.query.error", ui.A{
				"session": session.ID,
				"sql":     q,
				"error":   err.Error()})

			_ = tx.Rollback()

			return 0, util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}

		ui.Log(ui.SQLLogger, "sql.query", ui.A{
			"session": session.ID,
			"sql":     q})

		counts, err := db.Exec(q, values...)
		if err == nil {
			rowsAffected, _ := counts.RowsAffected()
			count = count + int(rowsAffected)
		} else {
			_ = tx.Rollback()

			return 0, util.ErrorResponse(w, session.ID, err.Error(), http.StatusConflict)
		}
	}

	return count, http.StatusOK
}

func getExcludeList(r *http.Request, db *database.Database, session *server.Session, tableName string, w http.ResponseWriter) (map[string]bool, int) {
	excludeList := map[string]bool{}

	p := r.URL.Query()
	if v, found := p[defs.ColumnParameterName]; found {
		// There is a column list, so build a list of all the columns, and then
		// remove the ones from the column parameter. This builds a list of columns
		// that are excluded.
		columns, err := getColumnInfo(db, session.User, tableName, session.ID)
		if err != nil {
			return nil, util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
		}

		for _, column := range columns {
			excludeList[column.Name] = true
		}

		for _, name := range v {
			nameParts := strings.Split(parsing.StripQuotes(name), ",")
			for _, part := range nameParts {
				if part != "" {
					// make sure the column name is actually valid. We assume the row ID name
					// is always valid.
					httpStatus := validateColumnName(part, columns, w, session)
					if httpStatus > http.StatusOK {
						return nil, httpStatus
					}

					// Valid name, so it can be removed from the exclude list.
					excludeList[part] = false
				}
			}
		}
	}

	return excludeList, http.StatusOK
}

// validateColumnName checks if the provided column name is in the list of valid column names. If not, an error payload
// is sent and an HTTP 400 status code is returned.
func validateColumnName(name string, columns []defs.DBColumn, w http.ResponseWriter, session *server.Session) int {
	found := false

	if name != defs.RowIDName {
		for _, column := range columns {
			if name == column.Name {
				found = true

				break
			}
		}

		if !found {
			return util.ErrorResponse(w, session.ID, "invalid COLUMN rest parameter: "+name, http.StatusBadRequest)
		}
	}

	return http.StatusOK
}

func getUpdateRows(r *http.Request, session *server.Session, err error, w http.ResponseWriter, excludeList map[string]bool) (defs.DBRowSet, error, int) {
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r.Body)
	rawPayload := buf.String()

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    rawPayload})
	}

	// Lets get the rows we are to update. This is either a row set, or a single object.
	rowSet := defs.DBRowSet{
		ServerInfo: util.MakeServerInfo(session.ID),
	}

	err = json.Unmarshal([]byte(rawPayload), &rowSet)
	if err != nil || len(rowSet.Rows) == 0 {
		// Not a valid row set, but might be a single item
		item := map[string]interface{}{}

		err = json.Unmarshal([]byte(rawPayload), &item)
		if err != nil {
			return defs.DBRowSet{}, nil, util.ErrorResponse(w, session.ID, "Invalid UPDATE payload: "+err.Error(), http.StatusBadRequest)
		} else {
			rowSet.Count = 1
			rowSet.Rows = make([]map[string]interface{}, 1)
			rowSet.Rows[0] = item
		}
	}

	// Anything in the data map that is on the exclude list is removed
	ui.Log(ui.TableLogger, "table.exclude", ui.A{
		"session": session.ID,
		"data":    excludeList})

	return rowSet, err, http.StatusOK
}

func filterErrorMessage(q string) string {
	if p := strings.Index(q, syntaxErrorPrefix); p >= 0 {
		msg := q[p+len(syntaxErrorPrefix):]
		if p := strings.Index(msg, defs.RowIDName); p > 0 {
			msg = msg[:p]
		}

		return "filter error: " + msg
	}

	return strings.TrimPrefix(q, "pq: ")
}

// Determine if we are using the abstract form of the database response, which
// includes the column names and types in the response as a separate object in
// the result set. This is useful when you want a smaller json payload, but is
// more complex to decode on the client side.
//
// The abstract form is used when the Accept header is set to
// "application/vnd.ego.rowset+json" or the URL has the ?abstract=true parameter.
func useAbstract(r *http.Request) bool {
	// First, did the specify a media type that tells us what to do?
	mediaTypes := r.Header["Accept"]

	for _, mediaType := range mediaTypes {
		if strings.EqualFold(strings.TrimSpace(mediaType), defs.AbstractRowSetMediaType) {
			return true
		}
	}

	// Or, did they use the ?abstract boolean flag to tell us what to do?
	q := r.URL.Query()
	for k, v := range q {
		if k == defs.AbstractParameterName {
			flag := false

			if len(v) == 0 {
				return true
			}

			if len(v) == 1 && data.String(v[0]) == "" {
				return true
			}

			if len(v) == 1 {
				flag = data.BoolOrFalse(v[0])
			}

			ui.Log(ui.RestLogger, "table.abstract", ui.A{
				"flag": flag})

			return flag
		}
	}

	return false
}
