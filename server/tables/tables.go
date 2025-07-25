package tables

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
	"github.com/tucats/ego/util"
)

const unexpectedNilPointerError = "Unexpected nil database object pointer"

// TableCreate handler creates a new table based on the JSON payload, which must be an array of
// DBColumn objects, defining the characteristics of each column in the table. If the table name
// is the special name "@sql" the payload instead is assumed to be a JSON-encoded string containing
// arbitrary SQL to execute. Only an admin user can use the "@sql" table name.
func TableCreate(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	sessionID := session.ID
	user := session.User
	tableName := data.String(session.URLParts["table"])

	// Open the database connection. Pass the optional DSN if given as a part of the path. If a DSN is
	// provided, then it contains the credentials to connect to the database. Otherwise, the user info
	// associated with the session is used to authenticate with the database.
	db, err := database.Open(session, data.String(session.URLParts["dsn"]), dsns.DSNAdminAction)
	if err == nil && db != nil {
		// Unless we're using sqlite, add explicit schema to the table name.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(user, tableName)
		}

		// Verify that we are allowed to do this. The caller must either be a root user or
		// explicitly have update permission for the table.
		if !session.Admin && Authorized(db, user, tableName, updateOperation) {
			return util.ErrorResponse(w, sessionID, "User does not have update permission", http.StatusForbidden)
		}

		// Create an array of column definitions which will receive the JSON payload from the
		// request.
		columns, httpStatus := getColumnPayload(r, w, sessionID)
		if httpStatus > 200 {
			return httpStatus
		}

		// Generate the SQL string that will create the table.
		q, err := parsing.FormCreateQuery(r.URL, user, session.Admin, columns, sessionID, w, db.Provider, db.HasRowID)
		if err != nil {
			return util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
		}

		if q == "" {
			return http.StatusOK
		}

		// If the provider isn't SQLite, create a schema in the database for the current user if it
		// does not already exist.
		if db.Provider != sqlite3Provider {
			if !createSchemaIfNeeded(w, sessionID, db, user, tableName) {
				return http.StatusOK
			}
		}

		// Execute the SQL that creates the table. Also write to the log when SQLLogger is active.
		counts, err := db.Exec(q)
		if err == nil {
			// If the table create was successful, construct a response object to send back to the
			// client. For a table create, the response is a DBRowCount object.
			rows, _ := counts.RowsAffected()
			result := defs.DBRowCount{
				ServerInfo: util.MakeServerInfo(sessionID),
				Count:      int(rows),
				Status:     http.StatusOK,
			}

			tableName, _ = parsing.FullName(user, tableName)
			result.Message = "Table " + tableName + " created successfully"

			// Create a table permissions for the newly created table. Because the requestor created
			// the table, they are automatically assigned read, delete, and update permissions.
			CreateTablePermissions(sessionID, db, user, tableName, readOperation, deleteOperation, updateOperation)
			w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

			// Convert the response object to JSON, write it to the response, log it, and we're done.
			b, _ := json.MarshalIndent(result, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
			_, _ = w.Write(b)
			session.ResponseLength += len(b)

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

		return util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
	}

	// We got here because we failed to open the database connection.
	ui.Log(ui.TableLogger, "table.write.error", ui.A{
		"session": sessionID,
		"error":   strings.TrimPrefix(err.Error(), "pq: ")})

	if err == nil {
		err = fmt.Errorf("unknown error")
	}

	return util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
}

func getColumnPayload(r *http.Request, w http.ResponseWriter, sessionID int) ([]defs.DBColumn, int) {
	columns := []defs.DBColumn{}

	// Read the body of the request and decode the JSON as an array of DBColumn objects.
	// If the payload has an ill-formed JSON string, return the error.
	if err := json.NewDecoder(r.Body).Decode(&columns); err != nil {
		return nil, util.ErrorResponse(w, sessionID, "Invalid table create payload: "+err.Error(), http.StatusBadRequest)
	}

	// Validate the column definitions, which must have a name and valid type.
	for _, column := range columns {
		if column.Name == "" {
			return nil, util.ErrorResponse(w, sessionID, "Missing or empty column name", http.StatusBadRequest)
		}

		if column.Type == "" {
			return nil, util.ErrorResponse(w, sessionID, "Missing or empty type name", http.StatusBadRequest)
		}

		if !parsing.KeywordMatch(column.Type, defs.TableColumnTypeNames...) {
			return nil, util.ErrorResponse(w, sessionID, "Invalid type name: "+column.Type, http.StatusBadRequest)
		}
	}

	return columns, 0
}

// Verify that the schema exists for this user, and create it if not found. This is required for
// databases like Postgres that require explicit schema creation.
func createSchemaIfNeeded(w http.ResponseWriter, sessionID int, db *database.Database, user string, tableName string) bool {
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
		util.ErrorResponse(w, sessionID, "Error constructing schema creation query; "+err.Error(), http.StatusInternalServerError)

		return false
	}

	// Execute the SQL query to create the schema. If it fails, write an error response to the REST
	// payload and return indicating we could not or did not create a schema.
	result, err := db.Exec(q)
	if err != nil {
		util.ErrorResponse(w, sessionID, "Error creating schema; "+err.Error(), http.StatusInternalServerError)

		return false
	}

	// If successful, the result will be a rows affected, which should be 1 if the schema was created by
	// this operation, or zero if it already existed. If it was created, log this information.
	_, _ = result.RowsAffected()

	return true
}

func getColumnInfo(db *database.Database, tableName string) ([]defs.DBColumn, error) {
	user := db.Session.User
	columns := make([]defs.DBColumn, 0)
	name, _ := parsing.FullName(user, tableName)

	q, err := parsing.QueryParameters(tableMetadataQuery, map[string]string{
		"table": name,
	})
	if err != nil {
		return nil, fmt.Errorf("error constructing table metadata query; %w", err)
	}

	if db.Provider == "sqlite3" {
		q, err = parsing.QueryParameters(tableSQLiteMetadataQuery, map[string]string{
			"table": name,
		})
		if err != nil {
			return nil, fmt.Errorf("error constructing table SQLite metadata query; %w", err)
		}
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
			if name == defs.RowIDName {
				continue
			}

			typeInfo := types[i]

			// Start by seeing what Go type it will become. If that isn't
			// known, then get the underlying database type name instead.
			typeName := typeInfo.ScanType().Name()
			if typeName == "" {
				typeName = typeInfo.DatabaseTypeName()
			}

			size, _ := typeInfo.Length()
			nullable, _ := typeInfo.Nullable()
			specified := true

			// SQLite3 has some funky names, so handle them here.
			if db.Provider == "sqlite3" {
				switch typeName {
				case "NullInt64":
					typeName = "int64"
					size = 8

				case "NullFloat64":
					typeName = "float64"
					size = 8
				case "NullString":
					typeName = "string"
				}

				nullable = false
				specified = false
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
	}

	return columns, nil
}

// DeleteTable will delete a database table from the user's schema.
func DeleteTable(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	sessionID := session.ID
	user := session.User
	isAdmin := session.Admin
	table := data.String(session.URLParts["table"])
	tableName, _ := parsing.FullName(user, table)
	dsnName := data.String(session.URLParts["dsn"])

	db, err := database.Open(session, dsnName, dsns.DSNAdminAction)
	if err == nil && db != nil {
		if !isAdmin && dsnName == "" && !Authorized(db, user, tableName, adminOperation) {
			return util.ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)
		}

		q, err := parsing.QueryParameters(tableDeleteQuery, map[string]string{
			"table": tableName,
		})
		if err != nil {
			return util.ErrorResponse(w, sessionID, "Error constructing table deletion query; "+err.Error(), http.StatusInternalServerError)
		}

		// If there was a DSN, we are not using the default table so we don't need to use
		// the aggregated user.table version of the table name.
		if dsnName != "" {
			tableName = table
			q = "DROP TABLE " + tableName
		}

		_, err = db.Exec(q)
		if err == nil {
			if dsnName == "" {
				RemoveTablePermissions(sessionID, db, tableName)
			}

			w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)
			w.WriteHeader(http.StatusOK)

			resp := defs.DBRowCount{
				ServerInfo: util.MakeServerInfo(sessionID),
				Count:      1,
				Status:     http.StatusOK,
				Message:    "Table " + tableName + " successfully deleted",
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

	msg := fmt.Sprintf("database table delete error, %s", strings.TrimPrefix(err.Error(), "pq: "))

	if err == nil && db == nil {
		msg = unexpectedNilPointerError
	}

	status := http.StatusBadRequest
	if strings.Contains(msg, "does not exist") {
		status = http.StatusNotFound
	}

	return util.ErrorResponse(w, sessionID, msg, status)
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
