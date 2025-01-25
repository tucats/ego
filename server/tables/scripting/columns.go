package scripting

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
)

func getColumnInfo(db *database.Database, user string, tableName string, sessionID int) ([]defs.DBColumn, error) {
	columns := make([]defs.DBColumn, 0)
	name, _ := parsing.FullName(user, tableName)

	q := parsing.QueryParameters(tableMetadataQuery, map[string]string{
		"table": name,
	})

	ui.Log(ui.SQLLogger, "sql.query", ui.A{
		"session": sessionID,
		"query":   q})

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
			nullable, hadNullable := typeInfo.Nullable()

			columns = append(columns, defs.DBColumn{
				Name:     name,
				Type:     typeName,
				Size:     int(size),
				Nullable: defs.BoolValue{Specified: hadNullable, Value: nullable}},
			)
		}
	}

	if err != nil {
		return columns, errors.New(err)
	}

	return columns, nil
}
