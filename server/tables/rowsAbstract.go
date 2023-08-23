package tables

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
	"github.com/tucats/ego/util"
)

// InsertRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func InsertAbstractRows(user string, isAdmin bool, tableName string, sessionID int, w http.ResponseWriter, r *http.Request) int {
	var err error

	ui.Log(ui.TableLogger, "[%d] Request to insert abstract rows into table %s", sessionID, tableName)

	if p := parameterString(r); p != "" {
		ui.Log(ui.TableLogger, "[%d] request parameters:  %s", sessionID, p)
	}

	db, err := database.Open(&user, "", 0)
	if err == nil && db != nil {
		// If not using sqlite3, fully qualify the table name with the user schema.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(user, tableName)
		}

		// Note that "update" here means add to or change the row. So we check "update"
		// on test for insert permissions
		if !isAdmin && Authorized(sessionID, nil, user, tableName, updateOperation) {
			return util.ErrorResponse(w, sessionID, "User does not have insert permission", http.StatusForbidden)
		}

		buf := new(strings.Builder)
		_, _ = io.Copy(buf, r.Body)
		rawPayload := buf.String()

		ui.Log(ui.RestLogger, "[%d] Raw payload:\n%s", sessionID, util.SessionLog(sessionID, rawPayload))

		// Lets get the rows we are to insert. This is either a row set, or a single object.
		rowSet := defs.DBAbstractRowSet{
			ServerInfo: util.MakeServerInfo(sessionID),
		}

		err = json.Unmarshal([]byte(rawPayload), &rowSet)
		if err != nil || len(rowSet.Rows) == 0 {
			// Not a valid row set, but might be a single item
			item := map[string]interface{}{}

			err = json.Unmarshal([]byte(rawPayload), &item)
			if err != nil {
				return util.ErrorResponse(w, sessionID, "Invalid INSERT payload: "+err.Error(), http.StatusBadRequest)
			} else {
				rowSet.Count = 1
				keys := make([]string, 0)
				values := make([]interface{}, 0)
				for k, v := range item {
					keys = append(keys, k)
					values = append(values, v)
				}

				rowSet.Rows = make([][]interface{}, 1)
				rowSet.Rows[0] = values
				rowSet.Columns = keys
				ui.Log(ui.RestLogger, "[%d] Converted object to rowset payload %v", sessionID, item)
			}
		} else {
			ui.Log(ui.RestLogger, "[%d] Received rowset with %d items", sessionID, len(rowSet.Rows))
		}

		// If we're showing our payload in the log, do that now
		if ui.IsActive(ui.RestLogger) {
			b, _ := json.MarshalIndent(rowSet, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

			ui.WriteLog(ui.RestLogger, "[%d] Resolved REST Request payload:\n%s", sessionID, util.SessionLog(sessionID, string(b)))
		}

		// If at this point we have an empty row set, then just bail out now. Return a success
		// status but an indicator that nothing was done.
		if len(rowSet.Rows) == 0 {
			return util.ErrorResponse(w, sessionID, "No rows found in INSERT payload", http.StatusNoContent)
		}

		// For any object in the payload, we must assign a UUID now. This overrides any previous
		// item in the set for _row_id_ or creates it if not found. Row IDs are always assigned
		// on input only.
		rowIDColumn := -1

		for pos, name := range rowSet.Columns {
			if name == defs.RowIDName {
				rowIDColumn = pos
			}
		}

		if rowIDColumn < 0 {
			rowSet.Columns = append(rowSet.Columns, defs.RowIDName)
			rowIDColumn = len(rowSet.Columns) - 1
		}

		for n := 0; n < len(rowSet.Rows); n++ {
			rowSet.Rows[n][rowIDColumn] = uuid.New().String()
		}

		// Start a transaction, and then lets loop over the rows in the rowset. Note this might
		// be just one row.
		tx, _ := db.Begin()
		count := 0

		for _, row := range rowSet.Rows {
			q, values := formAbstractInsertQuery(r.URL, user, rowSet.Columns, row)
			ui.Log(ui.TableLogger, "[%d] Insert row with query: %s", sessionID, q)

			_, err := db.Exec(q, values...)
			if err == nil {
				count++
			} else {
				_ = tx.Rollback()

				return util.ErrorResponse(w, sessionID, err.Error(), http.StatusConflict)
			}
		}

		if err == nil {
			result := defs.DBRowCount{
				ServerInfo: util.MakeServerInfo(sessionID),
				Count:      count,
			}

			w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

			b, _ := json.MarshalIndent(result, "", "  ")
			_, _ = w.Write(b)

			err = tx.Commit()
			if err == nil {
				ui.Log(ui.TableLogger, "[%d] Inserted %d rows", sessionID, count)

				return http.StatusOK
			}
		}

		_ = tx.Rollback()

		return util.ErrorResponse(w, sessionID, "insert error: "+err.Error(), http.StatusInternalServerError)
	}

	if err != nil {
		return util.ErrorResponse(w, sessionID, "insert error: "+err.Error(), http.StatusInternalServerError)
	}

	return http.StatusOK
}

// ReadRows reads the data for a given table, and returns it as an array
// of structs for each row, with the struct tag being the column name. The
// query can also specify filter, sort, and column query parameters to refine
// the read operation.
func ReadAbstractRows(user string, isAdmin bool, tableName string, sessionID int, w http.ResponseWriter, r *http.Request) int {
	ui.Log(ui.TableLogger, "[%d] Request to read abstract rows from table %s", sessionID, tableName)

	db, err := database.Open(&user, "", 0)
	if err == nil && db != nil {
		// If not using sqlite3, fully qualify the table name with the user schema.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(user, tableName)
		}

		if !isAdmin && Authorized(sessionID, nil, user, tableName, readOperation) {
			return util.ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)
		}

		q := parsing.FormSelectorDeleteQuery(r.URL, parsing.FiltersFromURL(r.URL), parsing.ColumnsFromURL(r.URL), tableName, user, selectVerb, db.Provider)
		if p := strings.Index(q, syntaxErrorPrefix); p > 0 {
			return util.ErrorResponse(w, sessionID, filterErrorMessage(q), http.StatusBadRequest)
		}

		ui.Log(ui.TableLogger, "[%d] Query: %s", sessionID, q)

		err = readAbstractRowData(db.Handle, q, sessionID, w)
		if err == nil {
			return http.StatusOK
		}
	}

	ui.Log(ui.TableLogger, "[%d] Error reading table, %v", sessionID, err)

	return util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
}

func readAbstractRowData(db *sql.DB, q string, sessionID int, w http.ResponseWriter) error {
	var (
		rows     *sql.Rows
		err      error
		rowCount int
		result   = [][]interface{}{}
	)

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

			err = rows.Scan(rowptrs...)
			if err == nil {
				result = append(result, row)
				rowCount++
			}
		}

		resp := defs.DBAbstractRowSet{
			ServerInfo: util.MakeServerInfo(sessionID),
			Columns:    columnNames,
			Rows:       result,
			Count:      len(result),
		}

		w.Header().Add(defs.ContentTypeHeader, defs.AbstractRowSetMediaType)

		b, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = w.Write(b)

		ui.Log(ui.TableLogger, "[%d] Read %d rows of %d columns", sessionID, rowCount, columnCount)
	}

	return err
}

// UpdateRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func UpdateAbstractRows(user string, isAdmin bool, tableName string, sessionID int, w http.ResponseWriter, r *http.Request) int {
	count := 0

	db, err := database.Open(&user, "", 0)
	if err == nil && db != nil {
		// If not using sqlite3, fully qualify the table name with the user schema.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(user, tableName)
		}

		if !isAdmin && Authorized(sessionID, nil, user, tableName, updateOperation) {
			return util.ErrorResponse(w, sessionID, "User does not have update permission", http.StatusForbidden)
		}

		// Get the payload in a string.
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, r.Body)
		rawPayload := buf.String()

		// Lets get the rows we are to update. This is either a row set, or a single object.
		rowSet := defs.DBAbstractRowSet{
			ServerInfo: util.MakeServerInfo(sessionID),
		}

		err = json.Unmarshal([]byte(rawPayload), &rowSet)
		if err != nil || len(rowSet.Rows) == 0 {
			// Not a valid row set, but might be a single item
			item := []interface{}{}

			err = json.Unmarshal([]byte(rawPayload), &item)
			if err != nil {
				return util.ErrorResponse(w, sessionID, "Invalid UPDATE payload: "+err.Error(), http.StatusBadRequest)
			} else {
				rowSet.Count = 1
				rowSet.Rows = make([][]interface{}, 1)
				rowSet.Rows[0] = item
				ui.Log(ui.RestLogger, "[%d] Converted object to rowset payload %v", sessionID, item)
			}
		} else {
			ui.Log(ui.RestLogger, "[%d] Received rowset with %d items", sessionID, len(rowSet.Rows))
		}

		// Start a transaction to ensure atomicity of the entire update
		tx, _ := db.Begin()

		// Loop over the row set doing the updates
		for _, data := range rowSet.Rows {
			ui.Log(ui.TableLogger, "[%d] values list = %v", sessionID, data)

			q := formAbstractUpdateQuery(r.URL, user, rowSet.Columns, data)
			if p := strings.Index(q, syntaxErrorPrefix); p > 0 {
				return util.ErrorResponse(w, sessionID, filterErrorMessage(q), http.StatusBadRequest)
			}

			ui.Log(ui.TableLogger, "[%d] Query: %s", sessionID, q)

			counts, err := db.Exec(q, data...)
			if err == nil {
				rowsAffected, _ := counts.RowsAffected()
				count = count + int(rowsAffected)
			} else {
				_ = tx.Rollback()

				return util.ErrorResponse(w, sessionID, err.Error(), http.StatusConflict)
			}
		}

		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
	}

	if err == nil {
		result := defs.DBRowCount{
			ServerInfo: util.MakeServerInfo(sessionID),
			Count:      count,
		}

		w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

		b, _ := json.MarshalIndent(result, "", "  ")
		_, _ = w.Write(b)

		ui.Log(ui.TableLogger, "[%d] Updated %d rows", sessionID, count)
	} else {
		return util.ErrorResponse(w, sessionID, "Error updating table, "+err.Error(), http.StatusInternalServerError)
	}

	return http.StatusOK
}
