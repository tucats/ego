package dbtables

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
)

// ReadTable reads the metadata for a given table, and returns it as an array
// of column names and types
func ReadTable(user string, tableName string, sessionID int32, w http.ResponseWriter, r *http.Request) {

	db, err := OpenDB(sessionID, user, "")

	if err == nil && db != nil {
		var rows *sql.Rows

		q := queryParameters(tableMetadataQuerySting, map[string]string{
			"table":  tableName,
			"schema": user,
		})

		rows, err = db.Query(q)
		if err == nil {
			names, _ := rows.Columns()
			types, _ := rows.ColumnTypes()
			columns := make([]DBColumn, len(names))

			for i, name := range names {
				typeInfo := types[i]
				typeName := typeInfo.ScanType().Name()
				size, _ := typeInfo.Length()
				nullable, _ := typeInfo.Nullable()
				columns[i] = DBColumn{Name: name, Type: typeName, Size: int(size), Nullable: nullable}
			}

			b, _ := json.MarshalIndent(columns, "", "  ")
			w.Write(b)

			return
		}
	}

	msg := fmt.Sprintf("Database table metadata error, %v", err)
	if err == nil && db == nil {
		msg = "Unexpected nil database object pointer"
	}

	ui.Debug(ui.ServerLogger, "[%d] Unable to read table, %v", sessionID, err)
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(msg))
}

func queryParameters(source string, args map[string]string) string {
	result := source
	for k, v := range args {
		v = sqlEscape(v)
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}

	return result
}

// @tomcole probably need to dump this entirely and work on variadic substitution arguments in query!
func sqlEscape(source string) string {
	var result strings.Builder

	for _, ch := range source {
		if ch == '\'' || ch == '"' || ch == '.' || ch == ',' || ch == ';' || ch == '(' || ch == ')' {
			return "INVALID-NAME"
		}
		result.WriteRune(ch)
	}

	return result.String()
}
