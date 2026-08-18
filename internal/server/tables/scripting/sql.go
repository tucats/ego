package scripting

import (
	"net/http"
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/server/dberrors"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/sqlparse"
)

// doSQL handles the "sql" opcode. It executes the raw SQL statement in
// task.SQL -- SELECT included; unlike before, a SELECT is no longer
// promoted to the "readrows" opcode by Handler before dispatch, it is
// classified here instead (see authorizeAndClassifySQL) -- and returns the
// number of rows affected (or, for a SELECT, the number of rows read).
//
// Validation rules enforced before execution:
//   - task.Columns must be empty — raw SQL has no separate column list.
//   - task.Filters must be empty — filters are for opcode-driven queries only.
//   - task.SQL must be non-empty (after trimming whitespace).
//   - task.Table must be empty — the table name (if any) must be embedded
//     directly in the SQL string.
//
// Before execution, task.SQL is parsed (via authorizeAndClassifySQL) to
// determine its statement kind -- which decides both whether to run it as
// a query or an exec, and whether a schema-cache flush is warranted
// afterward -- and, for a non-admin caller, to authorize every table it
// references against that caller's table_perms/DSN-admin standing, the
// same way the top-level @sql endpoint does (tables/sql_permissions.go).
// Reaching this function at all already required defs.SQLPermission (or
// ego.root) -- see Handler's first pass -- matching @sql's own route-level
// gate, since @transaction's own route has no such gate on its own.
//
// If task.EmptyError is true and no rows were affected (or read), returns 404.
// Returns (rowCount, httpStatus, cacheFlush, error).
func doSQL(sessionID int, db *database.Database, task defs.TXOperation, id int, syms *symbolTable) (int, int, bool, error) {
	var count int

	if err := applySymbolsToTask(sessionID, &task, id, syms); err != nil {
		return count, http.StatusBadRequest, false, errors.New(err)
	}

	if len(task.Columns) > 0 {
		return count, http.StatusBadRequest, false, errors.ErrTaskSQLUnsupported.Context("columns")
	}

	if len(task.Filters) > 0 {
		return count, http.StatusBadRequest, false, errors.ErrTaskSQLUnsupported.Context("filters")
	}

	if len(strings.TrimSpace(task.SQL)) == 0 {
		return count, http.StatusBadRequest, false, errors.ErrTaskSQLMissing
	}

	if len(strings.TrimSpace(task.Table)) != 0 {
		return count, http.StatusBadRequest, false, errors.ErrTaskSQLUnsupported.Context("table name")
	}

	q := task.SQL

	kind, cacheFlush, status, err := authorizeAndClassifySQL(db, q)
	if err != nil {
		return count, status, cacheFlush, err
	}

	// A statement that failed to parse (only reachable here for an admin
	// caller, or a test with no session -- see authorizeAndClassifySQL)
	// has an unknown kind; fall back to the original text-prefix guess so
	// admins still get correct SELECT-vs-exec dispatch for SQL our parser
	// doesn't cover, matching the top-level @sql endpoint's identical
	// fallback in sql.go.
	isSelect := kind == sqlparse.StmtSelect
	if kind == sqlparse.StmtUnknown {
		isSelect = strings.HasPrefix(strings.TrimSpace(strings.ToLower(q)), "select ")
	}

	if isSelect {
		count, status, err = readTxRowResultSet(db, q, sessionID, syms, task.EmptyError)

		return count, status, cacheFlush, err
	}

	rows, err := db.Exec(q)
	if err == nil {
		if affectedCount, err := rows.RowsAffected(); err == nil {
			count = int(affectedCount)
		}

		if count == 0 && task.EmptyError {
			return count, http.StatusNotFound, cacheFlush, errors.ErrTableRowsNoChanges
		}

		ui.Log(ui.TableLogger, "table.affected", ui.A{
			"session": sessionID,
			"count":   count,
			"status":  http.StatusOK})

		return count, http.StatusOK, cacheFlush, nil
	}

	// Distinct from the already-fixed top-level tables/sql.go @sql handler --
	// this is the @transaction "sql" opcode, which had the same hardcoded
	// 400 bug. Same classifier, same reasoning as doDelete/doDrop in this
	// package: db.Exec has already run, so an unrecognized failure is a
	// server fault by default, and a recognized one (missing table,
	// constraint conflict) reports what it actually was.
	return count, dberrors.ExecStatus(err), cacheFlush, errors.New(err)
}
