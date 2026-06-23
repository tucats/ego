package tables

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
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

// TableCreate handler creates a new table based on the JSON payload, which must be an array of
// DBColumn objects, defining the characteristics of each column in the table. If the table name
// is the special name "@sql" the payload instead is assumed to be a JSON-encoded string containing
// arbitrary SQL to execute. Only an admin user can use the "@sql" table name.
func TableCreate(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	sessionID := session.ID
	user := session.User
	tableName := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])

	// Open the database connection. Pass the optional DSN if given as a part of the path. If a DSN is
	// provided, then it contains the credentials to connect to the database. Otherwise, the user info
	// associated with the session is used to authenticate with the database.
	db, err := GetDatabase(session, dsnName, dsns.DSNAdminAction)
	if err == nil && db != nil {
		// Amend any table name with the provider-appropriate user schema name.
		tableName, _ = parsing.FullName(db.Provider, session.User, tableName)

		// Make sure there isn't a cached version fo this schema.
		tableEntryKey := session.User
		if dsnName == "" {
			tableEntryKey += "-/" + tableName
		} else {
			tableEntryKey += dsnName + "/" + tableName
		}

		caches.Delete(caches.SchemaCache, tableEntryKey)

		// Verify that we are allowed to do this. The caller must either be a root user or
		// explicitly have update permission for the table.
		if !session.Admin && Authorized(session, user, tableName, defs.TableAdminPermission) {
			return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.perm.admin"), http.StatusForbidden)
		}

		// Create an array of column definitions which will receive the JSON payload from the
		// request.
		columns, httpStatus := getColumnPayload(r, w, session)
		if httpStatus > 200 {
			return httpStatus
		}

		// Generate the SQL string that will create the table.
		q, err := parsing.FormCreateQuery(r.URL, user, session.Admin, columns, sessionID, w, db.Provider, db.HasRowID)
		if err != nil {
			return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), http.StatusBadRequest)
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
			if !createSchemaIfNeeded(w, sessionID, db, user, tableName) {
				return http.StatusOK
			}

		default:
			return util.ErrorResponse(w, sessionID,
				errors.ErrUnsupportedDatabase.Context(db.Provider).Localize(session.Language),
				http.StatusBadRequest)
		}

		// Execute the SQL that creates the table. Also write to the log when SQLLogger is active.
		counts, err := db.Exec(q)
		if err == nil {
			// If the table create was successful, construct a response object to send back to the
			// client. For a table create, the response is a DBRowCount object.
			rows, _ := counts.RowsAffected()
			response := defs.DBRowCount{
				ServerInfo: util.MakeServerInfo(sessionID),
				Count:      int(rows),
				Status:     http.StatusOK,
			}

			_ = createTablePermissions(session, user, dsnName, tableName)

			tableName, _ = parsing.FullName(db.Provider, user, tableName)
			response.Message = i18n.T("msg.server.table.created", ui.A{"name": tableName})

			w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

			// Convert the response object to JSON, write it to the response, log it, and we're done.
			b := util.WriteJSON(w, response, &session.ResponseLength)

			if ui.IsActive(ui.RestLogger) {
				ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
					"session": sessionID,
					"body":    string(b)})
			}

			ui.Log(ui.TableLogger, "table.created", ui.A{
				"session": sessionID})

			return http.StatusOK
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

	return util.ErrorResponse(w, sessionID, errors.Localize(err, session.Language), http.StatusBadRequest)
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

// createSchemaIfNeeded ensures the user's schema exists in the database before creating
// a table.  This is only meaningful for providers that support named schemas.
//
// Returns true when the schema is confirmed to exist (or was created), false when the
// operation failed.  On failure the function writes an HTTP error response to w.
//
// To add support for a new provider: implement any required schema-creation DDL and add
// a case in the switch below.
func createSchemaIfNeeded(w http.ResponseWriter, sessionID int, db *database.Database, user string, tableName string) bool {
	switch db.Provider {
	case defs.SqliteProvider:
		// SQLite has no schema concept — every table lives in the same flat namespace.
		return true

	case defs.PostgresProvider:
		// PostgreSQL requires the schema to be created explicitly before the first table
		// in that schema is added.  Fall through to the creation logic below.

	default:
		// An unrecognized provider cannot proceed.  Write an error response and signal
		// failure to the caller so it stops further processing.
		util.ErrorResponse(w, sessionID,
			errors.ErrUnsupportedDatabase.Context(db.Provider).Localize(db.Session.Language),
			http.StatusBadRequest)

		return false
	}

	// Default schema is the current user. However, if the table name is a two-part name, use the first part
	// of the name as the schema.
	schema := user
	if dot := strings.Index(tableName, "."); dot >= 0 {
		schema = tableName[:dot]
	}

	// Construct the SQL query to create the schema, including using the schema name just determined.
	q, err := parsing.QueryParameters(createSchemaQuery, map[string]string{
		"schema": schema,
	})
	if err != nil {
		util.ErrorResponse(w, sessionID, i18n.Text(db.Session.Language, "error.table.schema.query", ui.A{"err": err.Error()}), http.StatusInternalServerError)

		return false
	}

	// Execute the SQL query to create the schema. If it fails, write an error response to the REST
	// payload and return indicating we could not or did not create a schema.
	result, err := db.Exec(q)
	if err != nil {
		util.ErrorResponse(w, sessionID, i18n.Text(db.Session.Language, "error.table.schema.create", ui.A{"err": err.Error()}), http.StatusInternalServerError)

		return false
	}

	// If successful, the result will be a rows affected, which should be 1 if the schema was created by
	// this operation, or zero if it already existed. If it was created, log this information.
	_, _ = result.RowsAffected()

	return true
}

func getColumnInfo(db *database.Database, tableName string, showRowID bool) ([]defs.DBColumn, error) {
	user := db.Session.User
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

			typeInfo := types[i]

			// Start by seeing what Go type it will become. If that isn't
			// known, then get the underlying database type name instead.
			typeName := ""
			if t := typeInfo.ScanType(); t != nil {
				typeName = t.Name()
			}

			if typeName == "" {
				typeName = typeInfo.DatabaseTypeName()
			}

			size, _ := typeInfo.Length()
			nullable, _ := typeInfo.Nullable()
			specified := true

			// Normalize provider-specific type name strings into the portable names that the
			// rest of the server uses (e.g. "timestamp", "string", "int").  Each database
			// driver reports column type names differently; we map them here so that all
			// downstream code can work with a single, consistent vocabulary.
			// To add a new provider: add a case with the driver's type name mapping.
			switch db.Provider {
			case defs.SqliteProvider:
				// The modernc SQLite driver reports column type names in upper-case and uses
				// several non-standard names.  Map every known variant to the portable form.
				// SQLite also does not support nullable column metadata via the Go sql
				// interface, so we override those fields to safe defaults.
				switch typeName {
				case "INT":
					typeName = "int"
					size = 8

				case "BOOL":
					typeName = "bool"

				case "INT32":
					typeName = "int32"
					size = 4

				case "INT16":
					typeName = "int16"
					size = 2

				case "BYTE":
					typeName = "byte"
					size = 1

				case "FLOAT":
					typeName = "float64"
					size = 8

				case "STRING":
					typeName = "string"

				case "NullInt64":
					typeName = "int64"
					size = 8

				case "NullFloat64":
					typeName = "float64"
					size = 8

				case "NullString":
					typeName = "string"

				// Time-related columns: MapColumnType now declares these with their semantic
				// names (TIMESTAMP, TIME, DATE) rather than TEXT, so the driver echoes those
				// names back during schema introspection.  Normalize all known variants
				// (including TIMESTAMPTZ and DATETIME which may appear in imported schemas)
				// to lowercase portable names so that CoerceToColumnType can recognize them.
				case "TIMESTAMP", "TIMESTAMPTZ", "DATETIME":
					typeName = "timestamp"

				case "TIME":
					typeName = "time"

				case "DATE":
					typeName = "date"
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
				case "Time", "TIMESTAMPTZ", "TIMESTAMP WITH TIME ZONE",
					"TIMESTAMP", "DATETIME":
					// "Time" is what Go's reflect package returns for time.Time; the others
					// are raw PostgreSQL DDL type names from DatabaseTypeName().
					typeName = "timestamp"

				case "TIME", "TIME WITH TIME ZONE":
					typeName = "time"

				case "DATE":
					typeName = "date"
				}

			default:
				// An unrecognized provider reached column introspection.
				// The metadata query selection above should already have returned an error
				// for an unknown provider, so this branch is not reachable in normal
				// operation.  If it is reached, stop immediately with a clear error.
				return nil, errors.ErrUnsupportedDatabase.Context(db.Provider)
			}

			columns = append(columns, defs.DBColumn{
				Name:     name,
				Type:     typeName,
				Size:     int(size),
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

// DeleteTable will delete a database table from the user's schema.
func DeleteTable(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	sessionID := session.ID
	user := session.User
	isAdmin := session.Admin
	table := data.String(session.URLParts["table"])
	dsnName := data.String(session.URLParts["dsn"])

	db, err := GetDatabase(session, dsnName, dsns.DSNAdminAction)
	if err == nil && db != nil {
		tableName, _ := parsing.FullName(db.Provider, user, table)

		if !isAdmin && dsnName == "" && !Authorized(session, user, tableName, defs.TableAdminPermission) {
			return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.perm.admin"), http.StatusForbidden)
		}

		q, err := parsing.QueryParameters(tableDeleteQuery, map[string]string{
			"table": tableName,
		})
		if err != nil {
			return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.table.delete.query", ui.A{"err": err.Error()}), http.StatusInternalServerError)
		}

		// When dropping a table via a DSN, the correct DROP TABLE syntax depends on
		// the provider.  SQLite has no schema concept, so the table name must not be
		// schema-qualified.  PostgreSQL keeps the schema-qualified name built earlier.
		// To add a new provider: add a case with the appropriate DROP TABLE template.
		if dsnName != "" {
			switch db.Provider {
			case defs.SqliteProvider:
				// DSN-backed SQLite table: strip the schema prefix so the DROP succeeds.
				tableName = table
				q, err = parsing.QueryParameters(`DROP TABLE "{{table}}";`, map[string]string{
					"table": tableName,
				})

				if err != nil {
					return util.ErrorResponse(w, sessionID, i18n.Text(session.Language, "error.db.operation"), http.StatusInternalServerError)
				}

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
			// Make sure there isn't a cached version fo this table's schema.
			tableEntryKey := session.User
			if dsnName == "" {
				tableEntryKey += "-/" + tableName
			} else {
				tableEntryKey += dsnName + "/" + tableName
			}

			caches.Delete(caches.SchemaCache, tableEntryKey)

			// Remove the table permissions for this table.
			if dsnName == "" {
				removeTablePermissions(session, tableName)
			}

			w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)
			w.WriteHeader(http.StatusOK)

			resp := defs.DBRowCount{
				ServerInfo: util.MakeServerInfo(sessionID),
				Count:      1,
				Status:     http.StatusOK,
				Message:    i18n.T("msg.server.table.deleted", ui.A{"name": tableName}),
			}

			b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
			w.Write(b)
			session.ResponseLength += len(b)

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

	status := http.StatusBadRequest
	if strings.Contains(detail, "does not exist") {
		status = http.StatusNotFound
	}

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
