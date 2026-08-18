package tables

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
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

// InsertRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func InsertAbstractRows(user string, isAdmin bool, tableName string, session *router.Session, w http.ResponseWriter, r *http.Request) int {
	var err error

	dsnName := data.String(session.URLParts["dsn"])

	// Authorized() (security.go) expects its table argument in "dsn.table"
	// form -- it splits on "." to find which DSN's table_perms apply -- so
	// the bare table name must be captured here, before it is overwritten
	// below with the provider-qualified form. See the DSN-qualification
	// note further down for why this matters.
	rawTableName := tableName

	// This must request dsns.DSNWriteAction, not DSNNoAccess (0). The action
	// value feeds dsns.AuthDSN's bitmask test "(auth.Action & action) != 0"
	// (database/open.go), which can never be satisfied when action is 0 --
	// so a Restricted DSN denied every non-admin caller here regardless of
	// what they had been granted. Insert is a write, so this now matches
	// InsertRows's own GetDatabase call in rows.go and the (correct) write
	// action UpdateAbstractRows/DeleteRows request elsewhere in this
	// package.
	db, err := GetDatabase(session, dsnName, dsns.DSNWriteAction)

	if p := parameterString(r); p != "" {
		ui.Log(ui.TableLogger, "table.parms", ui.A{
			"session": session.ID,
			"params":  p})
	}

	if err == nil && db != nil {
		// Amend any table name with the provider-appropriate user schema name.
		tableName, _ = parsing.FullName(db.Provider, session.User, tableName)

		// Note that "update" here means add to or change the row. So we check "update"
		// on test for insert permissions
		//
		// Authorized() returns true when the caller IS permitted, so this
		// condition must be negated to deny when they are NOT. The missing
		// "!" inverted the whole check: a caller holding a valid table_perms
		// grant was denied, while a caller with no grant at all fell through
		// and was allowed to insert. See docs/issues/DATA-SECURITY.md §3.2.
		//
		// The table argument must be DSN-qualified (dsnName+"."+rawTableName),
		// not the provider-qualified "tableName" computed just above -- for
		// SQLite, parsing.FullName leaves the name undotted, so Authorized()
		// would see no "." and resolve dsn="", ReadDSN("") would always fail,
		// and every caller would be denied regardless of their table_perms
		// grant. For PostgreSQL, "tableName" is schema-qualified
		// ("user.table"), which does contain a dot, but the wrong one --
		// Authorized() would parse the session's own username as the DSN
		// name instead of the real DSN. Either way, using rawTableName here
		// (matching sql_permissions.go's own dsn+"."+table usage) is what
		// actually reaches the caller's grant.
		if !isAdmin && !Authorized(session, user, dsnName+"."+rawTableName, defs.TableWritePermission) {
			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.perm.write"), http.StatusForbidden)
		}

		buf := new(strings.Builder)
		_, _ = io.Copy(buf, r.Body)
		rawPayload := buf.String()

		ui.Log(ui.RestLogger, "rest.request.payload", ui.A{
			"session": session.ID,
			"body":    rawPayload})

		// Lets get the rows we are to insert. This is either a row set, or a single object.
		rowSet := defs.DBAbstractRowSet{
			ServerInfo: util.MakeServerInfo(session.ID),
		}

		err = json.Unmarshal([]byte(rawPayload), &rowSet)
		if err != nil || len(rowSet.Rows) == 0 {
			// Not a valid row set, but might be a single item
			item := map[string]any{}

			err = json.Unmarshal([]byte(rawPayload), &item)
			if err != nil {
				return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.insert.payload", ui.A{"err": err.Error()}), http.StatusBadRequest)
			} else {
				rowSet.Count = 1
				keys := make([]string, 0)
				values := make([]any, 0)

				for k, v := range item {
					keys = append(keys, k)
					values = append(values, v)
				}

				rowSet.Rows = make([][]any, 1)
				rowSet.Rows[0] = values

				rowSet.Columns = make([]defs.DBAbstractColumn, len(keys))
				for i, k := range keys {
					rowSet.Columns[i] = defs.DBAbstractColumn{
						Name: k,
					}
				}
			}
		}

		// If at this point we have an empty row set, then just bail out now. Return a success
		// status but an indicator that nothing was done.
		if len(rowSet.Rows) == 0 {
			return util.ErrorResponse(w, session.ID, errors.ErrTableNoRows.Localize(session.Language), http.StatusNoContent)
		}

		if db.HasRowID {
			// For any object in the payload, we must assign a UUID now. This overrides any previous
			// item in the set for _row_id_ or creates it if not found. Row IDs are always assigned
			// on input only.
			rowIDColumn := -1

			for pos, name := range rowSet.Columns {
				if name.Name == defs.RowIDName {
					rowIDColumn = pos
				}
			}

			if rowIDColumn < 0 {
				rowSet.Columns = append(rowSet.Columns, defs.DBAbstractColumn{
					Name: defs.RowIDName,
					Type: "string",
				})

				rowIDColumn = len(rowSet.Columns) - 1
			}

			for n := 0; n < len(rowSet.Rows); n++ {
				rowSet.Rows[n][rowIDColumn] = egostrings.Gibberish(uuid.New())
			}
		}

		// Start a transaction, and then lets loop over the rows in the rowset. Note this might
		// be just one row.
		_ = db.Begin()
		count := 0

		for _, row := range rowSet.Rows {
			columnNames := make([]string, len(rowSet.Columns))
			for i, c := range rowSet.Columns {
				columnNames[i] = c.Name
			}

			q, values := formAbstractInsertQuery(tableName, columnNames, row)

			_, err := db.Exec(q, values...)
			if err == nil {
				count++
			} else {
				_ = db.Rollback()

				return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), dberrors.ExecStatus(err))
			}
		}

		if err == nil {
			response := defs.DBRowCount{
				ServerInfo: util.MakeServerInfo(session.ID),
				Count:      count,
				Status:     http.StatusOK,
			}

			w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

			b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

			if ui.IsActive(ui.RestLogger) {
				ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
					"session": session.ID,
					"body":    string(b)})
			}

			err = db.Commit()
			if err == nil {
				ui.Log(ui.TableLogger, "table.inserted.rows", ui.A{
					"session": session.ID,
					"count":   count})

				return http.StatusOK
			}
		}

		_ = db.Rollback()

		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.insert.error", ui.A{"err": err.Error()}), http.StatusInternalServerError)
	}

	if err != nil {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.insert.error", ui.A{"err": err.Error()}), dberrors.PayloadStatus(err))
	}

	return http.StatusOK
}

// ReadRows reads the data for a given table, and returns it as an array
// of structs for each row, with the struct tag being the column name. The
// query can also specify filter, sort, and column query parameters to refine
// the read operation.
func ReadAbstractRows(user string, isAdmin bool, tableName string, session *router.Session, w http.ResponseWriter, r *http.Request) int {
	dsnName := data.String(session.URLParts["dsn"])

	// Authorized() expects "dsn.table"; capture the bare table name before
	// it is overwritten below with the provider-qualified form. See the
	// longer explanation on the equivalent line in InsertAbstractRows.
	rawTableName := tableName

	db, err := GetDatabase(session, dsnName, dsns.DSNReadAction)
	if err != nil || db == nil {
		if err == nil {
			err = errors.ErrNoDatabase
		}

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), dberrors.PayloadStatus(err))
	}

	// Amend any table name with the provider-appropriate user schema name.
	tableName, _ = parsing.FullName(db.Provider, session.User, tableName)

	// Authorized() returns true when the caller IS permitted, so this
	// condition must be negated to deny when they are NOT. The missing
	// "!" inverted the whole check (see docs/issues/DATA-SECURITY.md
	// §3.2): a caller holding a valid table_perms grant was denied,
	// while a caller with no grant at all fell through and was allowed
	// to read.
	//
	// The table argument must be dsnName+"."+rawTableName, not the
	// provider-qualified "tableName" above -- see InsertAbstractRows for
	// why the provider-qualified form can never resolve correctly here.
	if !isAdmin && !Authorized(session, user, dsnName+"."+rawTableName, defs.TableReadPermission) {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.perm.read"), http.StatusForbidden)
	}

	var q string

	q, err = parsing.FormSelectorDeleteQuery(r.URL, parsing.FiltersFromURL(r.URL), parsing.ColumnsFromURL(r.URL), tableName, user, selectVerb, db.Provider)
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusBadRequest)
	}

	if err = readAbstractRowData(db, q, session, w); errors.Nil(err) {
		return http.StatusOK
	}

	// readAbstractRowData already wrote an error response to w (see its own
	// error paths, each of which calls util.ErrorResponse itself) before
	// returning this non-nil error; the status computed below only feeds
	// the request log, it does not write to w again.
	ui.Log(ui.TableLogger, "table.read.error", ui.A{
		"session": session.ID,
		"error":   err.Error()})

	// This used to look for SQLite's "no such table" wording, so the same
	// missing table answered 404 against SQLite and 400 against PostgreSQL,
	// which words it "relation ... does not exist" (REST-1).
	return dberrors.ExecStatus(err)
}

func readAbstractRowData(db *database.Database, q string, session *router.Session, w http.ResponseWriter) error {
	var (
		rows     *sql.Rows
		err      error
		rowCount int
		result   = [][]any{}
		columns  []defs.DBAbstractColumn
	)

	rows, err = db.Query(q)
	if rows != nil {
		defer rows.Close()
	}

	if err != nil {
		// A table-not-found error becomes 404; anything the classifier does not
		// recognize stays a server-side 500, since Ego built this query itself.
		status := dberrors.ExecStatus(err)

		util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.query.error", ui.A{"err": err.Error()}), status)

		return err
	}

	if columnNames, err := rows.Columns(); err != nil {
		util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.column.names", ui.A{"err": err.Error()}), http.StatusInternalServerError)

		return err
	} else {
		columns = make([]defs.DBAbstractColumn, len(columnNames))
		for i, name := range columnNames {
			columns[i] = defs.DBAbstractColumn{
				Name: name,
			}
		}
	}

	if typeData, err := rows.ColumnTypes(); err == nil {
		for i, ct := range typeData {
			columns[i].Type = strings.ToLower(ct.DatabaseTypeName())
			columns[i].Nullable, _ = ct.Nullable()

			size, ok := ct.Length()
			if !ok {
				size = -1
			}

			columns[i].Size = int(size)
		}
	} else {
		util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.column.types", ui.A{"err": err.Error()}), http.StatusInternalServerError)

		return err
	}

	columnList := strings.Builder{}

	for i, c := range columns {
		if i > 0 {
			columnList.WriteString(", ")
		}

		columnList.WriteString(c.Name)
	}

	columnCount := len(columns)

	for rows.Next() {
		row := make([]any, columnCount)
		rowPointers := make([]any, columnCount)

		for i := range row {
			rowPointers[i] = &row[i]
		}

		err = rows.Scan(rowPointers...)
		if err == nil {
			result = append(result, row)
			rowCount++
		} else {
			util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.row.data", ui.A{"err": err.Error()}), http.StatusInternalServerError)

			return err
		}
	}

	effectiveLimit := session.Limit
	if effectiveLimit == 0 {
		effectiveLimit = parsing.DefaultRowLimit
	}

	response := defs.DBAbstractRowSet{
		ServerInfo: util.MakeServerInfo(session.ID),
		Columns:    columns,
		Rows:       result,
		Count:      len(result),
		Start:      session.Start,
		Limit:      effectiveLimit,
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.AbstractRowSetMediaType)

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	ui.Log(ui.TableLogger, "table.read.", ui.A{
		"session": session.ID,
		"rows":    rowCount,
		"columns": columnCount})

	return err
}

// UpdateRows updates the rows (specified by a filter clause as needed) with the data from the payload.
func UpdateAbstractRows(user string, isAdmin bool, tableName string, session *router.Session, w http.ResponseWriter, r *http.Request) int {
	count := 0
	dsnName := data.String(session.URLParts["dsn"])

	// Authorized() expects "dsn.table"; capture the bare table name before
	// it is overwritten below with the provider-qualified form. See the
	// longer explanation on the equivalent line in InsertAbstractRows.
	rawTableName := tableName

	// This must request dsns.DSNWriteAction, not DSNNoAccess (0), for the
	// same reason documented on InsertAbstractRows above: action 0 can
	// never satisfy AuthDSN's bitmask test, so a Restricted DSN denied
	// every non-admin caller regardless of their grants. 
	db, err := GetDatabase(session, dsnName, dsns.DSNWriteAction)
	if err != nil || db == nil {
		if err == nil {
			err = errors.ErrNoDatabase
		}

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), dberrors.PayloadStatus(err))
	}

	// Amend any table name with the provider-appropriate user schema name.
	tableName, _ = parsing.FullName(db.Provider, session.User, tableName)

	// Authorized() returns true when the caller IS permitted, so this
	// condition must be negated to deny when they are NOT. The missing
	// "!" inverted the whole check (see docs/issues/DATA-SECURITY.md
	// §3.2): a caller holding a valid table_perms grant was denied,
	// while a caller with no grant at all fell through and was allowed
	// to update.
	//
	// The table argument must be dsnName+"."+rawTableName, not the
	// provider-qualified "tableName" above -- see InsertAbstractRows for
	// why the provider-qualified form can never resolve correctly here.
	if !isAdmin && !Authorized(session, user, dsnName+"."+rawTableName, defs.TableUpdatePermission) {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.perm.update"), http.StatusForbidden)
	}

	// Get the payload in a string.
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r.Body)
	rawPayload := buf.String()

	// Lets get the rows we are to update. This is either a row set, or a single object.
	rowSet := defs.DBAbstractRowSet{
		ServerInfo: util.MakeServerInfo(session.ID),
	}

	err = json.Unmarshal([]byte(rawPayload), &rowSet)
	if err != nil || len(rowSet.Rows) == 0 {
		// Not a valid row set, but might be a single item
		item := []any{}

		err = json.Unmarshal([]byte(rawPayload), &item)
		if err != nil {
			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.update.payload", ui.A{"err": err.Error()}), http.StatusBadRequest)
		} else {
			rowSet.Count = 1
			rowSet.Rows = make([][]any, 1)
			rowSet.Rows[0] = item
		}
	}

	// Start a transaction to ensure atomicity of the entire update
	_ = db.Begin()

	// Loop over the row set doing the updates
	for _, data := range rowSet.Rows {
		ui.Log(ui.TableLogger, "table.values", ui.A{
			"session": session.ID,
			"data":    data})

		// Get the column names for the update
		columns := make([]string, len(rowSet.Columns))
		for i, c := range rowSet.Columns {
			columns[i] = c.Name
		}

		q, params, err := formAbstractUpdateQuery(r.URL, tableName, columns, data)
		if err != nil {
			return util.ErrorResponse(w, session.ID, filterErrorMessage(q), http.StatusBadRequest)
		}

		counts, err := db.Exec(q, params...)
		if err == nil {
			rowsAffected, _ := counts.RowsAffected()
			count = count + int(rowsAffected)
		} else {
			_ = db.Rollback()

			return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), dberrors.ExecStatus(err))
		}
	}

	// Every error path in the loop above already returned, so reaching here
	// means every row updated cleanly; a failed commit is the only way this
	// can still fail (matching InsertAbstractRows's own commit handling,
	// which likewise treats a commit failure as a plain 500).
	if err := db.Commit(); err != nil {
		_ = db.Rollback()

		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.table.update.error", ui.A{"err": err.Error()}), http.StatusInternalServerError)
	}

	response := defs.DBRowCount{
		ServerInfo: util.MakeServerInfo(session.ID),
		Count:      count,
		Status:     http.StatusOK,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.RowCountMediaType)

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)

	if ui.IsActive(ui.RestLogger) {
		ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
			"session": session.ID,
			"body":    string(b)})
	}

	ui.Log(ui.TableLogger, "table.updated", ui.A{
		"session": session.ID,
		"count":   count,
		"status":  http.StatusOK})

	return http.StatusOK
}
