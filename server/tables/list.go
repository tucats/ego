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
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
	"github.com/tucats/ego/util"
)

// ListTables will list all the tables for the given session.User.
func ListTablesHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		err        error
		httpStatus int
	)

	// Currently, the default is to include row counts in the listing. You
	// could change this in the future if it proves too inefficient.
	includeRowCounts := true

	v := r.URL.Query()[defs.RowCountParameterName]
	if len(v) == 1 {
		includeRowCounts, err = data.Bool(v[0])
		if err != nil {
			return util.ErrorResponse(w, session.ID, "Invalid row count parameter: "+v[0], http.StatusBadRequest)
		}
	}

	database, err := database.Open(session, data.String(session.URLParts["dsn"]), dsns.DSNReadAction)

	if err == nil && database.Handle != nil {
		err, httpStatus = listTables(database, session, r, err, includeRowCounts, w)
		if httpStatus > http.StatusOK {
			return httpStatus
		}

		if err == nil {
			return http.StatusOK
		}
	}

	msg := fmt.Sprintf("Database list error, %v", err)
	if err == nil && database == nil {
		msg = unexpectedNilPointerError
	}

	return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
}

// listTables generates the response payload for the list of tables.
func listTables(db *database.Database, session *server.Session, r *http.Request, err error, includeRowCounts bool, w http.ResponseWriter) (error, int) {
	var (
		rows       *sql.Rows
		httpStatus int
		names      []defs.Table
		count      int
	)

	schema := session.User
	q := strings.ReplaceAll(tablesListQuery, "{{schema}}", schema)

	if db.Provider == sqlite3Provider {
		q = "select name from sqlite_schema where (type='table' or type='view') "
		schema = ""
	}

	if paging := parsing.PagingClauses(r.URL); paging != "" {
		q = q + paging
	}

	ui.Log(ui.TableLogger, "table.schema.tables", ui.A{
		"session": session.ID,
		"schema":  session.User})

	rows, err = db.Query(q)
	if err == nil {
		var name string

		defer rows.Close()

		names, count, err, httpStatus = getTableNames(rows, name, db, schema, includeRowCounts, w)
		if httpStatus > http.StatusOK {
			return err, httpStatus
		}

		ui.Log(ui.TableLogger, "table.schema.count", ui.A{
			"session": session.ID,
			"count":   count})

		if err == nil {
			resp := defs.TableInfo{
				ServerInfo: util.MakeServerInfo(session.ID),
				Tables:     names,
				Count:      len(names),
				Status:     http.StatusOK,
			}

			w.Header().Add(defs.ContentTypeHeader, defs.TablesMediaType)

			b, _ := json.MarshalIndent(resp, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
			_, _ = w.Write(b)
			session.ResponseLength += len(b)

			if ui.IsActive(ui.RestLogger) {
				ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
					"session": session.ID,
					"body":    string(b)})
			}

			return nil, http.StatusOK
		}
	}

	return err, 0
}

func getTableNames(rows *sql.Rows, name string, db *database.Database, schema string, includeRowCounts bool, w http.ResponseWriter) ([]defs.Table, int, error, int) {
	var err error

	names := make([]defs.Table, 0)
	count := 0

	for rows.Next() {
		if err = rows.Scan(&name); err != nil {
			break
		}

		// Is the session.User authorized to see this table at all?
		if !db.Session.Admin && Authorized(db, db.Session.User, name, readOperation) {
			continue
		}

		// See how many columns are in this table. Must be a fully-qualified name.
		columnQuery := "SELECT * FROM \"" + schema + "\".\"" + name + "\" WHERE 1=0"
		if db.Provider == sqlite3Provider {
			columnQuery = "SELECT * FROM \"" + name + "\" WHERE 1=0"
		}

		tableInfo, err := db.Query(columnQuery)
		if err != nil {
			ui.Log(ui.SQLLogger, "sql.query.error", ui.A{
				"session": db.Session.ID,
				"sql":     columnQuery,
				"error":   err})

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

		// Let's also count the rows. This may become too expensive but let's try it.
		rowCount := 0

		if includeRowCounts {
			q, err := parsing.QueryParameters(rowCountQuery, map[string]string{
				"schema": db.Session.User,
				"table":  name,
			})
			if err != nil {
				return nil, 0, err, util.ErrorResponse(w, db.Session.ID, err.Error(), http.StatusInternalServerError)
			}

			if db.Provider == sqlite3Provider {
				q, err = parsing.QueryParameters(rowCountSQLiteQuery, map[string]string{
					"schema": db.Session.User,
					"table":  name,
				})
				if err != nil {
					return nil, 0, err, util.ErrorResponse(w, db.Session.ID, err.Error(), http.StatusInternalServerError)
				}
			}

			result, e2 := db.Query(q)
			if e2 != nil {
				return nil, 0, e2, util.ErrorResponse(w, db.Session.ID, e2.Error(), http.StatusInternalServerError)
			}

			defer result.Close()

			if result.Next() {
				_ = result.Scan(&rowCount)
			}
		}

		// Package up the info for this table to add to the list.
		names = append(names, defs.Table{
			Name:    name,
			Schema:  db.Session.User,
			Columns: columnCount,
			Rows:    rowCount,
		})
	}

	return names, count, err, http.StatusOK
}
