package tables

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/dberrors"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/sqlparse"
	"github.com/tucats/ego/internal/util"
)

// FormatSQL formats each of the SQL statements the client sent to @format
// using sqlparse's Format(), the same parser and dialect-rewriting/
// schema-qualification steps @sql (SQLTransaction, sql_permissions.go)
// applies to a statement before executing it. Unlike @sql, FormatSQL never
// opens a transaction or touches a row, so a caller only needs
// defs.SQLPermission to reach it -- there is no table_perms/DSNAdminPermission
// check here, since formatting text a caller already wrote themselves grants
// no access to data they could not already see.
func FormatSQL(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	sessionID := session.ID

	ui.Log(ui.TableLogger, "table.format", ui.A{
		"session": sessionID})

	var body string

	if b, err := io.ReadAll(r.Body); err == nil && b != nil {
		body = string(b)
	} else {
		return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.sql.payload.empty"), http.StatusBadRequest)
	}

	statements, httpStatus := getFormatStatementsFromRequest(body, w, session)
	if httpStatus > http.StatusOK {
		return httpStatus
	}

	// Only DSNReadAction is required: formatting never reads or writes a
	// row, it just needs the DSN's Provider/Schema/RestrictSchema to know
	// which dialect and schema to format each statement for.
	db, err := database.Open(session, data.String(session.URLParts["dsn"]), dsns.DSNReadAction)
	if err != nil {
		return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), dberrors.PayloadStatus(err))
	}

	defer db.Close()

	dialect := sqlDialect(db.Provider)
	formatted := make([]string, len(statements))

	for i, stmt := range statements {
		// Pull whole "--" and "//" comment lines out before parsing -- the
		// parser has no notion of a comment that isn't attached to real SQL
		// (see sqlparse's own "syntax only" design goal), and these need to
		// survive verbatim in the response rather than being silently
		// dropped or rejected as a syntax error. See extractLineComments's
		// own doc comment for the exact rule and its limitations.
		code, comments := extractLineComments(stmt)

		if strings.TrimSpace(code) == "" {
			formatted[i] = strings.Join(comments, "\n")

			continue
		}

		p, err := sqlparse.New(code, dialect)
		if err != nil {
			return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), http.StatusBadRequest)
		}

		// Normalize whichever dialect's generated-key, WITHOUT ROWID, or
		// INSERT OR ... syntax the client wrote to match db's own dialect,
		// same as @sql does (see authorizeAndFormatStatements's doc comment
		// in sql_permissions.go).
		if _, err := p.Rewrite(uniqueKeyLookup(session, db)); err != nil {
			return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), http.StatusBadRequest)
		}

		if db.Provider == defs.PostgresProvider {
			if db.RestrictSchema {
				if err := p.RestrictToSchema(db.User); err != nil {
					return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), http.StatusForbidden)
				}
			}

			p.QualifyTables(db.User)
		}

		formattedCode := p.Format()

		if len(comments) > 0 {
			formatted[i] = strings.Join(comments, "\n") + "\n" + formattedCode
		} else {
			formatted[i] = formattedCode
		}
	}

	response := defs.SQLFormatResponse{
		ServerInfo: util.MakeServerInfo(sessionID),
		Text:       formatted,
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.SQLFormatMediaType)

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": sessionID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// getFormatStatementsFromRequest decodes @format's request body into an
// array of SQL text to format, one entry per requested statement. Unlike
// @sql's getStatementsFromRequest (sql.go), a single request-body string is
// not further split on ";" into multiple statements, and no comment text is
// ever stripped here -- each array entry (or the lone string) is handed
// back exactly as the caller wrote it, including any comment lines, so
// FormatSQL can find and preserve them itself (see extractLineComments).
// Callers wanting several statements formatted independently should submit
// a JSON array rather than relying on ";"-splitting of one string.
func getFormatStatementsFromRequest(body string, w http.ResponseWriter, session *router.Session) ([]string, int) {
	statements := []string{}

	if err := json.Unmarshal([]byte(body), &statements); err != nil {
		var statement string

		if err := json.Unmarshal([]byte(body), &statement); err != nil {
			return nil, util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.sql.payload.invalid", ui.A{"err": err.Error()}), http.StatusBadRequest)
		}

		statements = []string{statement}
	}

	return statements, http.StatusOK
}

// extractLineComments pulls every whole "--" or "//" comment line out of
// stmt, returning the remaining SQL text (comment lines removed) plus the
// comment lines themselves, verbatim and in their original relative order.
// "//" is an Ego-specific extension to standard SQL comment syntax, matching
// the same convention splitSQLStatements (sql.go) already strips as an
// "Ego-style comment line" elsewhere.
//
// A line only counts as a comment here if the marker is the first
// non-blank thing on it -- an inline "-- ..." or "// ..." trailing actual
// SQL on the same line is left in the code text (and handled, or not, by
// whatever sqlparse itself does with it) rather than being pulled out,
// since chopping a code line in half at the marker would corrupt it.
//
// Every extracted comment is placed ahead of the statement's formatted SQL
// in the caller's output, in the order it appeared in stmt, regardless of
// whether it appeared before, after, or in the middle of the original code
// -- sqlparse's Format() rebuilds a statement's text structure from its
// parsed AST, so there is no reliable line correspondence between the
// original code and the reformatted code to interleave a mid-statement
// comment back into.
func extractLineComments(stmt string) (code string, comments []string) {
	lines := strings.Split(stmt, "\n")
	codeLines := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "//") {
			comments = append(comments, line)

			continue
		}

		codeLines = append(codeLines, line)
	}

	return strings.Join(codeLines, "\n"), comments
}
