package dbtables

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

// DeleteRows deletes rows from a table. If no filter is provided, then all rows are
// deleted and the tale is empty. If filter(s) are applied, only the matching rows
// are deleted. The function returns the number of rows deleted.
func DeleteRows(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	tableName, _ = fullName(user, tableName)
	// Verify that the parameters are valid, if given.
	if invalid := util.ValidateParameters(r.URL, map[string]string{
		defs.FilterParameterName: defs.Any,
		defs.UserParameterName:   "string",
	}); !errors.Nil(invalid) {
		ErrorResponse(w, sessionID, invalid.Error(), http.StatusBadRequest)

		return
	}

	ui.Debug(ui.ServerLogger, "[%d] Request to delete rows from table %s", sessionID, tableName)

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {
		if !isAdmin && Authorized(sessionID, nil, user, tableName, deleteOperation) {
			ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)

			return
		}

		q := formSelectorDeleteQuery(r.URL, user, deleteVerb)

		ui.Debug(ui.TableLogger, "[%d] Exec: %s", sessionID, q)

		rows, err := db.Exec(q)
		if err == nil {
			rowCount, _ := rows.RowsAffected()

			resp := defs.DBRowCount{
				Count: int(rowCount),
			}
			b, _ := json.MarshalIndent(resp, "", "  ")
			_, _ = w.Write(b)

			ui.Debug(ui.TableLogger, "[%d] Deleted %d rows ", sessionID, rowCount)

			return
		}
	}

	ui.Debug(ui.ServerLogger, "[%d] Error deleting from table, %v", sessionID, err)
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(err.Error()))
}

// InsertRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func InsertRows(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	var err error

	// Verify that the parameters are valid, if given.
	if invalid := util.ValidateParameters(r.URL, map[string]string{
		defs.UserParameterName: "string",
	}); !errors.Nil(invalid) {
		ErrorResponse(w, sessionID, invalid.Error(), http.StatusBadRequest)

		return
	}

	tableName, _ = fullName(user, tableName)

	ui.Debug(ui.ServerLogger, "[%d] Request to insert rows into table %s", sessionID, tableName)

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {
		// Note that "update" here means add to or change the row. So we check "update"
		// on test for insert permissions
		if !isAdmin && Authorized(sessionID, nil, user, tableName, updateOperation) {
			ErrorResponse(w, sessionID, "User does not have insert permission", http.StatusForbidden)

			return
		}

		var columns []defs.DBColumn

		var data map[string]interface{}

		tableName, _ = fullName(user, tableName)

		columns, err = getColumnInfo(db, user, tableName, sessionID)
		if !errors.Nil(err) {
			ErrorResponse(w, sessionID, "Unable to read table metadata, "+err.Error(), http.StatusBadRequest)

			return
		}

		err = json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			ErrorResponse(w, sessionID, "Invalid INSERT payload: "+err.Error(), http.StatusBadRequest)

			return
		}

		if _, found := data[defs.RowIDName]; !found {
			data[defs.RowIDName] = uuid.New().String()
		}

		for _, column := range columns {
			v, ok := data[column.Name]
			if !ok {
				ErrorResponse(w, sessionID, "Invalid column in request payload: "+column.Name, http.StatusBadRequest)

				return
			}

			// If it's one of the date/time values, make sure it is wrapped in single qutoes.
			if keywordMatch(column.Type, "time", "date", "timestamp") {
				text := strings.TrimPrefix(strings.TrimSuffix(datatypes.GetString(v), "\""), "\"")
				data[column.Name] = "'" + strings.TrimPrefix(strings.TrimSuffix(text, "'"), "'") + "'"
				ui.Debug(ui.TableLogger, "[%d] updated column %s value from %v to %v", sessionID, column.Name, v, data[column.Name])
			}
		}

		q, values := formInsertQuery(r.URL, user, data)
		ui.Debug(ui.TableLogger, "[%d] Insert rows with query: %s", sessionID, q)

		counts, err := db.Exec(q, values...)
		if err == nil {
			rows, _ := counts.RowsAffected()
			result := defs.DBRowCount{
				Count: int(rows),
			}

			b, _ := json.MarshalIndent(result, "", "  ")
			_, _ = w.Write(b)

			ui.Debug(ui.TableLogger, "[%d] Updated %d rows", sessionID, rows)

			return
		}

		ErrorResponse(w, sessionID, "insert error: "+err.Error(), http.StatusInternalServerError)

		return
	}

	if !errors.Nil(err) {
		ErrorResponse(w, sessionID, "insert error: "+err.Error(), http.StatusInternalServerError)
	}
}

// ReadRows reads the data for a given table, and returns it as an array
// of structs for each row, with the struct tag being the column name. The
// query can also specify filter, sort, and column query parameters to refine
// the read operation.
func ReadRows(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	tableName, _ = fullName(user, tableName)

	// Verify that the parameters are valid, if given.
	if invalid := util.ValidateParameters(r.URL, map[string]string{
		defs.StartParameterName:  "int",
		defs.LimitParameterName:  "int",
		defs.ColumnParameterName: "list",
		defs.SortParameterName:   "list",
		defs.FilterParameterName: defs.Any,
		defs.UserParameterName:   "string",
	}); !errors.Nil(invalid) {
		ErrorResponse(w, sessionID, invalid.Error(), http.StatusBadRequest)

		return
	}

	ui.Debug(ui.ServerLogger, "[%d] Request to read rows from table %s", sessionID, tableName)

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {
		if !isAdmin && Authorized(sessionID, nil, user, tableName, readOperation) {
			ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)

			return
		}

		q := formSelectorDeleteQuery(r.URL, user, selectVerb)

		ui.Debug(ui.TableLogger, "[%d] Query: %s", sessionID, q)

		err = readRowData(db, q, sessionID, w)
		if err == nil {
			return
		}
	}

	ui.Debug(ui.TableLogger, "[%d] Error reading table, %v", sessionID, err)
	ErrorResponse(w, sessionID, err.Error(), 400)
}

func readRowData(db *sql.DB, q string, sessionID int32, w http.ResponseWriter) error {
	var rows *sql.Rows

	var err error

	result := []map[string]interface{}{}
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
				newRow := map[string]interface{}{}
				for i, v := range row {
					newRow[columnNames[i]] = v
				}

				result = append(result, newRow)
				rowCount++
			}
		}

		resp := defs.DBRows{
			Rows:  result,
			Count: len(result),
		}

		b, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = w.Write(b)

		ui.Debug(ui.TableLogger, "[%d] Read %d rows of %d columns", sessionID, rowCount, columnCount)
	}

	return err
}

// UpdateRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func UpdateRows(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	tableName, _ = fullName(user, tableName)
	// Verify that the parameters are valid, if given.
	if invalid := util.ValidateParameters(r.URL, map[string]string{
		defs.FilterParameterName: defs.Any,
		defs.UserParameterName:   "string",
	}); !errors.Nil(invalid) {
		ErrorResponse(w, sessionID, invalid.Error(), http.StatusBadRequest)

		return
	}

	ui.Debug(ui.ServerLogger, "[%d] Request to update rows in table %s", sessionID, tableName)

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {
		if !isAdmin && Authorized(sessionID, nil, user, tableName, updateOperation) {
			ErrorResponse(w, sessionID, "User does not have update permission", http.StatusForbidden)

			return
		}

		var data map[string]interface{}

		err = json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			ErrorResponse(w, sessionID, "Invalid UPDATE payload: "+err.Error(), http.StatusBadRequest)

			return
		}

		q, values := formUpdateQuery(r.URL, user, data)
		ui.Debug(ui.TableLogger, "[%d] Query: %s", sessionID, q)

		counts, err := db.Exec(q, values...)
		if err == nil {
			rows, _ := counts.RowsAffected()
			result := defs.DBRowCount{
				Count: int(rows),
			}

			b, _ := json.MarshalIndent(result, "", "  ")
			_, _ = w.Write(b)

			ui.Debug(ui.TableLogger, "[%d] Updated %d rows", sessionID, rows)

			return
		}

		ErrorResponse(w, sessionID, "Error updating table, "+err.Error(), http.StatusInternalServerError)

		return
	}

	if !errors.Nil(err) {
		ErrorResponse(w, sessionID, "Error updating table, "+err.Error(), http.StatusInternalServerError)
	}
}
