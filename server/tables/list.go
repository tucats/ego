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

	database, err := database.Open(&session.User, data.String(session.URLParts["dsn"]), dsns.DSNReadAction)

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
func listTables(database *database.Database, session *server.Session, r *http.Request, err error, includeRowCounts bool, w http.ResponseWriter) (error, int) {
	var (
		rows       *sql.Rows
		httpStatus int
		names      []defs.Table
		count      int
	)

	db := database.Handle

	schema := session.User
	q := strings.ReplaceAll(tablesListQuery, "{{schema}}", schema)

	if database.Provider == sqlite3Provider {
		q = "select name from sqlite_schema where type='table' "
		schema = ""
	}

	if paging := parsing.PagingClauses(r.URL); paging != "" {
		q = q + paging
	}

	ui.Log(ui.TableLogger, "table.schema.tables", ui.A{
		"session": session.ID,
		"schema":  session.User})

	ui.Log(ui.SQLLogger, "sql.query", ui.A{
		"session": session.ID,
		"sql":     q})

	rows, err = db.Query(q)
	if err == nil {
		var name string

		defer rows.Close()

		names, count, err, httpStatus = getTableNames(rows, name, session, db, schema, database, includeRowCounts, w)
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

func getTableNames(rows *sql.Rows, name string, session *server.Session, db *sql.DB, schema string, database *database.Database, includeRowCounts bool, w http.ResponseWriter) ([]defs.Table, int, error, int) {
	var err error

	names := make([]defs.Table, 0)
	count := 0

	for rows.Next() {
		if err = rows.Scan(&name); err != nil {
			break
		}

		// Is the session.User authorized to see this table at all?
		if !session.Admin && Authorized(session.ID, db, session.User, name, readOperation) {
			continue
		}

		// See how many columns are in this table. Must be a fully-qualfiied name.
		columnQuery := "SELECT * FROM \"" + schema + "\".\"" + name + "\" WHERE 1=0"
		if database.Provider == sqlite3Provider {
			columnQuery = "SELECT * FROM \"" + name + "\" WHERE 1=0"
		}

		ui.Log(ui.SQLLogger, "sql.columns.metadata", ui.A{
			"session": session.ID,
			"sql":     columnQuery})

		tableInfo, err := db.Query(columnQuery)
		if err != nil {
			ui.Log(ui.SQLLogger, "sql.query.error", ui.A{
				"session": session.ID,
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
			q := parsing.QueryParameters(rowCountQuery, map[string]string{
				"schema": session.User,
				"table":  name,
			})

			if database.Provider == sqlite3Provider {
				q = parsing.QueryParameters(rowCountSQLiteQuery, map[string]string{
					"schema": session.User,
					"table":  name,
				})
			}

			ui.Log(ui.SQLLogger, "sql.row.count.query", ui.A{
				"session": session.ID,
				"sql":     q})

			result, e2 := db.Query(q)
			if e2 != nil {
				return nil, 0, e2, util.ErrorResponse(w, session.ID, e2.Error(), http.StatusInternalServerError)
			}

			defer result.Close()

			if result.Next() {
				_ = result.Scan(&rowCount)
			}
		}

		// Package up the info for this table to add to the list.
		names = append(names, defs.Table{
			Name:    name,
			Schema:  session.User,
			Columns: columnCount,
			Rows:    rowCount,
		})
	}

	return names, count, err, http.StatusOK
}
