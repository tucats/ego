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
	// Currently, the default is to include row counts in the listing. You
	// could change this in the future if it proves too inefficient.
	includeRowCounts := true

	v := r.URL.Query()[defs.RowCountParameterName]
	if len(v) == 1 {
		includeRowCounts = data.Bool(v[0])
	}

	database, err := database.Open(&session.User, data.String(session.URLParts["dsn"]), dsns.DSNReadAction)

	if err == nil && database.Handle != nil {
		var rows *sql.Rows

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

		ui.Log(ui.TableLogger, "[%d] attempting to read tables from schema %s", session.ID, session.User)
		ui.Log(ui.SQLLogger, "[%d] Query: %s", session.ID, q)

		rows, err = db.Query(q)
		if err == nil {
			var name string

			defer rows.Close()

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

				ui.Log(ui.SQLLogger, "[%d] Columns metadata query: %s", session.ID, columnQuery)

				tableInfo, err := db.Query(columnQuery)
				if err != nil {
					ui.Log(ui.SQLLogger, "[%d] query error: %v", session.ID, err)

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

					ui.Log(ui.SQLLogger, "[%d] Row count query: %s", session.ID, q)

					result, e2 := db.Query(q)
					if e2 != nil {
						return util.ErrorResponse(w, session.ID, e2.Error(), http.StatusInternalServerError)
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

			ui.Log(ui.TableLogger, "[%d] read %d table names", session.ID, count)

			if err == nil {
				resp := defs.TableInfo{
					ServerInfo: util.MakeServerInfo(session.ID),
					Tables:     names,
					Count:      len(names),
					Status:     http.StatusOK,
				}

				w.Header().Add(defs.ContentTypeHeader, defs.TablesMediaType)

				b, _ := json.MarshalIndent(resp, "", "  ")
				_, _ = w.Write(b)
				session.ResponseLength += len(b)

				if ui.IsActive(ui.RestLogger) {
					ui.WriteLog(ui.RestLogger, "[%d] Response payload:\n%s", session.ID, util.SessionLog(session.ID, string(b)))
				}

				return http.StatusOK
			}
		}
	}

	msg := fmt.Sprintf("Database list error, %v", err)
	if err == nil && database == nil {
		msg = unexpectedNilPointerError
	}

	return util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
}
