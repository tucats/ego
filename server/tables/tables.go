package tables

import (
	"database/sql"
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

// TableCreate creates a new table based on the JSON payload, which must be an array of DBColumn objects, defining
// the characteristics of each column in the table. If the table name is the special name "@sql" the payload instead
// is assumed to be a JSON-encoded string containing arbitrary SQL to exectue. Only an admin user can use the "@sql"
// table name.
func TableCreate(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	sessionID := session.ID
	user := session.User
	tableName := data.String(session.URLParts["table"])

	db, err := database.Open(&session.User, data.String(session.URLParts["dsn"]), dsns.DSNAdminAction)
	if err == nil && db != nil {
		// Unless we're using sqlite, add explicit schema to the table name.
		if db.Provider != sqlite3Provider {
			tableName, _ = parsing.FullName(user, tableName)
		}

		if !session.Admin && Authorized(sessionID, db.Handle, user, tableName, updateOperation) {
			return util.ErrorResponse(w, sessionID, "User does not have update permission", http.StatusForbidden)
		}

		data := []defs.DBColumn{}

		if err = json.NewDecoder(r.Body).Decode(&data); err != nil {
			return util.ErrorResponse(w, sessionID, "Invalid table create payload: "+err.Error(), http.StatusBadRequest)
		}

		for _, column := range data {
			if column.Name == "" {
				return util.ErrorResponse(w, sessionID, "Missing or empty column name", http.StatusBadRequest)
			}

			if column.Type == "" {
				return util.ErrorResponse(w, sessionID, "Missing or empty type name", http.StatusBadRequest)
			}

			if !parsing.KeywordMatch(column.Type, defs.TableColumnTypeNames...) {
				return util.ErrorResponse(w, sessionID, "Invalid type name: "+column.Type, http.StatusBadRequest)
			}
		}

		q := parsing.FormCreateQuery(r.URL, user, session.Admin, data, sessionID, w, db.Provider)
		if q == "" {
			return http.StatusOK
		}

		if db.Provider != sqlite3Provider {
			if !createSchemaIfNeeded(w, sessionID, db.Handle, user, tableName) {
				return http.StatusOK
			}
		}

		ui.Log(ui.SQLLogger, "[%d] Exec: %s", sessionID, q)

		counts, err := db.Exec(q)
		if err == nil {
			rows, _ := counts.RowsAffected()
			result := defs.DBRowCount{
				ServerInfo: util.MakeServerInfo(sessionID),
				Count:      int(rows),
				Status:     http.StatusOK,
			}

			tableName, _ = parsing.FullName(user, tableName)

			CreateTablePermissions(sessionID, db.Handle, user, tableName, readOperation, deleteOperation, updateOperation)
			w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

			b, _ := json.MarshalIndent(result, "", "  ")
			_, _ = w.Write(b)
			session.ResponseLength += len(b)

			if ui.IsActive(ui.RestLogger) {
				ui.WriteLog(ui.RestLogger, "[%d] Response payload:\n%s", sessionID, util.SessionLog(sessionID, string(b)))
			}

			ui.Log(ui.TableLogger, "[%d] table created", sessionID)

			return http.StatusOK
		}

		ui.Log(ui.TableLogger, "[%d] Error creating table, %v", sessionID, err)

		return util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
	}

	ui.Log(ui.TableLogger, "[%d] Error inserting into table, %v", sessionID, strings.TrimPrefix(err.Error(), "pq: "))

	if err == nil {
		err = fmt.Errorf("unknown error")
	}

	return util.ErrorResponse(w, sessionID, err.Error(), http.StatusBadRequest)
}

// Verify that the schema exists for this user, and create it if not found.
func createSchemaIfNeeded(w http.ResponseWriter, sessionID int, db *sql.DB, user string, tableName string) bool {
	schema := user
	if dot := strings.Index(tableName, "."); dot >= 0 {
		schema = tableName[:dot]
	}

	q := parsing.QueryParameters(createSchemaQuery, map[string]string{
		"schema": schema,
	})

	result, err := db.Exec(q)
	if err != nil {
		util.ErrorResponse(w, sessionID, "Error creating schema; "+err.Error(), http.StatusInternalServerError)

		return false
	}

	count, _ := result.RowsAffected()
	if count > 0 {
		ui.Log(ui.TableLogger, "[%d] Created schema %s", sessionID, schema)
	}

	return true
}

// ReadTable reads the metadata for a given table, and returns it as an array
// of column names and types.
func ReadTable(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	tableName := data.String(session.URLParts["table"])
	dsn := data.String(session.URLParts["dsn"])

	db, err := database.Open(&session.User, dsn, dsns.DSNAdminAction)
	if err == nil && db != nil {
		sqlite := strings.EqualFold(db.Provider, "sqlite3")
		tableName, _ = parsing.FullName(session.User, tableName)

		if !session.Admin && Authorized(session.ID, db.Handle, session.User, tableName, readOperation) {
			return util.ErrorResponse(w, session.ID, "User does not have read permission", http.StatusForbidden)
		}

		// Determine which columns must be unique. We don't do this for sqlite3.
		uniqueColumns := map[string]bool{}
		keys := []string{}
		nullableColumns := map[string]bool{}

		if !sqlite {
			q := parsing.QueryParameters(uniqueColumnsQuery, map[string]string{
				"table": tableName,
			})

			ui.Log(ui.SQLLogger, "[%d] Read unique query: \n%s", session.ID, util.SessionLog(session.ID, q))

			rows, err := db.Query(q)
			if err != nil {
				return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
			}

			defer rows.Close()

			for rows.Next() {
				var name string

				_ = rows.Scan(&name)
				uniqueColumns[name] = true

				keys = append(keys, name)
			}

			ui.Log(ui.TableLogger, "[%d] Unique columns: %v", session.ID, keys)

			// Determine which columns are nullable.

			q = parsing.QueryParameters(nullableColumnsQuery, map[string]string{
				"table": tableName,
				"quote": "",
			})

			ui.Log(ui.SQLLogger, "[%d] Read nullable query: %s", session.ID, util.SessionLog(session.ID, q))

			var nrows *sql.Rows

			nrows, err = db.Query(q)
			if err != nil {
				return util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
			}

			defer nrows.Close()

			keys = []string{}

			for nrows.Next() {
				var (
					schemaName, tableName, columnName string
					nullable                          bool
				)

				_ = nrows.Scan(&schemaName, &tableName, &columnName, &nullable)

				if nullable {
					nullableColumns[columnName] = true

					keys = append(keys, columnName)
				}
			}

			ui.Log(ui.TableLogger, "[%d] Nullable columns: %v", session.ID, keys)
		}

		// Get standard column names and type info.
		columns, e2 := getColumnInfo(db, session.User, tableName, session.ID)
		if e2 == nil {
			// Determine which columns are nullable
			for n, column := range columns {
				columns[n].Nullable.Specified = true
				columns[n].Nullable.Value = nullableColumns[column.Name]

				if column.Nullable.Value {
					columns[n].Nullable.Specified = true
					columns[n].Nullable.Value = true
				}

				if column.Size > 0 {
					columns[n].Size = column.Size
				}
			}

			// Determine which columns are also unique
			for n, column := range columns {
				columns[n].Unique = defs.BoolValue{Specified: true, Value: uniqueColumns[column.Name]}
			}

			resp := defs.TableColumnsInfo{
				ServerInfo: util.MakeServerInfo(session.ID),
				Columns:    columns,
				Count:      len(columns),
				Status:     http.StatusOK,
			}

			w.Header().Add(defs.ContentTypeHeader, defs.TableMetadataMediaType)

			b, _ := json.MarshalIndent(resp, "", "  ")
			_, _ = w.Write(b)
			session.ResponseLength += len(b)

			if ui.IsActive(ui.RestLogger) {
				ui.WriteLog(ui.RestLogger, "[%d] Response payload:\n%s", session.ID, util.SessionLog(session.ID, string(b)))
			}

			return http.StatusOK
		}

		err = errors.New(e2)
	}

	msg := fmt.Sprintf("database table metadata error, %s", strings.TrimPrefix(err.Error(), "pq: "))
	status := http.StatusBadRequest

	if strings.Contains(err.Error(), "does not exist") {
		status = http.StatusNotFound
	}

	if err == nil && db == nil {
		msg = unexpectedNilPointerError
		status = http.StatusInternalServerError
	}

	return util.ErrorResponse(w, session.ID, msg, status)
}

func getColumnInfo(db *database.Database, user string, tableName string, sessionID int) ([]defs.DBColumn, error) {
	columns := make([]defs.DBColumn, 0)
	name, _ := parsing.FullName(user, tableName)

	q := parsing.QueryParameters(tableMetadataQuery, map[string]string{
		"table": name,
	})

	if db.Provider == "sqlite3" {
		q = parsing.QueryParameters(tableSQLiteMetadataQuery, map[string]string{
			"table": name,
		})
	}

	ui.Log(ui.SQLLogger, "[%d] Reading table metadata query: %s", sessionID, q)

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

			// Start by seeing what Go type it will become. IF that isn't
			// known, then get the underlying database type name instead.
			typeName := typeInfo.ScanType().Name()
			if typeName == "" {
				typeName = typeInfo.DatabaseTypeName()
			}

			size, _ := typeInfo.Length()
			nullable, _ := typeInfo.Nullable()

			// SQLite3 has some funky names, so handle them here.
			if db.Provider == "sqlite3" {
				switch typeName {
				case "NullInt64":
					typeName = "int64"
					nullable = true
					size = 8

				case "NullFloat64":
					typeName = "float64"
					nullable = true
					size = 8
				case "NullString":
					typeName = "string"
					nullable = true
				}
			}

			columns = append(columns, defs.DBColumn{
				Name:     name,
				Type:     typeName,
				Size:     int(size),
				Nullable: defs.BoolValue{Specified: true, Value: nullable}},
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

	db, err := database.Open(&session.User, dsnName, dsns.DSNAdminAction)
	if err == nil && db != nil {
		if !isAdmin && dsnName == "" && !Authorized(sessionID, db.Handle, user, tableName, adminOperation) {
			return util.ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)
		}

		q := parsing.QueryParameters(tableDeleteQuery, map[string]string{
			"table": tableName,
		})

		// If there was a DSN, we are not using the defalt table so we don't need to use
		// the aggregated user.table version of the table name.
		if dsnName != "" {
			tableName = table
			q = "DROP TABLE " + tableName
		}

		ui.Log(ui.SQLLogger, "[%d] Query: %s", sessionID, q)

		_, err = db.Exec(q)
		if err == nil {
			if dsnName == "" {
				RemoveTablePermissions(sessionID, db.Handle, tableName)
			}

			return util.ErrorResponse(w, sessionID, "Table "+tableName+" successfully deleted", http.StatusOK)
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
