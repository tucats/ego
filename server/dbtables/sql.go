package dbtables

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

func SQLTransaction(r *http.Request, w http.ResponseWriter, sessionID int32, user string) {
	ui.Debug(ui.TableLogger, "[%d] Executing SQL statements as a transaction", sessionID)

	var body string

	if b, err := ioutil.ReadAll(r.Body); err == nil && b != nil {
		body = string(b)
	} else {
		util.ErrorResponse(w, sessionID, "Empty request payload", http.StatusBadRequest)

		return
	}

	// The payload can be either a single string, or a sequence of strings in an array.
	// So try to decode as an array, but if that fails, try as a single string. Note that
	// we have to re-use the body that was previously read because the r.Body() reader has
	// been exhausted already.
	statements := []string{}

	err := json.Unmarshal([]byte(body), &statements)
	if !errors.Nil(err) {
		statement := ""

		err := json.Unmarshal([]byte(body), &statement)
		if !errors.Nil(err) {
			util.ErrorResponse(w, sessionID, "Invalid SQL payload: "+err.Error(), http.StatusBadRequest)

			return
		}

		// The SQL could be multiple statements separated by a semicolon.  If so, we'd need to break the
		// code up into separate statments.
		statements = splitSQLStatements(statement)
	} else {
		// it's possible that an array was sent but the string values may contain
		// multiple statements. If so, we need to break them up again.
		wasResplit := false
		newStatements := make([]string, 0)

		for _, statement := range statements {
			splitStatements := splitSQLStatements(statement)
			if len(splitStatements) > 1 {
				wasResplit = true
			}
			newStatements = append(newStatements, splitStatements...)
		}

		// Did we end up having to further split the statements in the array? IF so, replace the
		// array with the newly-split array.
		if wasResplit {
			statements = newStatements
		}

		// If we're doing REST logging, dump out the statement array we will execute now.
		if ui.LoggerIsActive(ui.RestLogger) {
			b, _ := json.MarshalIndent(statements, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

			ui.Debug(ui.RestLogger, "[%d] SQL statements:\n%s", sessionID, util.SessionLog(sessionID, string(b)))
		}
	}

	// Sanity check -- if there is a SELECT in the transaction, it must be the last item in the
	// array since there's no way to retain the result set otherwise.
	for n, statement := range statements {
		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(statement)), "select ") {
			if n < len(statements)-1 {
				util.ErrorResponse(w, sessionID, "SELECT statement can only be used as last statement in transaction", http.StatusBadRequest)

				return
			}
		}
	}

	// We always do this under control of a transaction, so set that up now.
	db, err := OpenDB(sessionID, user, "")
	if !errors.Nil(err) {
		util.ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

		return
	} else {
		defer db.Close()
	}

	tx, err := db.Begin()
	if !errors.Nil(err) {
		util.ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)

		return
	}

	// Now execute each statement from the array of strings.
	for n, statement := range statements {
		if len(strings.TrimSpace(statement)) == 0 || statement[:1] == "#" {
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(statement)), "select ") {
			ui.Debug(ui.TableLogger, "[%d] SQL query: %s", sessionID, statement)

			err = readRowDataTx(tx, statement, sessionID, w)
			if err != nil {
				util.ErrorResponse(w, sessionID, "Error reading SQL query; "+err.Error(), http.StatusInternalServerError)

				return
			}
		} else {
			var rows sql.Result

			ui.Debug(ui.TableLogger, "[%d] SQL execute: %s", sessionID, statement)

			rows, err = tx.Exec(statement)
			if err == nil {
				count, _ := rows.RowsAffected()

				ui.Debug(ui.TableLogger, "[%d] Updated %d rows", sessionID, count)

				// If this is the last operation in the transaction, this is also our response
				// payload.
				if n == len(statements)-1 {
					reply := defs.DBRowCount{
						ServerInfo: util.MakeServerInfo(sessionID),
						Count:      int(count),
					}
					w.Header().Add("Content-Type", defs.RowCountMediaType)

					b, _ := json.MarshalIndent(reply, "", "  ")
					_, _ = w.Write(b)

					if ui.LoggerIsActive(ui.RestLogger) {
						ui.Debug(ui.RestLogger, "[%d] Response payload:\n%s", sessionID, util.SessionLog(sessionID, string(b)))
					}

					break
				}
			}

			if !errors.Nil(err) {
				_ = tx.Rollback()
			}
		}
	}

	if err != nil {
		_ = tx.Rollback()

		util.ErrorResponse(w, sessionID, "Error in SQL execute; "+err.Error(), http.StatusInternalServerError)
	} else {
		err = tx.Commit()

		if err != nil {
			_ = tx.Rollback()

			util.ErrorResponse(w, sessionID, "Error committing transaction; "+err.Error(), http.StatusInternalServerError)
		}
	}
}

func readRowDataTx(tx *sql.Tx, q string, sessionID int32, w http.ResponseWriter) error {
	var rows *sql.Rows

	var err error

	result := []map[string]interface{}{}
	rowCount := 0

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

		resp := defs.DBRowSet{
			ServerInfo: util.MakeServerInfo(sessionID),
			Rows:       result,
			Count:      len(result),
		}

		status := http.StatusOK

		w.Header().Add("Content-Type", defs.RowSetMediaType)
		w.WriteHeader(status)

		b, _ := json.MarshalIndent(resp, "", "  ")
		_, _ = w.Write(b)

		ui.Debug(ui.TableLogger, "[%d] Read %d rows of %d columns", sessionID, rowCount, columnCount)

		if ui.LoggerIsActive(ui.RestLogger) {
			ui.Debug(ui.RestLogger, "[%d] Response payload:\n%s", sessionID, util.SessionLog(sessionID, string(b)))
		}
	}

	return err
}

func splitSQLStatements(s string) []string {
	result := []string{}

	t := tokenizer.New(s)
	next := ""

	for !t.AtEnd() {
		token := t.Next()
		if token == ";" {
			if len(strings.TrimSpace(next)) > 0 {
				result = append(result, next)
			}

			next = ""
		} else {
			if len(strings.TrimSpace(next)) > 0 {
				next = next + " "
			}

			next = next + token
		}
	}

	if len(strings.TrimSpace(next)) > 0 {
		result = append(result, next)
	}

	return result
}
