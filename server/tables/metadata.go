package tables

// metadata.go implements GET /dsns/{dsnname}/@metadata, which returns a compact
// schema summary for every table in the named DSN. Each entry in the response
// lists the table name and an array of (column name, type) pairs.
//
// Paging is supported via ?start=N (1-based) and ?limit=N query parameters,
// matching the convention used by the existing table-list endpoints. The
// database does the pagination in SQL (LIMIT/OFFSET), so even large schemas
// are handled efficiently.

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/dsns"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
	"github.com/tucats/ego/util"
)

// DSNMetadataHandler handles GET /dsns/{dsnname}/@metadata.
//
// It opens the named DSN, queries the catalog for visible table names (with
// SQL-level paging applied), and for each table retrieves its column names and
// portable type strings. The result is returned as a DSNMetadataResponse.
//
// Authorization: the caller must have at least read-level access to the DSN.
// For secured DSNs, tables that the session user is not permitted to read are
// silently omitted from the response (matching the behavior of ListTablesHandler).
//
// Paging parameters:
//
//	?start=N   — 1-based index of the first table to return (default 1)
//	?limit=N   — maximum number of tables to include (default 1000)
//
// Both values are echoed back in the response body so the client knows which
// slice of the full catalog it received.
func DSNMetadataHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	dsnName := data.String(session.URLParts["dsn"])

	// Read the paging parameters early — we include them in the response body
	// regardless of whether any tables are found, so clients can always tell
	// which window they requested.
	start, limit := pagingFromURL(r.URL)

	// Open a database connection for the named DSN. DSNReadAction is the
	// minimum privilege required; it prevents unauthorized users from
	// enumerating the schema of a DSN they have no business accessing.
	db, err := GetDatabase(session, dsnName, dsns.DSNReadAction)
	if err != nil {
		return util.ErrorResponse(w, session.ID,
			i18n.T("error.db.list.error", ui.A{"err": err}),
			http.StatusBadRequest)
	}

	if db == nil {
		// GetDatabase should never return (nil, nil), but guard against it so
		// we get a clear 500 rather than a nil-dereference panic.
		return util.ErrorResponse(w, session.ID,
			i18n.T("error.db.nil.pointer"),
			http.StatusInternalServerError)
	}

	// Normalize the deprecated "sqlite3" provider alias to the canonical
	// "sqlite" name so the rest of the handler only compares against one string.
	if strings.EqualFold(db.Provider, defs.DeprecatedSqliteProvider) {
		db.Provider = defs.SqliteProvider
	}

	ui.Log(ui.TableLogger, "table.dsn.metadata.start", ui.A{
		"session":  session.ID,
		"dsn":      dsnName,
		"provider": db.Provider,
		"start":    start,
		"limit":    limit,
	})

	// Fetch the table names for this DSN, applying LIMIT/OFFSET in SQL so the
	// database does the paging work rather than loading all names into memory.
	tableNames, httpStatus, err := listTableNamesForMetadata(db, session, r)
	if err != nil {
		return util.ErrorResponse(w, session.ID, err.Error(), httpStatus)
	}

	// For each table name, retrieve its column descriptors. If a table
	// disappears between the list query and the column query (e.g., a race
	// with a DROP TABLE), we log the failure and continue — one bad table
	// should not abort the entire metadata response.
	items := make([]defs.TableMetadata, 0, len(tableNames))

	for _, tableName := range tableNames {
		columns, colErr := getColumnInfo(db, tableName, false /* omit internal _row_id_ column */)
		if colErr != nil {
			// Log the failure with enough detail for an operator to diagnose
			// it, but do not propagate the error to the caller. The caller
			// receives the successfully-fetched tables; skipped tables are
			// silently absent.
			ui.Log(ui.TableLogger, "table.dsn.metadata.column.error", ui.A{
				"session": session.ID,
				"table":   tableName,
				"error":   colErr.Error(),
			})

			continue
		}

		// Convert the full DBColumn descriptors to the lighter MetadataField
		// structs that this endpoint exposes. We intentionally omit size,
		// nullable, and unique flags here — those details are available from
		// the per-table GET /dsns/{dsn}/tables/{table} endpoint for callers
		// that need them.
		fields := make([]defs.MetadataField, 0, len(columns))

		for _, col := range columns {
			fields = append(fields, defs.MetadataField{
				Name: col.Name,
				Type: col.Type,
			})
		}

		items = append(items, defs.TableMetadata{
			Table:  tableName,
			Fields: fields,
		})
	}

	ui.Log(ui.TableLogger, "table.dsn.metadata.count", ui.A{
		"session": session.ID,
		"count":   len(items),
		"start":   start,
	})

	response := defs.DSNMetadataResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		Provider:   db.Provider,
		Start:      start,
		Count:      len(items),
		Items:      items,
		Status:     http.StatusOK,
		Message: i18n.T("msg.server.dsn.metadata", ui.A{
			"dsn":   dsnName,
			"count": len(items),
		}),
	}

	w.Header().Set(defs.ContentTypeHeader, defs.DSNMetadataMediaType)

	b := util.WriteJSON(w, response, &session.ResponseLength)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b),
		})
	}

	return http.StatusOK
}

// listTableNamesForMetadata queries the database catalog for the names of all
// tables visible to the session user, applying LIMIT/OFFSET paging from the
// request URL at the SQL level.
//
// For secured DSNs, tables that the current user is not permitted to read are
// excluded, matching the behavior of the existing ListTablesHandler.
//
// Returns the unqualified table names (no schema prefix), the HTTP status to
// use if an error is returned, and the error itself.
func listTableNamesForMetadata(db *database.Database, session *router.Session, r *http.Request) ([]string, int, error) {
	var (
		q      string
		params []any
		schema string
	)

	// Build a provider-appropriate catalog query. Each database system exposes
	// its table list through a different system catalog interface.
	//
	// To add support for a new provider: add a case here with the catalog
	// query appropriate for that provider. The query must return one column
	// containing the unqualified table name.
	switch db.Provider {
	case defs.SqliteProvider:
		// SQLite uses sqlite_schema (formerly sqlite_master). It has no schema
		// concept, so no parameter is needed.
		q = "SELECT name FROM sqlite_schema WHERE (type='table' OR type='view') "
		schema = ""

	case defs.PostgresProvider:
		// PostgreSQL: select all user tables in the schema that matches the
		// session user. The tablesListQuery constant (defined in defs.go)
		// returns names ordered alphabetically, which gives deterministic
		// paging results.
		q = tablesListQuery
		params = []any{session.User}
		schema = session.User

	default:
		return nil, http.StatusBadRequest,
			errors.ErrUnsupportedDatabase.Context(db.Provider)
	}

	// Append LIMIT / OFFSET from the URL query parameters. PagingClauses
	// reads ?start= and ?limit= (with sensible defaults) and produces the
	// appropriate SQL clause. Pushing pagination into SQL avoids loading
	// the full table list into memory when a DSN has many tables.
	if paging := parsing.PagingClauses(r.URL); paging != "" {
		q += paging
	}

	ui.Log(ui.TableLogger, "table.schema.tables", ui.A{
		"session": session.ID,
		"schema":  schema,
	})

	rows, err := db.Query(q, params...)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	defer rows.Close()

	names := make([]string, 0)

	for rows.Next() {
		var name string

		if err = rows.Scan(&name); err != nil {
			return nil, http.StatusInternalServerError, err
		}

		// Authorization gate: skip any table the current user is not
		// permitted to read. Authorized() returns true when the DSN is not
		// secured (open access), when the user is a server administrator,
		// or when an explicit read-permission record exists for this user
		// and table. This mirrors the check in ListTablesHandler.
		if !session.Admin && !Authorized(session, session.User, name, defs.TableReadPermission) {
			continue
		}

		names = append(names, name)
	}

	return names, http.StatusOK, nil
}

// pagingFromURL extracts the 1-based start index and the per-page limit from
// URL query parameters, using the same parameter names as PagingClauses so
// the two functions always agree.
//
// Defaults: start=1, limit=parsing.DefaultRowLimit (1000).
//
// We parse the values ourselves here (rather than calling PagingClauses) because
// we need the numeric values to echo them back in the response body — PagingClauses
// only returns a SQL fragment.
func pagingFromURL(u *url.URL) (start, limit int) {
	start = 1
	limit = parsing.DefaultRowLimit

	if u == nil {
		return
	}

	for k, v := range u.Query() {
		if len(v) != 1 {
			continue
		}

		i, convErr := strconv.Atoi(v[0])
		if convErr != nil || i <= 0 {
			continue
		}

		switch {
		case parsing.KeywordMatch(k, "start", "offset"):
			start = i
		case parsing.KeywordMatch(k, "limit", "count"):
			limit = i
		}
	}

	return
}
