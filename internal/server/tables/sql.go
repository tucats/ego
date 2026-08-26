package tables

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tucats/ego/internal/caches"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/dberrors"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/server/tables/parsing"
	"github.com/tucats/ego/internal/sqlparse"
	"github.com/tucats/ego/internal/util"
)

// SQLTransaction executes a series of SQL statements from the REST client and attempts
// to execute them all as a single transaction. The payload can be an array of strings,
// each of which is executed as a single statement within the transaction, or the payload
// can be a single string which is parsed using a ";" separator into individual statements.
//
// If there are multiple statements, then only the last statement in the payload can be a
// SELECT statements as that will be the result of the request.
func SQLTransaction(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		body       string
		rows       sql.Result
		sessionID  = session.ID
		cacheFlush bool
	)

	// This API call changed from a PUT to a POST on 2024-06-05. The old PUT is still
	// supported for backward compatibility, but it is deprecated and will be removed
	// in a future release. Put out a warning log message to that effect.
	if r.Method == http.MethodPut {
		ui.Log(ui.ServerLogger, "table.tx.deprecated", ui.A{
			"session": sessionID,
			"method":  r.Method,
			"from":    r.Header.Get("User-Agent"),
			"path":    r.URL.Path})
	}

	ui.Log(ui.TableLogger, "table.tx", ui.A{
		"session": sessionID})

	if b, err := io.ReadAll(r.Body); err == nil && b != nil {
		body = string(b)
	} else {
		return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.sql.payload.empty"), http.StatusBadRequest)
	}

	// The payload can be either a single string, or a sequence of strings in an array.
	// So try to decode as an array, but if that fails, try as a single string. Note that
	// we have to re-use the body that was previously read because the r.Body() reader has
	// been exhausted already.
	statements, httpStatus := getStatementsFromRequest(body, w, session)
	if httpStatus > http.StatusOK {
		return httpStatus
	}

	// We always do this under control of a transaction, so set that up now. This
	// also tells us the DSN's Provider (sqlite vs. postgres), which the next step
	// needs to parse and format each statement for the right dialect.
	db, err := database.Open(session, data.String(session.URLParts["dsn"]), dsns.DSNWriteAction+dsns.DSNReadAction)
	if err != nil {
		// A DSN that does not exist is a 404, not a server fault (REST-2).
		return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), dberrors.PayloadStatus(err))
	} else {
		defer db.Close()
	}

	// Parse each statement, enforce table_perms/DSN-admin authorization for a
	// non-admin caller, and rewrite each statement through the formatter so its
	// identifier quoting matches db's dialect (see sql_permissions.go). This
	// also gives us each statement's primary verb (kinds) so the "SELECT must
	// be last" rule below can use the parsed statement, not a text guess.
	statements, kinds, httpStatus := authorizeAndFormatStatements(session, db, statements, w)
	if httpStatus > http.StatusOK {
		return httpStatus
	}

	// Sanity check -- if there is a SELECT in the transaction, it must be the last item in the
	// array since there's no way to retain the result set otherwise. A statement that failed to
	// parse (only possible for an admin caller -- see authorizeAndFormatStatements) has an unknown
	// kind here; fall back to the original text-prefix guess for that one statement so admins keep
	// the same protection non-admins get from the parser.
	for n, kind := range kinds {
		isSelect := kind == sqlparse.StmtSelect
		if kind == sqlparse.StmtUnknown {
			isSelect = strings.HasPrefix(strings.TrimSpace(strings.ToLower(statements[n])), "select ")
		}

		if isSelect && n < len(kinds)-1 {
			return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.sql.select.last"), http.StatusBadRequest)
		}
	}

	err = db.Begin()
	if err != nil {
		return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	// Now execute each statement from the array of strings.
	err, cacheFlush, httpStatus = executeStatements(statements, kinds, sessionID, db, w, rows, err)
	if httpStatus > http.StatusOK {
		_ = db.Rollback()

		return httpStatus
	}

	if err != nil {
		_ = db.Rollback()

		// This used to look for PostgreSQL's "does not exist" wording, so a
		// missing table answered 404 against PostgreSQL but fell through to 500
		// against SQLite, which says "no such table" -- the exact mirror of
		// the bug in rowsAbstract.go. It also treated any error mentioning
		// "constraint" as a 409, which swept up NOT NULL and CHECK violations
		// that are really the client sending a bad value (REST-1).
		status := dberrors.ExecStatus(err)

		return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.sql.execute.error", ui.A{"err": filterErrorMessage(err.Error())}), status)
	} else {
		if err = db.Commit(); err != nil {
			_ = db.Rollback()

			return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.sql.commit.error", ui.A{"err": filterErrorMessage(err.Error())}), http.StatusInternalServerError)
		} else {
			// Everything went well including the commit, so if the SQL modified any schemas, now is the time
			// to flush the schema cache and let it rebuild by re-reading directly from the database.
			if cacheFlush {
				caches.Purge(caches.SchemaCache)
			}
		}
	}

	return http.StatusOK
}

// executeStatements executes each of the SQL statements in the provided array and returns the first error encountered. Note that if the array contains
// a SELECT statement, it must be the last item in the array since there's no way to retain the result set otherwise. If there is a select statement,
// the response payload is the result set from the SELECT statement. Otherwise, the response payload is the row count from the operations.
//
// kinds is the parsed StatementKind for each entry in statements, from authorizeAndFormatStatements
// (sql_permissions.go) -- SQLTransaction has already paid the cost of parsing every statement there,
// so reusing that result here to decide cacheFlush is free. kinds[n] is sqlparse.StmtUnknown for a
// statement that failed to parse (only possible for an admin caller); isSchemaAlteringKind's default
// case treats that the same as any other non-DDL kind, matching the pre-existing (and still correct)
// assumption that a caller cannot alter schema with SQL not even Ego's own parser recognizes.
func executeStatements(statements []string, kinds []sqlparse.StatementKind, sessionID int, db *database.Database, w http.ResponseWriter, rows sql.Result, err error) (error, bool, int) {
	startTime := time.Now()
	rowsAffected := 0
	cacheFlush := false

	for n, statement := range statements {
		// Is this a statement that can change a table's column layout,
		// primary/unique-key structure, or its very existence? If so, set
		// the flag saying we are a candidate for flushing the table schema
		// cache (see isSchemaAlteringKind's doc comment in sql_permissions.go).
		if isSchemaAlteringKind(kinds[n]) {
			cacheFlush = true
		}

		tokens := strings.Fields(strings.TrimSpace(strings.ToLower(statement)))
		if len(tokens) > 0 && tokens[0] == "select" {
			if err := readRowDataTx(db, statement, startTime, w); err != nil {
				// Unlike the query paths Ego builds itself, the statement here
				// came from the client, so a missing table is a 404 rather
				// than a server fault (REST-1).
				return nil, false, util.ErrorResponse(w, db.Session.ID, i18n.Text(db.Session.Language, "error.sql.query.read", ui.A{"err": filterErrorMessage(err.Error())}), dberrors.ExecStatus(err))
			}
		} else {
			rows, err = db.Exec(statement)
			if err == nil {
				count, _ := rows.RowsAffected()
				rowsAffected += int(count)

				ui.Log(ui.TableLogger, "sql.rows", ui.A{
					"session": sessionID,
					"count":   count})

				// If this is the last operation in the transaction, this is also our response
				// payload.
				if n == len(statements)-1 {
					response := defs.DBRowCount{
						ServerInfo: util.MakeServerInfo(sessionID),
						Count:      rowsAffected,
						Status:     http.StatusOK,
						Elapsed:    time.Since(startTime).String(),
					}

					w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

					b := util.WriteJSON(w, db.Session.Response(), http.StatusOK, response)

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

	return err, cacheFlush, http.StatusOK
}

// getStatementsFromRequest tries to parse the request body into an array of SQL statements. The body might be
// an array of strings, or a single string. In either case, it is returned to the caller as a simple array of strings.
// If there was an error in handling the request payload, an HTTP error status code is returned.
func getStatementsFromRequest(body string, w http.ResponseWriter, session *router.Session) ([]string, int) {
	statements := []string{}
	sessionID := session.ID

	err := json.Unmarshal([]byte(body), &statements)
	if err != nil {
		statement := ""

		if err := json.Unmarshal([]byte(body), &statement); err != nil {
			return nil, util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.sql.payload.invalid", ui.A{"err": err.Error()}), http.StatusBadRequest)
		}

		// The SQL could be multiple statements separated by a semicolon. If so, we'd need to break the
		// code up into separate statements.
		statements = splitSQLStatements(statement)
	} else {
		// it's possible that an array was sent but the string values may contain
		// multiple statements. If so, we need to break them up again.
		newStatements := make([]string, 0)

		for _, statement := range statements {
			splitStatements := splitSQLStatements(statement)
			newStatements = append(newStatements, splitStatements...)
		}

		statements = newStatements

		// If we're doing REST logging, dump out the statement array we will execute now.
		if ui.IsActive(ui.RestLogger) {
			ui.WriteLog(ui.RestLogger, "rest.sql", ui.A{
				"session":    sessionID,
				"statements": statements})
		}
	}

	return statements, http.StatusOK
}

func readRowDataTx(db *database.Database, q string, startTime time.Time, w http.ResponseWriter) error {
	var (
		rows     *sql.Rows
		err      error
		rowCount int
		result   = []map[string]any{}
	)

	rows, err = db.Query(q)
	if err == nil {
		defer rows.Close()

		columnNames, _ := rows.Columns()
		columnCount := len(columnNames)

		// Introspect the actual result-set column types (as opposed to a table's
		// catalog metadata, which may not exist for an arbitrary query) so that
		// driver-native representations -- such as SQLite storing "bool" columns
		// as integers -- are normalized to their portable Ego types below. If type
		// introspection is unavailable, coercion is skipped and raw driver values
		// are returned, same as before this normalization was added.
		var columns []defs.DBColumn

		if typeData, typeErr := rows.ColumnTypes(); typeErr == nil {
			columns = make([]defs.DBColumn, columnCount)

			for i, ct := range typeData {
				typeName, size, nullable, specified, normErr := normalizeColumnType(db.Provider, ct)
				if normErr != nil {
					return normErr
				}

				columns[i] = defs.DBColumn{
					Name:     columnNames[i],
					Type:     typeName,
					Size:     size,
					Nullable: defs.BoolValue{Specified: specified, Value: nullable},
				}
			}
		}

		for rows.Next() {
			row := make([]any, columnCount)
			rowPointers := make([]any, columnCount)

			for i := range row {
				rowPointers[i] = &row[i]
			}

			err = rows.Scan(rowPointers...)
			if err == nil {
				newRow := map[string]any{}

				for i, v := range row {
					if v != nil && columns != nil {
						v, err = parsing.CoerceToColumnType(columnNames[i], v, columns)
						if err != nil {
							return err
						}
					}

					newRow[columnNames[i]] = v
				}

				result = append(result, newRow)
				rowCount++
			}
		}

		pkey, ptable := analyzeSingleTableSelect(db.Session, db, q, columnNames)

		response := defs.DBRowSet{
			ServerInfo: util.MakeServerInfo(db.Session.ID),
			Columns:    columnNames,
			PKey:       pkey,
			PTable:     ptable,
			Rows:       result,
			Count:      len(result),
			Status:     http.StatusOK,
			Elapsed:    time.Since(startTime).String(),
		}

		status := http.StatusOK

		w.Header().Add(defs.ContentTypeHeader, defs.RowSetMediaType)
		// The status is not sent here: util.WriteJSON below issues it, because it may
		// first need to add a Content-Encoding header, and headers set after
		// WriteHeader() are silently discarded.

		b := util.WriteJSON(w, db.Session.Response(), status, response)

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

// splitSQLStatements splits a string containing one or more SQL statements into
// individual statement strings using a SQL-aware scanner. The scanner correctly
// handles the following cases so that semicolons inside them are never treated as
// statement terminators:
//
//   - Single-quoted string literals: 'hello world', 'O”Brian' (SQL ” escape)
//   - Double-quoted identifiers:     "my col", "my""col" (SQL "" escape)
//   - SQL line comments:             -- this is a comment
//   - SQL block comments:            /* this is a comment */
//
// Lines that start with '#' or '//' (Ego-style comments) are stripped before
// scanning. Blank lines are also stripped. Each returned statement is trimmed of
// leading and trailing whitespace; empty statements are omitted.
func splitSQLStatements(s string) []string {
	// Strip blank lines and Ego-style comment lines (#, //).
	lines := strings.Split(s, "\n")
	kept := lines[:0]

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}

		kept = append(kept, line)
	}

	s = strings.Join(kept, "\n")

	// Walk the SQL rune-by-rune. Track context (inside a string, identifier, or
	// comment) so that ';' is only recognized as a statement separator when it
	// appears outside all of those contexts.
	var (
		result         []string
		current        strings.Builder
		inSingleQuote  bool
		inDoubleQuote  bool
		inLineComment  bool
		inBlockComment bool
	)

	runes := []rune(s)
	n := len(runes)

	for i := 0; i < n; i++ {
		ch := runes[i]

		switch {
		// ── inside a SQL line comment (-- ... \n) ────────────────────────────
		case inLineComment:
			if ch == '\n' {
				inLineComment = false
				// The newline itself acts as whitespace between tokens; write it
				// so that multi-statement strings separated only by a newline work.
				current.WriteRune(ch)
			}
			// characters inside the comment are discarded

		// ── inside a SQL block comment (/* ... */) ───────────────────────────
		case inBlockComment:
			if ch == '*' && i+1 < n && runes[i+1] == '/' {
				inBlockComment = false
				i++ // consume the closing '/'
			}
			// characters inside the comment are discarded

		// ── inside a single-quoted string literal ('...') ────────────────────
		case inSingleQuote:
			current.WriteRune(ch)

			if ch == '\'' {
				// SQL escaped single-quote: two consecutive apostrophes stay inside
				// the string literal; a lone apostrophe closes it.
				if i+1 < n && runes[i+1] == '\'' {
					current.WriteRune('\'')

					i++ // consume the second apostrophe
				} else {
					inSingleQuote = false
				}
			}

		// ── inside a double-quoted identifier ("...") ────────────────────────
		case inDoubleQuote:
			current.WriteRune(ch)

			if ch == '"' {
				// SQL escaped double-quote: two consecutive double-quotes stay inside
				// the identifier; a lone double-quote closes it.
				if i+1 < n && runes[i+1] == '"' {
					current.WriteRune('"')

					i++ // consume the second double-quote
				} else {
					inDoubleQuote = false
				}
			}

		// ── normal context: look for the start of each special form ───────────
		case ch == '\'':
			inSingleQuote = true

			current.WriteRune(ch)

		case ch == '"':
			inDoubleQuote = true

			current.WriteRune(ch)

		case ch == '-' && i+1 < n && runes[i+1] == '-':
			// Start of a SQL line comment; skip the '--' and everything that follows
			// on this line.
			inLineComment = true
			i++ // consume the second '-'

		case ch == '/' && i+1 < n && runes[i+1] == '*':
			// Start of a SQL block comment.
			inBlockComment = true
			i++ // consume the '*'

		case ch == ';':
			// Statement separator — save what we have so far.
			if stmt := strings.TrimSpace(current.String()); stmt != "" {
				result = append(result, stmt)
			}

			current.Reset()

		default:
			current.WriteRune(ch)
		}
	}

	// Capture any trailing statement that has no closing semicolon.
	if stmt := strings.TrimSpace(current.String()); stmt != "" {
		result = append(result, stmt)
	}

	return result
}
