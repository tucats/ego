package dbtables

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

const UnexpectedNilPointerError = "Unexpected nil database object pointer"

// TableCreate creates a new table based on the JSON payload, which must be an array of DBColumn objects, defining
// the characteristics of each column in the table. If the table name is the special name "@sql" the payload instead
// is assumed to be a JSON-encoded string containing arbitrary SQL to exectue. Only an admin user can use the "@sql"
// table name.
func TableCreate(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	var err error

	// Verify that there are no parameters
	if invalid := util.ValidateParameters(r.URL, map[string]string{
		defs.UserParameterName: "string",
	}); !errors.Nil(invalid) {
		ErrorResponse(w, sessionID, invalid.Error(), http.StatusBadRequest)

		return
	}

	db, err := OpenDB(sessionID, user, "")

	if err == nil && db != nil {
		// Special case; if the table name is @SQL then the payload is processed as a simple
		// SQL  statement and not a table creation.
		if strings.EqualFold(tableName, sqlPseudoTable) {
			if !isAdmin {
				ErrorResponse(w, sessionID, "No privilege for direct SQL execution", http.StatusForbidden)

				return
			}

			var statementText string

			err := json.NewDecoder(r.Body).Decode(&statementText)
			if err == nil {
				if strings.HasPrefix(strings.TrimSpace(strings.ToLower(statementText)), "select ") {
					ui.Debug(ui.TableLogger, "[%d] SQL query: %s", sessionID, statementText)

					err = readRowData(db, statementText, sessionID, w)
					if err != nil {
						ErrorResponse(w, sessionID, "Error reading SQL query; "+err.Error(), http.StatusInternalServerError)

						return
					}
				} else {
					var rows sql.Result

					ui.Debug(ui.TableLogger, "[%d] SQL execute: %s", sessionID, statementText)

					rows, err = db.Exec(statementText)
					if err == nil {
						count, _ := rows.RowsAffected()
						reply := defs.DBRowCount{Count: int(count)}

						b, _ := json.MarshalIndent(reply, "", "  ")
						_, _ = w.Write(b)

						return
					}
				}
			}

			if err != nil {
				ErrorResponse(w, sessionID, "Error in SQL execute; "+err.Error(), http.StatusInternalServerError)
			}

			return
		}

		if !isAdmin && Authorized(sessionID, nil, user, tableName, updateOperation) {
			ErrorResponse(w, sessionID, "User does not have update permission", http.StatusForbidden)

			return
		}

		data := []defs.DBColumn{}

		err = json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			ErrorResponse(w, sessionID, "Invalid table create payload: "+err.Error(), http.StatusBadRequest)

			return
		}

		for _, column := range data {
			if column.Name == "" {
				ErrorResponse(w, sessionID, "Missing or empty column name", http.StatusBadRequest)

				return
			}

			if column.Type == "" {
				ErrorResponse(w, sessionID, "Missing or empty type name", http.StatusBadRequest)

				return
			}

			if !keywordMatch(column.Type, defs.TableColumnTypeNames...) {
				ErrorResponse(w, sessionID, "Invalid type name: "+column.Type, http.StatusBadRequest)

				return
			}
		}

		q := formCreateQuery(r.URL, user, isAdmin, data, sessionID, w)
		if q == "" {
			return
		}

		if !createSchemaIfNeeded(w, sessionID, db, user, tableName) {
			return
		}

		ui.Debug(ui.TableLogger, "[%d] Create table with query: %s", sessionID, q)

		counts, err := db.Exec(q)
		if err == nil {
			rows, _ := counts.RowsAffected()
			result := defs.DBRowCount{
				Count: int(rows),
				RestResponse: defs.RestResponse{
					Status: http.StatusOK,
				},
			}

			tableName, _ = fullName(user, tableName)
			CreateTablePermissions(sessionID, db, user, tableName, readOperation, deleteOperation, updateOperation)

			b, _ := json.MarshalIndent(result, "", "  ")
			_, _ = w.Write(b)

			ui.Debug(ui.ServerLogger, "[%d] table created", sessionID)

			return
		}

		ui.Debug(ui.ServerLogger, "[%d] Error creating table, %v", sessionID, err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))

		return
	}

	ui.Debug(ui.TableLogger, "[%d] Error inserting into table, %v", sessionID, err)
	w.WriteHeader(http.StatusBadRequest)

	if err == nil {
		err = fmt.Errorf("unknown error")
	}

	_, _ = w.Write([]byte(err.Error()))
}

// Verify that the schema exists for this user, and create it if not found.
func createSchemaIfNeeded(w http.ResponseWriter, sessionID int32, db *sql.DB, user string, tableName string) bool {
	schema := user
	if dot := strings.Index(tableName, "."); dot >= 0 {
		schema = tableName[:dot]
	}

	q := queryParameters(createSchemaQuery, map[string]string{
		"schema": schema,
	})

	result, err := db.Exec(q)
	if err != nil {
		ErrorResponse(w, sessionID, "Error creating schema; "+err.Error(), http.StatusInternalServerError)

		return false
	}

	count, _ := result.RowsAffected()
	if count > 0 {
		ui.Debug(ui.TableLogger, "[%d] Created schema %s", sessionID, schema)
	}

	return true
}

// ReadTable reads the metadata for a given table, and returns it as an array
// of column names and types.
func ReadTable(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	// Verify that there are no parameters
	if invalid := util.ValidateParameters(r.URL, map[string]string{
		defs.UserParameterName: "string",
	}); !errors.Nil(invalid) {
		ErrorResponse(w, sessionID, invalid.Error(), http.StatusBadRequest)

		return
	}

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {
		// Special case; if the table name is @permissions then the payload is processed as request
		// to read all the permissions data
		if strings.EqualFold(tableName, permissionsPseudoTable) {
			if !isAdmin {
				ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)

				return
			}

			ReadAllPermissions(db, sessionID, w, r)

			return
		}

		tableName, _ = fullName(user, tableName)

		if !isAdmin && Authorized(sessionID, nil, user, tableName, readOperation) {
			ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)

			return
		}

		columns, e2 := getColumnInfo(db, tableName, sessionID)
		if errors.Nil(e2) {
			resp := defs.TableColumnsInfo{
				Columns: columns,
				Count:   len(columns),
				RestResponse: defs.RestResponse{
					Status: http.StatusOK,
				},
			}

			b, _ := json.MarshalIndent(resp, "", "  ")
			_, _ = w.Write(b)

			return
		}

		err = e2
	}

	msg := fmt.Sprintf("database table metadata error, %v", err)
	status := http.StatusBadRequest

	if strings.Contains(err.Error(), "does not exist") {
		status = http.StatusNotFound
	}

	if err == nil && db == nil {
		msg = UnexpectedNilPointerError
		status = http.StatusInternalServerError
	}

	ErrorResponse(w, sessionID, msg, status)
}

func getColumnInfo(db *sql.DB, tableName string, sessionID int32) ([]defs.DBColumn, *errors.EgoError) {
	columns := make([]defs.DBColumn, 0)
	q := queryParameters(tableMetadataQuery, map[string]string{
		"table": tableName,
	})

	ui.Debug(ui.TableLogger, "[%d] Reading table metadata with query %s", sessionID, q)

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

			columns = append(columns, defs.DBColumn{
				Name:     name,
				Type:     typeName,
				Size:     int(size),
				Nullable: nullable},
			)
		}
	}

	if err != nil {
		return columns, errors.New(err)
	}

	return columns, nil
}

//DeleteTable will delete a database table from the user's schema.
func DeleteTable(user string, isAdmin bool, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {
	// Verify that there are no parameters
	if invalid := util.ValidateParameters(r.URL, map[string]string{
		defs.UserParameterName: "string",
	}); !errors.Nil(invalid) {
		ErrorResponse(w, sessionID, invalid.Error(), http.StatusBadRequest)

		return
	}

	tableName, _ = fullName(user, tableName)

	db, err := OpenDB(sessionID, user, "")
	if err == nil && db != nil {
		if !isAdmin && Authorized(sessionID, nil, user, tableName, adminOperation) {
			ErrorResponse(w, sessionID, "User does not have read permission", http.StatusForbidden)

			return
		}

		q := queryParameters(tableDeleteQuery, map[string]string{
			"table": tableName,
		})

		ui.Debug(ui.ServerLogger, "[%d] attempting to delete table %s", sessionID, tableName)
		ui.Debug(ui.TableLogger, "[%d]    with query %s", sessionID, q)

		_, err = db.Exec(q)
		if err == nil {
			RemoveTablePermissions(sessionID, db, tableName)
			ErrorResponse(w, sessionID, "Table "+tableName+" successfully deleted", http.StatusOK)

			return
		}
	}

	msg := fmt.Sprintf("database table delete error, %v", err)
	if err == nil && db == nil {
		msg = UnexpectedNilPointerError
	}

	status := http.StatusBadRequest
	if strings.Contains(msg, "does not exist") {
		status = http.StatusNotFound
	}

	ErrorResponse(w, sessionID, msg, status)
}

// ListTables will list all the tables for the given user.
func ListTables(user string, isAdmin bool, sessionID int32, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		msg := "Unsupported method " + r.Method + " " + r.URL.Path
		ErrorResponse(w, sessionID, msg, http.StatusBadRequest)

		return
	}

	// Verify that the parameters are valid, if given.
	if invalid := util.ValidateParameters(r.URL, map[string]string{
		defs.StartParameterName: "int",
		defs.LimitParameterName: "int",
		defs.UserParameterName:  "string",
	}); !errors.Nil(invalid) {
		ErrorResponse(w, sessionID, invalid.Error(), http.StatusBadRequest)

		return
	}

	db, err := OpenDB(sessionID, user, "")

	if err == nil && db != nil {
		var rows *sql.Rows

		q := strings.ReplaceAll(tablesListQuery, "{{schema}}", user)
		if paging := pagingClauses(r.URL); paging != "" {
			q = q + paging
		}

		ui.Debug(ui.ServerLogger, "[%d] attempting to read tables from schema %s", sessionID, user)
		ui.Debug(ui.TableLogger, "[%d]    with query %s", sessionID, q)

		rows, err = db.Query(q)
		if err == nil {
			var name string

			defer rows.Close()

			names := make([]defs.Table, 0)
			count := 0

			for rows.Next() {
				err = rows.Scan(&name)
				if err != nil {
					break
				}

				// Is the user authorized to see this table at all?
				if !isAdmin && Authorized(sessionID, db, user, name, readOperation) {
					continue
				}

				// See how many columns are in this table. Must be a fully-qualfiied name.
				columnQuery := "SELECT * FROM " + user + "." + name + " WHERE 1=0"
				ui.Debug(ui.TableLogger, "[%d] Reading columns metadata with query %s", sessionID, columnQuery)

				tableInfo, err := db.Query(columnQuery)
				if err != nil {
					continue
				}

				defer tableInfo.Close()
				count++

				columns, _ := tableInfo.Columns()
				columnCount := len(columns)

				for _, columnName := range columns {
					if columnName == defs.RowIDName {
						columnCount--

						break
					}
				}

				names = append(names, defs.Table{
					Name:    name,
					Schema:  user,
					Columns: columnCount,
				})
			}

			ui.Debug(ui.ServerLogger, "[%d] read %d table names", sessionID, count)

			if err == nil {
				resp := defs.TableInfo{
					Tables: names,
					Count:  len(names),
					RestResponse: defs.RestResponse{
						Status: http.StatusOK,
					},
				}

				b, _ := json.MarshalIndent(resp, "", "  ")
				_, _ = w.Write(b)

				return
			}
		}
	}

	msg := fmt.Sprintf("Database list error, %v", err)
	if err == nil && db == nil {
		msg = UnexpectedNilPointerError
	}

	ErrorResponse(w, sessionID, msg, http.StatusBadRequest)
}
