package scripting

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/server/dberrors"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/server/tables/parsing"
)

// doRows handles the "readrows" opcode. It runs a SELECT (or other
// row-returning) query and stores the complete result set — all matching
// rows — as a single symbol under the key resultSetSymbolName. After the
// transaction commits, Handler detects that key and returns its value as
// the HTTP response body (a defs.DBRowSet), rather than returning the
// plain row-count response.
//
// The SQL query is taken from task.SQL when present (e.g. for a raw
// "select …" statement, authorized via authorizeAndClassifySQL the same
// way doSQL's "sql" opcode is). Otherwise it is built from task.Table,
// task.Filters, and task.Columns just like doSelect, and authorized via
// authorizedForTable instead. A raw "sql" opcode whose text is a SELECT is
// no longer promoted to this opcode by Handler -- doSQL classifies and
// handles that case directly now (see sql.go).
//
// Unlike doSelect, any number of rows is valid. If task.EmptyError is true and
// zero rows are returned, the operation fails with 404.
func doRows(sessionID int, user string, db *database.Database, task defs.TXOperation, id int, syms *symbolTable) (int, int, error) {
	var (
		err    error
		count  int
		status int
	)

	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return 0, http.StatusBadRequest, err
	}

	// Captured after applySymbolsToTask (not before, as this used to read)
	// so that a {{name}} reference inside a raw SQL string is actually
	// expanded -- q used to be captured from task.SQL before symbol
	// substitution ran, so it silently ignored the substituted value and
	// used the original, unexpanded text instead.
	q := task.SQL

	if q == "" {
		if !authorizedForTable(db, task.Table, defs.TableReadPermission) {
			return 0, http.StatusForbidden, errors.ErrNoPrivilegeForOperation.Context(task.Table)
		}

		fakeURL, _ := url.Parse("http://localhost/tables/" + task.Table + "/rows")

		q, err = parsing.FormSelectorDeleteQuery(fakeURL, task.Filters, strings.Join(task.Columns, ","), task.Table, user, selectVerb, db.Provider)
		if err != nil {
			return count, http.StatusBadRequest, errors.Message(filterErrorMessage(q))
		}
	} else {
		// Raw SQL text supplied directly (task.SQL, rather than the
		// structured Table/Filters/Columns fields) -- authorize every
		// table it references, the same way doSQL does for the "sql"
		// opcode; this is the other opcode that can run client-supplied
		// SQL text. Gated at Handler's first pass on defs.SQLPermission
		// before either handler is ever reached.
		if _, _, status, err := authorizeAndClassifySQL(db, q); err != nil {
			return 0, status, err
		}
	}

	count, status, err = readTxRowResultSet(db, q, sessionID, syms, task.EmptyError)
	if err == nil {
		return count, status, nil
	}

	ui.Log(ui.TableLogger, "table.read.error", ui.A{
		"session": sessionID,
		"sql":     q,
		"error":   err})

	return 0, status, errors.New(err)
}

// readTxRowResultSet executes query q, collects every row into a
// []map[string]any slice, and stores the slice under resultSetSymbolName in
// the symbol table. Each map represents one row, keyed by column name.
//
// Any previous result set stored under resultSetSymbolName is deleted before
// the new query runs — there can be at most one result set per transaction.
//
// emptyResultError controls whether zero rows is treated as an error:
//   - true  → 404 + ErrTableNoRows
//   - false → success; the stored slice is empty but present
//
// If the query itself fails, or a row fails to scan partway through the
// result set, status is classified via dberrors.ExecStatus and the error is
// wrapped and returned; in either case the result set is not stored.
func readTxRowResultSet(db *database.Database, q string, sessionID int, syms *symbolTable, emptyResultError bool) (int, int, error) {
	var (
		rows     *sql.Rows
		err      error
		rowCount int
		result   = []map[string]any{}
		status   = http.StatusOK
	)

	// If the symbol table doesn't exist, create it. If it does, delete any
	// previous result set (to quote the Highlander, "there can be only one.")
	if syms == nil || len(syms.symbols) == 0 {
		*syms = symbolTable{symbols: map[string]any{}}
	} else {
		delete(syms.symbols, resultSetSymbolName)
	}

	rows, err = db.Query(q)
	if err == nil {
		defer rows.Close()

		columnNames, _ := rows.Columns()
		columnCount := len(columnNames)

		for rows.Next() {
			row := make([]any, columnCount)
			rowPointers := make([]any, columnCount)

			for i := range row {
				rowPointers[i] = &row[i]
			}

			if err = rows.Scan(rowPointers...); err != nil {
				// A scan failure partway through the result set means the
				// remaining rows cannot be trusted either. Stop immediately
				// rather than continuing to call rows.Next(): looping on
				// would let a later row's successful Scan overwrite err back
				// to nil, making this failure disappear entirely by the time
				// the loop ends (REST-3 7.4) -- silently reporting 200 for a
				// read that actually failed partway through.
				break
			}

			newRow := map[string]any{}
			for i, v := range row {
				newRow[columnNames[i]] = v
			}

			result = append(result, newRow)
			rowCount++
		}

		if err != nil {
			// Exec-stage failure: the query already ran, so an unrecognized
			// cause defaults to 500, not 400 -- same classifier every other
			// opcode in this package uses for a post-Exec/Query failure.
			status = dberrors.ExecStatus(err)
		} else {
			syms.symbols[resultSetSymbolName] = result

			ui.Log(ui.TableLogger, "table.read", ui.A{
				"session": sessionID,
				"rows":    rowCount,
				"columns": columnCount,
				"status":  status})
		}
	} else {
		status = dberrors.ExecStatus(err)
	}

	if err == nil && rowCount == 0 && emptyResultError {
		return rowCount, http.StatusNotFound, errors.ErrTableNoRows
	}

	if err != nil {
		err = errors.New(err)
	}

	return rowCount, status, err
}
