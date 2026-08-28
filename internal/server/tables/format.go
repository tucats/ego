package tables

import (
	"io"
	"net/http"

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

	// The payload uses the same array-of-strings-or-single-string convention
	// as @sql (see getStatementsFromRequest in sql.go), including splitting a
	// single string containing multiple ";"-separated statements.
	statements, httpStatus := getStatementsFromRequest(body, w, session)
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
		p, err := sqlparse.New(stmt, dialect)
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

		formatted[i] = p.Format()
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
