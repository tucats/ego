package tables

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
	"github.com/tucats/ego/util"
)

// InsertRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func InsertAbstractRows(user string, isAdmin bool, tableName string, session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var err error

	dsnName := data.String(session.URLParts["dsn"])
	db, err := database.Open(&user, dsnName, 0)

	// If not using sqlite3, fully qualify the table name with the user schema.
	if db.Provider != sqlite3Provider {
		tableName, _ = parsing.FullName(user, tableName)
	}

	if p := parameterString(r); p != "" {
		ui.Log(ui.TableLogger, "table.parms", ui.A{
			"session": session.ID,
			"params":  p})
	}

	if err == nil && db != nil {
		// If not using sqlite3, fully qualify the table name with the user schema.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(user, tableName)
		}

		// Note that "update" here means add to or change the row. So we check "update"
		// on test for insert permissions
		if !isAdmin && Authorized(session.ID, nil, user, tableName, updateOperation) {
			return util.ErrorResponse(w, session.ID, "User does not have insert permission", http.StatusForbidden)
		}

		buf := new(strings.Builder)
		_, _ = io.Copy(buf, r.Body)
		rawPayload := buf.String()

		ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    rawPayload})

		// Lets get the rows we are to insert. This is either a row set, or a single object.
		rowSet := defs.DBAbstractRowSet{
			ServerInfo: util.MakeServerInfo(session.ID),
		}

		err = json.Unmarshal([]byte(rawPayload), &rowSet)
		if err != nil || len(rowSet.Rows) == 0 {
			// Not a valid row set, but might be a single item
			item := map[string]interface{}{}

			err = json.Unmarshal([]byte(rawPayload), &item)
			if err != nil {
				return util.ErrorResponse(w, session.ID, "Invalid INSERT payload: "+err.Error(), http.StatusBadRequest)
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

				rowSet.Columns = make([]defs.DBAbstractColumn, len(keys))
				for i, k := range keys {
					rowSet.Columns[i] = defs.DBAbstractColumn{
						Name: k,
					}
				}
			}
		}

		// If at this point we have an empty row set, then just bail out now. Return a success
		// status but an indicator that nothing was done.
		if len(rowSet.Rows) == 0 {
			return util.ErrorResponse(w, session.ID, errors.ErrTableNoRows.Error(), http.StatusNoContent)
		}

		// For any object in the payload, we must assign a UUID now. This overrides any previous
		// item in the set for _row_id_ or creates it if not found. Row IDs are always assigned
		// on input only.
		rowIDColumn := -1

		for pos, name := range rowSet.Columns {
			if name.Name == defs.RowIDName {
				rowIDColumn = pos
			}
		}

		if rowIDColumn < 0 {
			rowSet.Columns = append(rowSet.Columns, defs.DBAbstractColumn{
				Name: defs.RowIDName,
				Type: "string",
			})

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
			columnNames := make([]string, len(rowSet.Columns))
			for i, c := range rowSet.Columns {
				columnNames[i] = c.Name
			}

			q, values := formAbstractInsertQuery(r.URL, user, columnNames, row)

			_, err := db.Exec(q, values...)
			if err == nil {
				count++
			} else {
				_ = tx.Rollback()

				return util.ErrorResponse(w, session.ID, err.Error(), http.StatusConflict)
			}
		}

		if err == nil {
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
				ui.Log(ui.TableLogger, "table.inserted.rows", ui.A{
					"session": session.ID,
					"count":   count})

				return http.StatusOK
			}
		}

		_ = tx.Rollback()

		return util.ErrorResponse(w, session.ID, "insert error: "+err.Error(), http.StatusInternalServerError)
	}

	if err != nil {
		return util.ErrorResponse(w, session.ID, "insert error: "+err.Error(), http.StatusInternalServerError)
	}

	return http.StatusOK
}

// ReadRows reads the data for a given table, and returns it as an array
// of structs for each row, with the struct tag being the column name. The
// query can also specify filter, sort, and column query parameters to refine
// the read operation.
func ReadAbstractRows(user string, isAdmin bool, tableName string, session *server.Session, w http.ResponseWriter, r *http.Request) int {
	dsnName := data.String(session.URLParts["dsn"])

	db, err := database.Open(&user, dsnName, dsns.DSNReadAction)
	if err == nil && db != nil {
		// If not using sqlite3, fully qualify the table name with the user schema.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(user, tableName)
		}

		if !isAdmin && Authorized(session.ID, nil, user, tableName, readOperation) {
			return util.ErrorResponse(w, session.ID, "User does not have read permission", http.StatusForbidden)
		}

		var q string

		q, err = parsing.FormSelectorDeleteQuery(r.URL, parsing.FiltersFromURL(r.URL), parsing.ColumnsFromURL(r.URL), tableName, user, selectVerb, db.Provider)
		if err != nil {
			return util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)
		}

		ui.Log(ui.TableLogger, "sql.query", ui.A{
			"session": session.ID,
			"sql":     q})

		if err = readAbstractRowData(db.Handle, q, session, w); errors.Nil(err) {
			return http.StatusOK
		}
	}

	if err == nil && db == nil {
		err = errors.ErrNoDatabase
	}

	status := http.StatusOK

	if err != nil {
		ui.Log(ui.TableLogger, "table.read.error", ui.A{
			"session": session.ID,
			"error":   err.Error()})

		if strings.Contains(err.Error(), "no such table") {
			status = http.StatusNotFound
		} else {
			status = http.StatusBadRequest
		}
	}

	return status
}

func readAbstractRowData(db *sql.DB, q string, session *server.Session, w http.ResponseWriter) error {
	var (
		rows     *sql.Rows
		err      error
		rowCount int
		result   = [][]interface{}{}
		columns  []defs.DBAbstractColumn
	)

	rows, err = db.Query(q)
	if rows != nil {
		defer rows.Close()
	}

	if err != nil {
		status := http.StatusInternalServerError
		// If this is a table-not-found error, change the status code to 404
		if strings.Contains(err.Error(), "no such table") {
			status = http.StatusNotFound
		}

		util.ErrorResponse(w, session.ID, "Error executing query: "+err.Error(), status)

		return err
	}

	if columnNames, err := rows.Columns(); err != nil {
		util.ErrorResponse(w, session.ID, "Error reading column names: "+err.Error(), http.StatusInternalServerError)

		return err
	} else {
		columns = make([]defs.DBAbstractColumn, len(columnNames))
		for i, name := range columnNames {
			columns[i] = defs.DBAbstractColumn{
				Name: name,
			}
		}
	}

	if typeData, err := rows.ColumnTypes(); err == nil {
		for i, ct := range typeData {
			columns[i].Type = strings.ToLower(ct.DatabaseTypeName())
			columns[i].Nullable, _ = ct.Nullable()

			size, ok := ct.Length()
			if !ok {
				size = -1
			}

			columns[i].Size = int(size)
		}
	} else {
		util.ErrorResponse(w, session.ID, "Error reading column types: "+err.Error(), http.StatusInternalServerError)

		return err
	}

	columnList := strings.Builder{}

	for i, c := range columns {
		if i > 0 {
			columnList.WriteString(", ")
		}

		columnList.WriteString(c.Name)
	}

	columnCount := len(columns)

	for rows.Next() {
		row := make([]interface{}, columnCount)
		rowPointers := make([]interface{}, columnCount)

		for i := range row {
			rowPointers[i] = &row[i]
		}

		err = rows.Scan(rowPointers...)
		if err == nil {
			result = append(result, row)
			rowCount++
		} else {
			util.ErrorResponse(w, session.ID, "Error reading row data: "+err.Error(), http.StatusInternalServerError)

			return err
		}
	}

	resp := defs.DBAbstractRowSet{
		ServerInfo: util.MakeServerInfo(session.ID),
		Columns:    columns,
		Rows:       result,
		Count:      len(result),
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.AbstractRowSetMediaType)

	b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	ui.Log(ui.TableLogger, "table.read.", ui.A{
		"session": session.ID,
		"rows":    rowCount,
		"columns": columnCount})

	return err
}

// UpdateRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func UpdateAbstractRows(user string, isAdmin bool, tableName string, session *server.Session, w http.ResponseWriter, r *http.Request) int {
	count := 0
	dsnName := data.String(session.URLParts["dsn"])

	db, err := database.Open(&user, dsnName, 0)
	if err == nil && db != nil {
		// If not using sqlite3, fully qualify the table name with the user schema.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(user, tableName)
		}

		if !isAdmin && Authorized(session.ID, nil, user, tableName, updateOperation) {
			return util.ErrorResponse(w, session.ID, "User does not have update permission", http.StatusForbidden)
		}

		// Get the payload in a string.
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, r.Body)
		rawPayload := buf.String()

		// Lets get the rows we are to update. This is either a row set, or a single object.
		rowSet := defs.DBAbstractRowSet{
			ServerInfo: util.MakeServerInfo(session.ID),
		}

		err = json.Unmarshal([]byte(rawPayload), &rowSet)
		if err != nil || len(rowSet.Rows) == 0 {
			// Not a valid row set, but might be a single item
			item := []interface{}{}

			err = json.Unmarshal([]byte(rawPayload), &item)
			if err != nil {
				return util.ErrorResponse(w, session.ID, "Invalid UPDATE payload: "+err.Error(), http.StatusBadRequest)
			} else {
				rowSet.Count = 1
				rowSet.Rows = make([][]interface{}, 1)
				rowSet.Rows[0] = item
			}
		}

		// Start a transaction to ensure atomicity of the entire update
		tx, _ := db.Begin()

		// Loop over the row set doing the updates
		for _, data := range rowSet.Rows {
			ui.Log(ui.TableLogger, "table.values", ui.A{
				"session": session.ID,
				"data":    data})

			// Get the column names for the update
			columns := make([]string, len(rowSet.Columns))
			for i, c := range rowSet.Columns {
				columns[i] = c.Name
			}

			q, err := formAbstractUpdateQuery(r.URL, user, columns, data)
			if err != nil {
				return util.ErrorResponse(w, session.ID, filterErrorMessage(q), http.StatusBadRequest)
			}

			ui.Log(ui.TableLogger, "sql.query", ui.A{
				"session": session.ID,
				"sql":     q})

			counts, err := db.Exec(q, data...)
			if err == nil {
				rowsAffected, _ := counts.RowsAffected()
				count = count + int(rowsAffected)
			} else {
				_ = tx.Rollback()

				return util.ErrorResponse(w, session.ID, err.Error(), http.StatusConflict)
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

		ui.Log(ui.TableLogger, "table.updated", ui.A{
			"session": session.ID,
			"count":   count,
			"status":  http.StatusOK})
	} else {
		return util.ErrorResponse(w, session.ID, "Error updating table, "+err.Error(), http.StatusInternalServerError)
	}

	return http.StatusOK
}
