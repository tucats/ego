package scripting

import (
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/server/tables/parsing"
)

// getColumnInfo returns the column definitions for a table without reading any
// actual row data. It executes tableMetadataQuery (a SELECT … WHERE 1=0) so
// the database sends back column metadata but zero rows. This is used by
// doInsert and doUpdate to validate and coerce the caller's payload before
// building the final INSERT/UPDATE query.
//
// For each column the function records:
//   - Name: the column name as the database knows it
//   - Type: the Go scan type name (e.g. "int64", "string"); falls back to the
//     raw database type name (e.g. "VARCHAR") if the driver doesn't know the
//     Go type
//   - Size: max byte length (0 for fixed-size types)
//   - Nullable: whether the column allows NULLs, and whether the driver
//     reported this information at all (Specified flag)
//
// The internal row-ID column (defs.RowIDName) is silently skipped — it is
// managed by the server and should not be visible to callers.
func getColumnInfo(db *database.Database, user string, tableName string) ([]defs.DBColumn, error) {
	columns := make([]defs.DBColumn, 0)
	name, _ := parsing.FullName(user, tableName)

	q, err := parsing.QueryParameters(tableMetadataQuery, map[string]string{
		"table": name,
	})
	if err != nil {
		return nil, err
	}

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

			// Start by seeing what Go type it will become. If that isn't
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
