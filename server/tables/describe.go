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

// ReadTable handler reads the metadata for a given table, and returns it as an array
// of column names and types. This is used by the 'ego tables show' command, for example.
func ReadTable(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Get the table name and DSN name from the URL. If not present, these will be blank.
	tableName := data.String(session.URLParts["table"])
	dsn := data.String(session.URLParts["dsn"])

	// Attempt to connect to the table. If the DSN name exists, then it is used to get the
	// credentials for the database. Otherwise, the session user informaiton is used to connect.
	db, err := database.Open(&session.User, dsn, dsns.DSNAdminAction)
	if err == nil && db != nil {
		sqlite := strings.EqualFold(db.Provider, "sqlite3")
		tableName, _ = parsing.FullName(session.User, tableName)

		// If the current user is not an administrator, see if the user has read permission for this table.
		// If not, return a 403 Forbidden error.
		if !session.Admin && Authorized(session.ID, db.Handle, session.User, tableName, readOperation) {
			return util.ErrorResponse(w, session.ID, "User does not have read permission", http.StatusForbidden)
		}

		// Get the table metadata. We don't do this for sqlite3.
		var columns []defs.DBColumn

		// Determine which columns must have unique values and which cannot be null values. These are
		// database attribute of each column.  This is not supported for sqlite3.
		var httpStatus int

		uniqueColumns := map[string]bool{}
		nullableColumns := map[string]bool{}

		if !sqlite {
			// Form the query for determining the unique columns for a given table.
			uniqueColumns, nullableColumns, httpStatus = getColumnMetadata(db, tableName, session, w)
			if httpStatus > 200 {
				return httpStatus
			}
		}

		// Get standard column names and type info. This is done regardless of the database
		// provider.
		columns, e2 := getColumnInfo(db, session.User, tableName, session.ID)
		if e2 == nil {
			// If it succeeded, merge in the information we gleaned about nullable columns
			// and send the response to the caller.
			return sendColumnResponse(columns, nullableColumns, uniqueColumns, session, w)
		}

		// Form an Ego error and get ready to report failure...
		err = errors.New(e2)
	}

	// Something failed, and it's stored in the 'err' variable. Trim off any leading "pq: " prefix
	// put there for database errors from the Postgresql driver.
	msg := fmt.Sprintf("database table metadata error, %s", strings.TrimPrefix(err.Error(), "pq: "))
	status := http.StatusBadRequest

	// If the error is due to a non-existing table, return a 404 status code.
	if strings.Contains(err.Error(), "does not exist") {
		status = http.StatusNotFound
	}

	// If after all this we didn't get an error but we also never got a database connection,
	// it means there was an unexpected nil pointer error. Report this to the caller as a
	// 500 status code.
	if err == nil && db == nil {
		msg = unexpectedNilPointerError
		status = http.StatusInternalServerError
	}

	// Return the error response with the most accurage msg and status.
	return util.ErrorResponse(w, session.ID, msg, status)
}

// For the array of column info, merge in the metadata from the database provider (if any) and generate
// a response to the caller.
func sendColumnResponse(columns []defs.DBColumn, nullableColumns map[string]bool, uniqueColumns map[string]bool, session *server.Session, w http.ResponseWriter) int {
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

	// Construct a response object which contains the server info header, and the array of column
	// information. The response includes the total count of columns in the table.
	// The server info header is included in the response.
	resp := defs.TableColumnsInfo{
		ServerInfo: util.MakeServerInfo(session.ID),
		Columns:    columns,
		Count:      len(columns),
		Status:     http.StatusOK,
	}

	// Set the return type to indicate it is JSON for table metadata.
	w.Header().Add(defs.ContentTypeHeader, defs.TableMetadataMediaType)

	// Convert the response object to JSON and write it to the response.
	b, _ := json.MarshalIndent(resp, "", "  ")
	_, _ = w.Write(b)
	session.ResponseLength += len(b)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	return http.StatusOK
}

// getColumnMetadata retrieves the unique and nullable columns for a given table. This cannot be used
// when the database provider is SQLite.
func getColumnMetadata(db *database.Database, tableName string, session *server.Session, w http.ResponseWriter) (map[string]bool, map[string]bool, int) {
	uniqueColumns := map[string]bool{}
	nullableColumns := map[string]bool{}
	keys := []string{}

	q := parsing.QueryParameters(uniqueColumnsQuery, map[string]string{
		"table": tableName,
	})

	ui.Log(ui.SQLLogger, "sql.read.unique", ui.A{
		"session": session.ID,
		"sql":     q})

	// Execute the query to get the unique columns.
	rows, err := db.Query(q)
	if err != nil {
		return uniqueColumns, nullableColumns, util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	defer rows.Close()

	// Read the rows from the result, which will be the names of the columns in the table that
	// are defined as UNIQUE.
	for rows.Next() {
		var name string

		_ = rows.Scan(&name)
		uniqueColumns[name] = true

		keys = append(keys, name)
	}

	ui.Log(ui.TableLogger, "[table.unique.columns", ui.A{
		"session": session.ID,
		"list":    keys})

	// Determine which columns are nullable. Form the quero to the database to get the nullable
	// column names.
	q = parsing.QueryParameters(nullableColumnsQuery, map[string]string{
		"table": tableName,
		"quote": "",
	})

	ui.Log(ui.SQLLogger, "sql.read.nullable", ui.A{
		"session": session.ID,
		"sql":     q})

	var nrows *sql.Rows

	// Execute the query to get the nullable columns.
	nrows, err = db.Query(q)
	if err != nil {
		return uniqueColumns, nullableColumns, util.ErrorResponse(w, session.ID, err.Error(), http.StatusInternalServerError)
	}

	defer nrows.Close()

	keys = []string{}

	// Read the rows from the result, which will be the names of the columns in the table that
	// are defined as NULLABLE.
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

	ui.Log(ui.TableLogger, "[table.nullab.ecolumns", ui.A{
		"session": session.ID,
		"list":    keys})

	return uniqueColumns, nullableColumns, 0
}
