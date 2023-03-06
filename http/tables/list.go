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
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/util"
)

// ListTables will list all the tables for the given session.User.
func ListTablesHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	if err := util.AcceptedMediaType(r, []string{defs.TablesMediaType}); err != nil {
		util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)

		return http.StatusBadRequest
	}

	if r.Method != http.MethodGet {
		msg := "Unsupported method " + r.Method + " " + r.URL.Path
		util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)

		return http.StatusBadRequest
	}

	// Verify that the parameters are valid, if given.
	if err := util.ValidateParameters(r.URL, map[string]string{
		defs.StartParameterName:    "int",
		defs.LimitParameterName:    "int",
		defs.UserParameterName:     "string",
		defs.RowCountParameterName: "bool",
	}); err != nil {
		util.ErrorResponse(w, session.ID, err.Error(), http.StatusBadRequest)

		return http.StatusBadRequest
	}

	// Currently, the default is to include row counts in the listing. You
	// could change this in the future if it proves too inefficient.
	includeRowCounts := true

	v := r.URL.Query()[defs.RowCountParameterName]
	if len(v) == 1 {
		includeRowCounts = data.Bool(v[0])
	}

	db, err := OpenDB(session.ID, session.User, "")

	if err == nil && db != nil {
		var rows *sql.Rows

		q := strings.ReplaceAll(tablesListQuery, "{{schema}}", session.User)
		if paging := pagingClauses(r.URL); paging != "" {
			q = q + paging
		}

		ui.Log(ui.ServerLogger, "[%d] attempting to read tables from schema %s", session.ID, session.User)
		ui.Log(ui.SQLLogger, "[%d] Query: %s", session.ID, q)

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

				// Is the session.User authorized to see this table at all?
				if !session.Admin && Authorized(session.ID, db, session.User, name, readOperation) {
					continue
				}

				// See how many columns are in this table. Must be a fully-qualfiied name.
				columnQuery := "SELECT * FROM \"" + session.User + "\".\"" + name + "\" WHERE 1=0"
				ui.Log(ui.SQLLogger, "[%d] Columns metadata query: %s", session.ID, columnQuery)

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

				// Let's also count the rows. This may become too expensive but let's try it.
				rowCount := 0

				if includeRowCounts {
					q := queryParameters(rowCountQuery, map[string]string{
						"schema": session.User,
						"table":  name,
					})

					ui.Log(ui.SQLLogger, "[%d] Row count query: %s", session.ID, q)

					result, e2 := db.Query(q)
					if e2 != nil {
						util.ErrorResponse(w, session.ID, e2.Error(), http.StatusInternalServerError)

						return http.StatusInternalServerError
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

			ui.Log(ui.ServerLogger, "[%d] read %d table names", session.ID, count)

			if err == nil {
				resp := defs.TableInfo{
					ServerInfo: util.MakeServerInfo(session.ID),
					Tables:     names,
					Count:      len(names),
				}

				w.Header().Add(defs.ContentTypeHeader, defs.TablesMediaType)

				b, _ := json.MarshalIndent(resp, "", "  ")
				_, _ = w.Write(b)

				if ui.IsActive(ui.RestLogger) {
					ui.WriteLog(ui.RestLogger, "[%d] Response payload:\n%s", session.ID, util.SessionLog(session.ID, string(b)))
				}

				return http.StatusOK
			}
		}
	}

	msg := fmt.Sprintf("Database list error, %v", err)
	if err == nil && db == nil {
		msg = unexpectedNilPointerError
	}

	util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)

	return http.StatusBadRequest
}
