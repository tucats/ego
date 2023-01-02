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
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// InsertRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func InsertAbstractRows(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	var err error

	// Verify that the parameters are valid, if given.
	if invalid := util.ValidateParameters(r.URL, map[string]string{
		defs.UserParameterName:     "string",
		defs.AbstractParameterName: "bool",
	}); !errors.Nil(invalid) {
		util.ErrorResponse(w, sessionID, invalid.Error(), http.StatusBadRequest)

		return
	}

	tableName, _ = fullName(user, tableName)

	ui.Debug(ui.ServerLogger, "[%d] Request to insert abstract rows into table %s", sessionID, tableName)

	if p := parameterString(r); p != "" {
		ui.Debug(ui.ServerLogger, "[%d] request parameters:  %s", sessionID, p)
	}

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {
		// Note that "update" here means add to or change the row. So we check "update"
		// on test for insert permissions
		if !isAdmin && Authorized(sessionID, nil, user, tableName, updateOperation) {
			util.ErrorResponse(w, sessionID, "User does not have insert permission", http.StatusForbidden)

			return
		}

		/*
			// Get the column metadata for the table we're insert into, so we can validate column info.
			var columns []defs.DBColumn

			tableName, _ = fullName(user, tableName)

			columns, err = getColumnInfo(db, user, tableName, sessionID)
			if !errors.Nil(err) {
				util.ErrorResponse(w, sessionID, "Unable to read table metadata, "+err.Error(), http.StatusBadRequest)

				return
			}
		*/

		buf := new(strings.Builder)
		_, _ = io.Copy(buf, r.Body)
		rawPayload := buf.String()

		ui.Debug(ui.RestLogger, "[%d] Raw payload:\n%s", sessionID, util.SessionLog(sessionID, rawPayload))

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
				util.ErrorResponse(w, sessionID, "Invalid INSERT payload: "+err.Error(), http.StatusBadRequest)

				return
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
				ui.Debug(ui.RestLogger, "[%d] Converted object to rowset payload %v", sessionID, item)
			}
		} else {
			ui.Debug(ui.RestLogger, "[%d] Received rowset with %d items", sessionID, len(rowSet.Rows))
		}

		// If we're showing our payload in the log, do that now
		if ui.IsActive(ui.RestLogger) {
			b, _ := json.MarshalIndent(rowSet, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

			ui.Debug(ui.RestLogger, "[%d] Resolved REST Request payload:\n%s", sessionID, util.SessionLog(sessionID, string(b)))
		}

		// If at this point we have an empty row set, then just bail out now. Return a success
		// status but an indicator that nothing was done.
		if len(rowSet.Rows) == 0 {
			util.ErrorResponse(w, sessionID, "No rows found in INSERT payload", http.StatusNoContent)

			return
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
			ui.Debug(ui.TableLogger, "[%d] Insert row with query: %s", sessionID, q)

			_, err := db.Exec(q, values...)
			if err == nil {
				count++
			} else {
				util.ErrorResponse(w, sessionID, err.Error(), http.StatusConflict)
				_ = tx.Rollback()

				return
			}
		}

		if err == nil {
			result := defs.DBRowCount{
				ServerInfo: util.MakeServerInfo(sessionID),
				Count:      count,
			}

			w.Header().Add("Content-Type", defs.RowCountMediaType)

			b, _ := json.MarshalIndent(result, "", "  ")
			_, _ = w.Write(b)

			err = tx.Commit()
			if err == nil {
				ui.Debug(ui.TableLogger, "[%d] Inserted %d rows", sessionID, count)

				return
			}
		}

		_ = tx.Rollback()

		util.ErrorResponse(w, sessionID, "insert error: "+err.Error(), http.StatusInternalServerError)

		return
	}

	if !errors.Nil(err) {
		util.ErrorResponse(w, sessionID, "insert error: "+err.Error(), http.StatusInternalServerError)
	}
}

// ReadRows reads the data for a given table, and returns it as an array
// of structs for each row, with the struct tag being the column name. The
// query can also specify filter, sort, and column query parameters to refine
// the read operation.
func ReadAbstractRows(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	tableName, _ = fullName(user, tableName)

	// Verify that the parameters are valid, if given.
	if invalid := util.ValidateParameters(r.URL, map[string]string{
		defs.StartParameterName:    "int",
		defs.LimitParameterName:    "int",
		defs.ColumnParameterName:   "list",
		defs.SortParameterName:     "list",
		defs.FilterParameterName:   defs.Any,
		defs.UserParameterName:     "string",
		defs.AbstractParameterName: "bool",
	}); !errors.Nil(invalid) {
		util.ErrorResponse(w, sessionID, invalid.Error(), http.StatusBadRequest)

		return
	}

	ui.Debug(ui.ServerLogger, "[%d] Request to read abstract rows from table %s", sessionID, tableName)

	if p := parameterString(r); p != "" {
		ui.Debug(ui.ServerLogger, "[%d] request parameters:  %s", sessionID, p)
	}

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {
		if !isAdmin && Authorized(sessionID, nil, user, tableName, readOperation) {
			util.ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)

			return
		}

		q := formSelectorDeleteQuery(r.URL, filtersFromURL(r.URL), columnsFromURL(r.URL), tableName, user, selectVerb)
		if p := strings.Index(q, syntaxErrorPrefix); p > 0 {
			util.ErrorResponse(w, sessionID, filterErrorMessage(q), http.StatusBadRequest)

			return
		}

		ui.Debug(ui.TableLogger, "[%d] Query: %s", sessionID, q)

		err = readAbstractRowData(db, q, sessionID, w)
		if err == nil {
			return
		}
	}

	ui.Debug(ui.TableLogger, "[%d] Error reading table, %v", sessionID, err)
	util.ErrorResponse(w, sessionID, err.Error(), 400)
}

func readAbstractRowData(db *sql.DB, q string, sessionID int32, w http.ResponseWriter) error {
	var rows *sql.Rows

	var err error

	result := [][]interface{}{}

	rowCount := 0

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

		w.Header().Add("Content-Type", defs.AbstractRowSetMediaType)

		b, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = w.Write(b)

		ui.Debug(ui.TableLogger, "[%d] Read %d rows of %d columns", sessionID, rowCount, columnCount)
	}

	return err
}

// UpdateRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func UpdateAbstractRows(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	tableName, _ = fullName(user, tableName)
	count := 0

	// Verify that the parameters are valid, if given.
	if invalid := util.ValidateParameters(r.URL, map[string]string{
		defs.FilterParameterName: defs.Any,
		defs.UserParameterName:   "string",
		defs.ColumnParameterName: "string",
	}); !errors.Nil(invalid) {
		util.ErrorResponse(w, sessionID, invalid.Error(), http.StatusBadRequest)

		return
	}

	ui.Debug(ui.ServerLogger, "[%d] Request to update abstract rows in table %s", sessionID, tableName)

	if p := parameterString(r); p != "" {
		ui.Debug(ui.ServerLogger, "[%d] request parameters:  %s", sessionID, p)
	}

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {
		if !isAdmin && Authorized(sessionID, nil, user, tableName, updateOperation) {
			util.ErrorResponse(w, sessionID, "User does not have update permission", http.StatusForbidden)

			return
		}

		// For debugging, show the raw payload. We may remove this later...
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, r.Body)
		rawPayload := buf.String()

		ui.Debug(ui.RestLogger, "[%d] Raw payload:\n%s", sessionID, util.SessionLog(sessionID, rawPayload))

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
				util.ErrorResponse(w, sessionID, "Invalid UPDATE payload: "+err.Error(), http.StatusBadRequest)

				return
			} else {
				rowSet.Count = 1
				rowSet.Rows = make([][]interface{}, 1)
				rowSet.Rows[0] = item
				ui.Debug(ui.RestLogger, "[%d] Converted object to rowset payload %v", sessionID, item)
			}
		} else {
			ui.Debug(ui.RestLogger, "[%d] Received rowset with %d items", sessionID, len(rowSet.Rows))
		}

		// Start a transaction to ensure atomicity of the entire update
		tx, _ := db.Begin()

		// Loop over the row set doing the updates
		for _, data := range rowSet.Rows {
			ui.Debug(ui.TableLogger, "[%d] values list = %v", sessionID, data)

			q := formAbstractUpdateQuery(r.URL, user, rowSet.Columns, data)
			if p := strings.Index(q, syntaxErrorPrefix); p > 0 {
				util.ErrorResponse(w, sessionID, filterErrorMessage(q), http.StatusBadRequest)

				return
			}

			ui.Debug(ui.TableLogger, "[%d] Query: %s", sessionID, q)

			counts, err := db.Exec(q, data...)
			if err == nil {
				rowsAffected, _ := counts.RowsAffected()
				count = count + int(rowsAffected)
			} else {
				util.ErrorResponse(w, sessionID, err.Error(), http.StatusConflict)
				_ = tx.Rollback()

				return
			}
		}

		if errors.Nil(err) {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
	}

	if errors.Nil(err) {
		result := defs.DBRowCount{
			ServerInfo: util.MakeServerInfo(sessionID),
			Count:      count,
		}

		w.Header().Add("Content-Type", defs.RowCountMediaType)

		b, _ := json.MarshalIndent(result, "", "  ")
		_, _ = w.Write(b)

		ui.Debug(ui.TableLogger, "[%d] Updated %d rows", sessionID, count)
	} else {
		util.ErrorResponse(w, sessionID, "Error updating table, "+err.Error(), http.StatusInternalServerError)
	}
}
