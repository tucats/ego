package tables

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

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
	"github.com/tucats/ego/internal/util"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// Provider-reported column type names. These are the raw strings each database
// driver returns from schema introspection (ScanType().Name() or
// DatabaseTypeName()); normalizeColumnType maps them to the portable canonical
// type names declared further below.
const (
	// SqliteType* names are reported by the modernc SQLite driver.
	SqliteTypeInt         = "INT"
	SqliteTypeBool        = "BOOL"
	SqliteTypeBoolean     = "BOOLEAN"
	SqliteTypeInt32       = "INT32"
	SqliteTypeInt16       = "INT16"
	SqliteTypeByte        = "BYTE"
	SqliteTypeFloat       = "FLOAT"
	SqliteTypeString      = "STRING"
	SqliteTypeNullInt64   = "NullInt64"
	SqliteTypeNullFloat64 = "NullFloat64"
	SqliteTypeNullString  = "NullString"
	SqliteTypeTimestamp   = "TIMESTAMP"
	SqliteTypeTimestampTZ = "TIMESTAMPTZ"
	SqliteTypeDatetime    = "DATETIME"
	SqliteTypeTime        = "TIME"
	SqliteTypeDate        = "DATE"

	// PostgresReflectTypeTime is the Go reflect type name lib/pq's ScanType()
	// returns for every date/time-shaped column; it collapses DATE, TIME,
	// TIMETZ, TIMESTAMP, and TIMESTAMPTZ down to this single name, so it can't
	// by itself say which of those a column is -- see the DatabaseTypeName()
	// switch in normalizeColumnType that disambiguates it.
	PostgresReflectTypeTime = "Time"

	// PostgresType* names are lib/pq's DatabaseTypeName() (DDL-style) names.
	PostgresTypeDate                  = "DATE"
	PostgresTypeTime                  = "TIME"
	PostgresTypeTimeTZ                = "TIMETZ"
	PostgresTypeTimeWithTimeZone      = "TIME WITH TIME ZONE"
	PostgresTypeTimestamp             = "TIMESTAMP"
	PostgresTypeTimestampTZ           = "TIMESTAMPTZ"
	PostgresTypeTimestampWithTimeZone = "TIMESTAMP WITH TIME ZONE"
	PostgresTypeDatetime              = "DATETIME"
	PostgresTypeFloat4                = "FLOAT4"
	PostgresTypeFloat8                = "FLOAT8"
)

// Canonical portable column type names used once a provider-specific type name
// has been normalized. These overlap conceptually with the *TypeName constants
// in the data package (data.IntTypeName, data.BoolTypeName, etc.), which
// normalizeColumnType uses directly where they apply; the date/time names below
// have no equivalent there.
const (
	CanonicalTimestamp = "timestamp"
	CanonicalTime      = "time"
	CanonicalDate      = "date"
)

// TableCreate handler creates a new table based on the JSON payload, which must be an array of
// DBColumn objects, defining the characteristics of each column in the table.
//
// The "@sql" pseudo-table name is handled by a separate handler, SQLTransaction,
// registered on its own static route (PUT/POST /tables/@sql) that takes
// precedence over this one's wildcard {{table}} route -- this function never
// sees that name and has no special-case logic for it. (This comment used to
// say otherwise; it hasn't been true since SQLTransaction was split out.)
// Every successful call here is therefore a genuine "create the table named
// in the URL" operation, which is why its success response can unconditionally
// report 201 Created with a Location header.
func TableCreate(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	sessionID := session.ID
	user := session.User
	table := data.String(session.URLParts["table"])
	tableName := table
	dsnName := data.String(session.URLParts["dsn"])

	// Open the database connection. Pass the optional DSN if given as a part of the path. If a DSN is
	// provided, then it contains the credentials to connect to the database. Otherwise, the user info
	// associated with the session is used to authenticate with the database.
	db, err := GetDatabase(session, dsnName, dsns.DSNAdminAction)
	if err == nil && db != nil {
		defer db.Close()

		// Amend any table name with the provider-appropriate schema name
		// (the DSN's configured schema, not the Ego identity -- see db.User's
		// doc comment in database/open.go).
		tableName, _ = parsing.FullName(db.Provider, db.User, tableName)

		// DATA-SECURITY-2.md findings #2 and #3: there used to be an extra
		// permission check here, calling Authorized(session, user,
		// tableName, defs.TableAdminPermission) to ask "does the caller
		// have an ego.table.admin grant in table_perms for this table?"
		// before letting the create proceed. It was removed, not fixed, for
		// two reasons that both point the same way:
		//
		//  1. Go note for readers new to the language: "tableName" at this
		//     point in the function is NOT the same string as the "table"
		//     variable declared at the top -- it was just reassigned two
		//     lines up, by parsing.FullName(), to a *provider-qualified*
		//     name (for example "myschema.orders" on PostgreSQL, or just a
		//     quoted "orders" on SQLite, with no DSN name anywhere in it).
		//     Authorized() (security.go) expects its table argument in
		//     "dsn.table" form -- it splits on the first "." to figure out
		//     which DSN's permissions to check -- so passing the
		//     provider-qualified name meant Authorized() was asking about a
		//     DSN that does not exist (an empty name on SQLite, or the
		//     database schema name mistaken for a DSN name on PostgreSQL).
		//     That lookup always failed, so Authorized() always returned
		//     false, for every caller who was not session.Admin (literal
		//     ego.root). No non-root caller could ever create a table.
		//
		//  2. Even if that argument had been fixed to the correct
		//     "dsnName+"."+table" form (the pattern used everywhere else in
		//     this package -- see rows.go, list.go, describe.go), the check
		//     still could not have worked as intended: table_perms only
		//     gets a row for a table *after* it is created (see
		//     createTablePermissions, called a little further down, once
		//     the CREATE TABLE below actually succeeds). Asking "does the
		//     caller already hold ego.table.admin on this not-yet-existing
		//     table" can only ever answer "no" for a restricted DSN, for
		//     literally any caller. A per-table grant is the wrong tool for
		//     authorizing the act of creating the table in the first place.
		//
		// The check this function actually needs is "does the caller have
		// admin standing on the DSN itself" -- and that check already
		// happened, correctly, a few lines up: GetDatabase(session,
		// dsnName, dsns.DSNAdminAction) only returned a non-nil db here
		// because the caller is session.Admin, holds identity-wide
		// ego.dsn.admin, holds a DSN-specific dsns_auth admin grant for
		// dsnName, or the DSN is unrestricted (in which case docs/SERVER.md
		// says Ego imposes no access control of its own at all). That is
		// exactly the same DSN-level authorization every other
		// schema-changing operation in this codebase requires -- compare
		// sql_permissions.go's UsageAdmin branch and
		// scripting/authz.go's authorizedForDDL, neither of which consults
		// table_perms either, for the identical reason. Nothing more needs
		// checking here; the route registration in routes.go was updated to
		// match (see its comment for the other half of this fix).

		// Create an array of column definitions which will receive the JSON payload from the
		// request.
		columns, httpStatus := getColumnPayload(r, w, session)
		if httpStatus > 200 {
			return httpStatus
		}

		// Generate the SQL string that will create the table.
		q, err := parsing.FormCreateQuery(r.URL, db.User, session.Admin, columns, db.Provider, db.HasRowID)
		if err != nil {
			// FormCreateQuery no longer writes its own response (REST-3
			// 7.5) -- classify the returned error the same way every other
			// payload-stage failure in this handler does, so
			// ErrNoPrivilegeForOperation correctly reports 403 instead of
			// being flattened to 400.
			return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), dberrors.PayloadStatus(err))
		}

		if q == "" {
			return http.StatusOK
		}

		// Create the user schema in the database if it does not yet exist.
		// SQLite has no schema concept, so this step is skipped for SQLite.
		// For an unrecognized provider the helper returns false and writes an
		// error response; we stop here in that case.
		switch db.Provider {
		case defs.SqliteProvider:
			// SQLite: all tables share one flat namespace — no schema creation needed.

		case defs.PostgresProvider:
			if ok, status := createSchemaIfNeeded(w, sessionID, db, db.User, tableName); !ok {
				return status
			}

		default:
			return util.ErrorResponse(w, sessionID,
				errors.ErrUnsupportedDatabase.Context(db.Provider).Localize(session.Language),
				http.StatusBadRequest)
		}

		// Execute the SQL that creates the table. Also write to the log when SQLLogger is active.
		counts, err := db.Exec(q)
		if err == nil {
			// A prior table by this name may have left cached schema info behind (e.g. it was
			// dropped and is now being recreated with different columns), so flush the schema
			// cache to make sure no one reads stale column info for the new table.
			caches.Purge(caches.SchemaCache)

			// If the table create was successful, construct a response object to send back to the
			// client. For a table create, the response is a DBRowCount object.
			rows, _ := counts.RowsAffected()
			response := defs.DBRowCount{
				ServerInfo: util.MakeServerInfo(sessionID),
				Count:      int(rows),
				Status:     http.StatusCreated,
			}

			// Use the raw, unqualified table name (not the FullName-qualified
			// tableName above) -- that's what GrantPermissions/ReadPermissions/
			// ReadTablePermissions store and filter table_perms.table by (see
			// removeTablePermissions's identical note in DeleteTable below). Before
			// this fix, the creator's own auto-grant was stored under the quoted
			// name (e.g. `"mytable"`) and so never matched a later lookup by any
			// of those three handlers -- it was silently unreachable.
			_ = createTablePermissions(session, user, dsnName, table)

			tableName, _ = parsing.FullName(db.Provider, db.User, tableName)
			response.Message = i18n.T("msg.server.table.created", ui.A{"name": tableName})

			w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

			// A successful create reports 201, not 200, with a Location header
			// naming the new table's own URL (RFC 9110 §10.2.2). This is a PUT
			// to the exact URL that now identifies the table, so the request's
			// own path already is that location -- GET on this same path
			// returns the table just created (ReadTable).
			w.Header().Set(defs.LocationHeader, r.URL.Path)

			// Convert the response object to JSON, write it to the response, log it, and we're done.
			b := util.WriteJSON(w, session.Response(), http.StatusCreated, response)

			if ui.IsActive(ui.RestLogger) {
				ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
					"session": sessionID,
					"body":    string(b)})
			}

			ui.Log(ui.TableLogger, "table.created", ui.A{
				"session": sessionID})

			return http.StatusCreated
		}

		ui.Log(ui.TableLogger, "table.query.error", ui.A{
			"session": sessionID,
			"query":   q,
			"error":   err.Error()})

		return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), http.StatusBadRequest)
	}

	// We got here because we failed to open the database connection.
	ui.Log(ui.TableLogger, "table.write.error", ui.A{
		"session": sessionID,
		"error":   strings.TrimPrefix(err.Error(), "pq: ")})

	if err == nil {
		err = errors.ErrGeneric
	}

	// A DSN named in the URL that does not exist is a 404 (REST-2).
	return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), dberrors.PayloadStatus(err))
}

func getColumnPayload(r *http.Request, w http.ResponseWriter, session *router.Session) ([]defs.DBColumn, int) {
	columns := []defs.DBColumn{}

	// Read the body of the request and decode the JSON as an array of DBColumn objects.
	// If the payload has an ill-formed JSON string, return the error.
	if err := json.NewDecoder(r.Body).Decode(&columns); err != nil {
		return nil, util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.create.payload", ui.A{"err": err.Error()}), http.StatusBadRequest)
	}

	// Validate the column definitions, which must have a name and valid type.
	for _, column := range columns {
		if column.Name == "" {
			return nil, util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.column.name.empty"), http.StatusBadRequest)
		}

		if column.Type == "" {
			return nil, util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.type.name.empty"), http.StatusBadRequest)
		}

		if !parsing.KeywordMatch(column.Type, defs.TableColumnTypeNames...) {
			return nil, util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.type.name.invalid", ui.A{"name": column.Type}), http.StatusBadRequest)
		}
	}

	return columns, 0
}

// ensureSchemaExists is createSchemaIfNeeded's PostgreSQL-only "CREATE SCHEMA
// IF NOT EXISTS <schema>" step, without the http.ResponseWriter dependency --
// for a caller like scripting's "sql" opcode (doSQL, sql.go in that package)
// that reports failure through a returned error instead of writing a
// response directly, and so cannot call createSchemaIfNeeded itself. See
// that function's own doc comment for why PostgreSQL needs this at all.
func ensureSchemaExists(db *database.Database, schema string) error {
	q, err := parsing.QueryParameters(createSchemaQuery, map[string]string{
		"schema": schema,
	})
	if err != nil {
		return err
	}

	_, err = db.Exec(q)

	return err
}

// createSchemaIfNeeded ensures the user's schema exists in the database before creating
// a table.  This is only meaningful for providers that support named schemas.
//
// Returns true when the schema is confirmed to exist (or was created), false when the
// operation failed.  On failure the function writes an HTTP error response to w and
// returns the status code it wrote, so the caller can propagate that same status
// instead of reporting 200 on top of an error response already sent to the client.
//
// To add support for a new provider: implement any required schema-creation DDL and add
// a case in the switch below.
func createSchemaIfNeeded(w http.ResponseWriter, sessionID int, db *database.Database, user string, tableName string) (bool, int) {
	switch db.Provider {
	case defs.SqliteProvider:
		// SQLite has no schema concept — every table lives in the same flat namespace.
		return true, http.StatusOK

	case defs.PostgresProvider:
		// PostgreSQL requires the schema to be created explicitly before the first table
		// in that schema is added.  Fall through to the creation logic below.

	default:
		// An unrecognized provider cannot proceed.  Write an error response and signal
		// failure to the caller so it stops further processing.
		status := util.ErrorResponse(w, sessionID,
			errors.ErrUnsupportedDatabase.Context(db.Provider).Localize(db.Session.Language),
			http.StatusBadRequest)

		return false, status
	}

	// Default schema is the DSN's configured schema (db.User). However, if the
	// table name is a two-part name, use the first part of the name as the schema.
	schema := user
	if dot := strings.Index(tableName, "."); dot >= 0 {
		schema = tableName[:dot]
	}

	// Construct the SQL query to create the schema, including using the schema name just determined.
	q, err := parsing.QueryParameters(createSchemaQuery, map[string]string{
		"schema": schema,
	})
	if err != nil {
		status := util.ErrorResponse(w, sessionID, i18n.Text(db.Session.Language, "error.table.schema.query", ui.A{"err": err.Error()}), http.StatusInternalServerError)

		return false, status
	}

	// Execute the SQL query to create the schema. If it fails, write an error response to the REST
	// payload and return indicating we could not or did not create a schema.
	result, err := db.Exec(q)
	if err != nil {
		status := util.ErrorResponse(w, sessionID, i18n.Text(db.Session.Language, "error.table.schema.create", ui.A{"err": err.Error()}), http.StatusInternalServerError)

		return false, status
	}

	// If successful, the result will be a rows affected, which should be 1 if the schema was created by
	// this operation, or zero if it already existed. If it was created, log this information.
	_, _ = result.RowsAffected()

	return true, http.StatusOK
}

func getColumnInfo(db *database.Database, tableName string, showRowID bool) ([]defs.DBColumn, error) {
	user := db.User
	columns := make([]defs.DBColumn, 0)
	name, _ := parsing.FullName(db.Provider, user, tableName)

	// Is it in our cache already? Form a unique key from the user identity, dsn, and
	// table name.
	cacheKey := user + "/"
	if db.DSN == "" {
		cacheKey += "-/" + name
	} else {
		cacheKey += db.DSN + "/" + name
	}

	// Also have to account for the showRowID flag
	cacheKey += "/"
	if showRowID {
		cacheKey += "rowid"
	}

	if cached, ok := caches.Find(caches.SchemaCache, cacheKey); ok {
		if columns, ok := cached.([]defs.DBColumn); ok {
			return columns, nil
		}
	}

	// Choose the metadata query for the target provider.
	// Each provider exposes column type information via a different catalogue interface.
	// To add a new provider: define a query template and add a case here.
	var metadataQueryTemplate string

	switch db.Provider {
	case defs.SqliteProvider:
		metadataQueryTemplate = tableSQLiteMetadataQuery

	case defs.PostgresProvider:
		metadataQueryTemplate = tableMetadataQuery

	default:
		return nil, errors.ErrUnsupportedDatabase.Context(db.Provider)
	}

	q, err := parsing.QueryParameters(metadataQueryTemplate, map[string]string{
		"table": name,
	})
	if err != nil {
		return nil, errors.New(errors.ErrTableQueryBuild).Context(err.Error())
	}

	rows, err := db.Query(q)
	if err == nil {
		defer rows.Close()

		names, _ := rows.Columns()
		types, _ := rows.ColumnTypes()

		for i, name := range names {
			// Special case, we synthetically create a defs.RowIDName column
			// and it is always of type "UUID". But we don't return it
			// as a user column name.
			if name == defs.RowIDName && !showRowID {
				continue
			}

			typeName, size, nullable, specified, typeErr := normalizeColumnType(db.Provider, types[i])
			if typeErr != nil {
				return nil, typeErr
			}

			columns = append(columns, defs.DBColumn{
				Name:     name,
				Type:     typeName,
				Size:     size,
				Nullable: defs.BoolValue{Specified: specified, Value: nullable}},
			)
		}
	}

	if err != nil {
		return columns, errors.New(err)
	} else {
		caches.Add(caches.SchemaCache, cacheKey, columns)
	}

	return columns, nil
}

// normalizeColumnType converts a database/sql column type descriptor into the portable
// type vocabulary used throughout the server (e.g. "timestamp", "string", "int"). Each
// database driver reports column type names differently; this maps provider-specific
// quirks to that shared vocabulary so callers such as parsing.CoerceToColumnType can work
// with a single, consistent set of names regardless of which provider produced the row.
// To add a new provider: add a case with the driver's type name mapping.
func normalizeColumnType(provider string, typeInfo *sql.ColumnType) (typeName string, size int, nullable bool, specified bool, err error) {
	// Start by seeing what Go type it will become. If that isn't
	// known, then get the underlying database type name instead.
	if t := typeInfo.ScanType(); t != nil {
		typeName = t.Name()
	}

	if typeName == "" {
		typeName = typeInfo.DatabaseTypeName()
	}

	length, _ := typeInfo.Length()
	size = int(length)
	nullable, _ = typeInfo.Nullable()
	specified = true

	switch provider {
	case defs.SqliteProvider:
		// The modernc SQLite driver reports column type names in upper-case and uses
		// several non-standard names.  Map every known variant to the portable form.
		// SQLite also does not support nullable column metadata via the Go sql
		// interface, so we override those fields to safe defaults.
		switch typeName {
		case SqliteTypeInt:
			typeName = data.IntTypeName
			size = 8

		case SqliteTypeBool, SqliteTypeBoolean:
			typeName = data.BoolTypeName

		case SqliteTypeInt32:
			typeName = data.Int32TypeName
			size = 4

		case SqliteTypeInt16:
			typeName = data.Int16TypeName
			size = 2

		case SqliteTypeByte:
			typeName = data.ByteTypeName
			size = 1

		case SqliteTypeFloat:
			typeName = data.Float64TypeName
			size = 8

		case SqliteTypeString:
			typeName = data.StringTypeName

		case SqliteTypeNullInt64:
			typeName = data.Int64TypeName
			size = 8

		case SqliteTypeNullFloat64:
			typeName = data.Float64TypeName
			size = 8

		case SqliteTypeNullString:
			typeName = data.StringTypeName

		// Time-related columns: MapColumnType now declares these with their semantic
		// names (TIMESTAMP, TIME, DATE) rather than TEXT, so the driver echoes those
		// names back during schema introspection.  Normalize all known variants
		// (including TIMESTAMPTZ and DATETIME which may appear in imported schemas)
		// to lowercase portable names so that CoerceToColumnType can recognize them.
		case SqliteTypeTimestamp, SqliteTypeTimestampTZ, SqliteTypeDatetime:
			typeName = CanonicalTimestamp

		case SqliteTypeTime:
			typeName = CanonicalTime

		case SqliteTypeDate:
			typeName = CanonicalDate
		}

		nullable = false
		specified = false

	case defs.PostgresProvider:
		// PostgreSQL normalization.  The lib/pq driver returns either the Go reflect
		// type name (from ScanType().Name()) or the PostgreSQL-dialect DDL name (from
		// DatabaseTypeName()).  In practice, ScanType().Name() for TIMESTAMP WITH TIME
		// ZONE columns returns "Time" (the Go type name), while DatabaseTypeName()
		// returns "TIMESTAMPTZ".  We normalize both to the portable lowercase names.
		switch typeName {
		case PostgresReflectTypeTime:
			// lib/pq's ScanType() collapses every date/time-shaped Postgres OID
			// (DATE, TIME, TIMETZ, TIMESTAMP, TIMESTAMPTZ) to this single Go
			// reflect type name (rows.go's scanType map), so "Time" alone can't
			// tell a DATE column from a TIMESTAMP one -- naively mapping it
			// straight to "timestamp" (as this used to) silently misreported
			// every DATE and TIME column's DescribeTable response as
			// "timestamp". DatabaseTypeName() still returns the real,
			// undisambiguated DDL name (e.g. "DATE") for these OIDs, unlike the
			// FLOAT4/FLOAT8 case below where ScanType() itself returns nil and
			// DatabaseTypeName() is already what populated typeName above --
			// so it's consulted directly here instead.
			switch typeInfo.DatabaseTypeName() {
			case PostgresTypeDate:
				typeName = CanonicalDate
			case PostgresTypeTime, PostgresTypeTimeTZ:
				typeName = CanonicalTime
			default: // TIMESTAMP, TIMESTAMPTZ, and anything else time.Time-shaped.
				typeName = CanonicalTimestamp
			}

		case PostgresTypeTimestampTZ, PostgresTypeTimestampWithTimeZone, PostgresTypeTimestamp, PostgresTypeDatetime:
			typeName = CanonicalTimestamp

		case PostgresTypeTime, PostgresTypeTimeWithTimeZone:
			typeName = CanonicalTime

		case PostgresTypeDate:
			typeName = CanonicalDate

		// lib/pq's ScanType() returns nil for REAL and DOUBLE PRECISION columns
		// (unlike BOOL, INT4, and VARCHAR, which it resolves to concrete Go
		// types), so typeName falls back to DatabaseTypeName() above and arrives
		// here as the raw Postgres OID type name rather than a Go reflect name.
		case PostgresTypeFloat4:
			typeName = data.Float32TypeName

		case PostgresTypeFloat8:
			typeName = data.Float64TypeName
		}

	default:
		// An unrecognized provider reached column introspection.  The caller should
		// already have rejected an unknown provider earlier, so this branch is not
		// reachable in normal operation.  If it is reached, stop immediately with a
		// clear error rather than guessing at type semantics.
		err = errors.ErrUnsupportedDatabase.Context(provider)
	}

	return typeName, size, nullable, specified, err
}

// DeleteTable will delete a database table from the user's schema.
func DeleteTable(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	sessionID := session.ID
	table := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])

	db, err := GetDatabase(session, dsnName, dsns.DSNAdminAction)
	if err == nil && db != nil {
		defer db.Close()

		tableName, _ := parsing.FullName(db.Provider, db.User, table)

		// DATA-SECURITY-2.md findings #2 and #3: this used to have an extra
		// check here -- "if !isAdmin && dsnName == "" && ..." -- guarding an
		// Authorized(session, user, tableName, defs.TableAdminPermission)
		// call. It was removed for the same reasons TableCreate's
		// equivalent check was removed just above in this file (see that
		// function's much longer comment for the full explanation); the
		// short version:
		//
		// The route this handler is registered on (routes.go) is
		// "/dsns/{{dsn}}/tables/{{table}}" -- the "{{dsn}}" URL segment is
		// mandatory for a request to match this route at all, so
		// "dsnName == """" is not a real, reachable case for a request that
		// actually got routed here; this was dead code. And even if it
		// somehow were reached, the Authorized() call it guarded had the
		// same bug as TableCreate's: tableName here is the
		// provider-qualified name from parsing.FullName() a couple of
		// lines up, not the "dsn.table" form Authorized() expects, so the
		// check could never succeed for a non-admin caller anyway.
		//
		// The actual authorization for deleting a table already happened
		// above, in GetDatabase(session, dsnName, dsns.DSNAdminAction):
		// only a caller with DSN-level admin standing (identity-wide,
		// DSN-specific, or an unrestricted DSN) gets a non-nil db back at
		// all. That is the correct and sufficient check -- dropping a table
		// is a schema change, and every other schema-changing operation in
		// this codebase (see sql_permissions.go's UsageAdmin branch and
		// scripting/authz.go's authorizedForDDL) is authorized at the DSN
		// level too, never against a per-table table_perms grant.

		// Note the deliberate use of a separate name here rather than ":=" on
		// "err". A ":=" would declare a *second* err scoped to this block, and
		// the "_, err = db.Exec(q)" below would then assign to that inner copy.
		// The outer err would still be nil when execution falls out of this
		// block, and the error-reporting code at the end of the function --
		// which calls err.Error() -- would dereference nil and panic. Dropping
		// a table that does not exist did exactly that, and the server's panic
		// recovery turned it into a generic 500 (REST-1).
		q, queryErr := parsing.QueryParameters(tableDeleteQuery, map[string]string{
			"table": tableName,
		})
		if queryErr != nil {
			return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.table.delete.query", ui.A{"err": queryErr.Error()}), http.StatusInternalServerError)
		}

		// When dropping a table via a DSN, the correct DROP TABLE syntax depends on
		// the provider.  SQLite has no schema concept, so the table name must not be
		// schema-qualified.  PostgreSQL keeps the schema-qualified name built earlier.
		// To add a new provider: add a case with the appropriate DROP TABLE template.
		if dsnName != "" {
			switch db.Provider {
			case defs.SqliteProvider:
				// DSN-backed SQLite table: strip the schema prefix so the DROP succeeds.
				// Quote the raw (unqualified) table name directly as a SQL identifier
				// instead of substituting it into a double-quote-delimited template via
				// QueryParameters/SQLEscape: SQLEscape only rejects embedded "'" and ";"
				// characters, not '"', so a table name containing a '"' could otherwise
				// break out of the template's own quoting and inject arbitrary SQL.
				tableName = table
				q = "DROP TABLE " + egostrings.SQLIdentifier(tableName) + ";"

			case defs.PostgresProvider:
				// PostgreSQL with a DSN: the schema-qualified query built above is correct.

			default:
				return util.ErrorResponse(w, sessionID,
					errors.ErrUnsupportedDatabase.Context(db.Provider).Localize(session.Language),
					http.StatusBadRequest)
			}
		}

		_, err = db.Exec(q)
		if err == nil {
			// Make sure there isn't a cached version of this table's schema -- otherwise a
			// future CREATE TABLE reusing this name would inherit the dropped table's stale
			// cached column info.
			caches.Purge(caches.SchemaCache)

			// Remove the table permissions for this table. This uses the raw,
			// unqualified table name (not the FullName-qualified tableName
			// above), because that's what GrantPermissions/ReadPermissions
			// store in table_perms.table -- and it must run regardless of
			// dsnName, since dsn is a required path segment here and "" just
			// means the default/baseline DSN slot, not "no DSN".
			removeTablePermissions(session, table)

			w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

			resp := defs.DBRowCount{
				ServerInfo: util.MakeServerInfo(sessionID),
				Count:      1,
				Status:     http.StatusOK,
				Message:    i18n.T("msg.server.table.deleted", ui.A{"name": tableName}),
			}

			// WriteJSON issues the status itself, so no WriteHeader call belongs above.
			b := util.WriteJSON(w, session.Response(), http.StatusOK, resp)

			if ui.IsActive(ui.RestLogger) {
				ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
					"session": sessionID,
					"body":    string(b)})
			}

			ui.Log(ui.TableLogger, "table.deleted", ui.A{
				"name":    tableName,
				"session": sessionID})

			return resp.Status
		}
	}

	if err == nil && db == nil {
		return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.db.nil.pointer"), http.StatusInternalServerError)
	}

	detail := strings.TrimPrefix(err.Error(), "pq: ")
	ui.Log(ui.TableLogger, "table.delete.error", ui.A{
		"session": sessionID,
		"error":   detail})

	// Dropping a table that does not exist is a 404. This used to match only
	// PostgreSQL's wording, so SQLite answered 400 for the same case (REST-1).
	status := dberrors.PayloadStatus(err)

	return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.table.delete.error"), status)
}

func parameterString(r *http.Request) string {
	m := r.URL.Query()
	result := strings.Builder{}

	for k, v := range m {
		if result.Len() == 0 {
			result.WriteRune('?')
		} else {
			result.WriteRune('&')
		}

		result.WriteString(k)

		if len(v) > 0 {
			result.WriteRune('=')

			for n, value := range v {
				if n > 0 {
					result.WriteRune(',')
				}

				result.WriteString(value)
			}
		}
	}

	return result.String()
}
