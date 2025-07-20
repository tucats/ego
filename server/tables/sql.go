package tables

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// SQLTransaction executes a series of SQL statements from the REST client and attempts
// to execute them all as a single transaction. The payload can be an array of strings,
// each of which is executed as a single statement within the transaction, or the payload
// can be a single string which is parsed using a ";" separator into individual statements.
//
// If there are multiple statements, then only the last statement in the payload can be a
// SELECT statements as that will be the result of the request.
func SQLTransaction(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		body      string
		rows      sql.Result
		sessionID = session.ID
	)

	ui.Log(ui.TableLogger, "table.tx", ui.A{
		"session": sessionID})

	if b, err := io.ReadAll(r.Body); err == nil && b != nil {
		body = string(b)
	} else {
		return util.ErrorResponse(w, sessionID, "Empty request payload", http.StatusBadRequest)
	}

	// The payload can be either a single string, or a sequence of strings in an array.
	// So try to decode as an array, but if that fails, try as a single string. Note that
	// we have to re-use the body that was previously read because the r.Body() reader has
	// been exhausted already.
	statements, httpStatus := getStatementsFromRequest(body, w, sessionID)
	if httpStatus > http.StatusOK {
		return httpStatus
	}

	// Sanity check -- if there is a SELECT in the transaction, it must be the last item in the
	// array since there's no way to retain the result set otherwise.
	for n, statement := range statements {
		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(statement)), "select ") {
			if n < len(statements)-1 {
				return util.ErrorResponse(w, sessionID, "SELECT statement can only be used as last statement in transaction", http.StatusBadRequest)
			}
		}
	}

	// We always do this under control of a transaction, so set that up now.
	db, err := database.Open(session, data.String(session.URLParts["dsn"]), dsns.DSNWriteAction+dsns.DSNReadAction)
	if err != nil {
		return util.ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)
	} else {
		defer db.Close()
	}

	err = db.Begin()
	if err != nil {
		return util.ErrorResponse(w, sessionID, err.Error(), http.StatusInternalServerError)
	}

	// Now execute each statement from the array of strings.
	err, httpStatus = executeStatements(statements, sessionID, db, w, rows, err)
	if httpStatus > http.StatusOK {
		_ = db.Rollback()

		return httpStatus
	}

	if err != nil {
		_ = db.Rollback()

		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}

		if strings.Contains(err.Error(), "constraint") {
			status = http.StatusConflict
		}

		return util.ErrorResponse(w, sessionID, "Error in SQL execute; "+filterErrorMessage(err.Error()), status)
	} else {
		if err = db.Commit(); err != nil {
			_ = db.Rollback()

			return util.ErrorResponse(w, sessionID, "Error committing transaction; "+filterErrorMessage(err.Error()), http.StatusInternalServerError)
		}
	}

	return http.StatusOK
}

// executeStatements executes each of the SQL statements in the provided array and returns the first error encountered. Note that if the array contains
// a SELECT statement, it must be the last item in the array since there's no way to retain the result set otherwise. If there is a select statement,
// the response payload is the result set from the SELECT statement. Otherwise, the response payload is the row count from the operations.
func executeStatements(statements []string, sessionID int, db *database.Database, w http.ResponseWriter, rows sql.Result, err error) (error, int) {
	for n, statement := range statements {
		if len(strings.TrimSpace(statement)) == 0 || statement[:1] == "#" {
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(statement)), "select ") {
			if err := readRowDataTx(db, statement, w); err != nil {
				return nil, util.ErrorResponse(w, db.Session.ID, "Error reading SQL query; "+filterErrorMessage(err.Error()), http.StatusInternalServerError)
			}
		} else {
			rows, err = db.Exec(statement)
			if err == nil {
				count, _ := rows.RowsAffected()

				ui.Log(ui.TableLogger, "sql.rows", ui.A{
					"session": sessionID,
					"count":   count})

				// If this is the last operation in the transaction, this is also our response
				// payload.
				if n == len(statements)-1 {
					reply := defs.DBRowCount{
						ServerInfo: util.MakeServerInfo(sessionID),
						Count:      int(count),
						Status:     http.StatusOK,
					}

					w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

					b, _ := json.MarshalIndent(reply, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
					_, _ = w.Write(b)
					db.Session.ResponseLength += len(b)

					if ui.IsActive(ui.RestLogger) {
						ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
							"session": db.Session.ID,
							"body":    string(b)})
					}

					break
				}
			}

			if err != nil {
				break
			}
		}
	}

	return err, http.StatusOK
}

// getStatementsFromRequest tries to parse the request body into an array of SQL statements. The body might be
// an array of strings, or a single string. In either case, it is returned to the caller as a simple array of strings.
// If there was an error in handling the request payload, an HTTP error status code is returned.
func getStatementsFromRequest(body string, w http.ResponseWriter, sessionID int) ([]string, int) {
	statements := []string{}

	err := json.Unmarshal([]byte(body), &statements)
	if err != nil {
		statement := ""

		if err := json.Unmarshal([]byte(body), &statement); err != nil {
			return nil, util.ErrorResponse(w, sessionID, "Invalid SQL payload: "+err.Error(), http.StatusBadRequest)
		}

		// The SQL could be multiple statements separated by a semicolon.  If so, we'd need to break the
		// code up into separate statements.
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

		// Did we end up having to further split the statements in the array? If so, replace the
		// array with the newly-split array.
		if wasResplit {
			statements = newStatements
		}

		// If we're doing REST logging, dump out the statement array we will execute now.
		if ui.IsActive(ui.RestLogger) {
			ui.WriteLog(ui.RestLogger, "rest.sql", ui.A{
				"session":    sessionID,
				"statements": statements})
		}
	}

	return statements, http.StatusOK
}

func readRowDataTx(db *database.Database, q string, w http.ResponseWriter) error {
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
			ServerInfo: util.MakeServerInfo(db.Session.ID),
			Rows:       result,
			Count:      len(result),
			Status:     http.StatusOK,
		}

		status := http.StatusOK

		w.Header().Add(defs.ContentTypeHeader, defs.RowSetMediaType)
		w.WriteHeader(status)

		b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		_, _ = w.Write(b)
		db.Session.ResponseLength += len(b)

		ui.Log(ui.TableLogger, "sql.read.rows", ui.A{
			"session": db.Session.ID,
			"rows":    rowCount,
			"columns": columnCount})

		if ui.IsActive(ui.RestLogger) {
			ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
				"session": db.Session.ID,
				"body":    string(b)})
		}
	}

	return err
}

func splitSQLStatements(s string) []string {
	result := []string{}

	t := tokenizer.New(s, false)
	next := ""

	for !t.AtEnd() {
		token := t.Next()
		if token.Is(tokenizer.SemicolonToken) {
			if len(strings.TrimSpace(next)) > 0 {
				result = append(result, next)
			}

			next = ""
		} else {
			if len(strings.TrimSpace(next)) > 0 {
				next = next + " "
			}

			if token.IsString() {
				next = next + strconv.Quote(token.Spelling())
			} else {
				next = next + token.Spelling()
			}
		}
	}

	if len(strings.TrimSpace(next)) > 0 {
		result = append(result, next)
	}

	return result
}
