package resources

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/egostrings"
)

// Generate the SQL that reads row(s) from the table that will be formatted
// as resource objects. This does not include any filtering or sorting, but
// explicitly call out the columns to be retrieved.
func (r ResHandle) readRowSQL() string {
	sql := strings.Builder{}

	sql.WriteString("select ")

	for index, column := range r.Columns {
		if index > 0 {
			sql.WriteRune(',')
		}

		sql.WriteString(egostrings.SQLIdentifier(column.SQLName))
	}

	sql.WriteString(fmt.Sprintf(" from %s ", egostrings.SQLIdentifier(r.Table)))

	return sql.String()
}

// Generate the SQL that is used to create the table for the
// resource handle. This identifies the column names, types,
// nullability, and whether the column is a primary key.
func (r ResHandle) createTableSQL() string {
	sql := strings.Builder{}

	sql.WriteString(fmt.Sprintf("create table %s (", egostrings.SQLIdentifier(r.Table)))

	for index, column := range r.Columns {
		if index > 0 {
			sql.WriteRune(',')
		}

		sql.WriteString(egostrings.SQLIdentifier(column.SQLName))
		sql.WriteRune(' ')
		sql.WriteString(column.SQLType)

		if column.Nullable {
			// NULL is the correct SQL keyword for an explicitly nullable column.
			// The invalid keyword "nullable" previously used here is not recognized
			// by either PostgreSQL or SQLite.
			sql.WriteString(" NULL")
		}

		if column.Primary {
			sql.WriteString(" primary key")
		}
	}

	sql.WriteRune(')')

	return sql.String()
}

// Generate the SQL used to determine if a given resource table exists.
func (r ResHandle) doesTableExistSQL() string {
	sql := fmt.Sprintf("select * from %s where 1=0", egostrings.SQLIdentifier(r.Table))

	return sql
}

// Generate the code to insert values into a resource type table. The
// SQL includes the column names, and uses "$" substitution operators
// for the values clause. This assumes that the user of this statement
// will use the same column order array to specify the values.
func (r ResHandle) insertSQL() string {
	sql := strings.Builder{}

	sql.WriteString(fmt.Sprintf("insert into %s(", r.Table))

	for index, column := range r.Columns {
		if index > 0 {
			sql.WriteString(", ")
		}

		sql.WriteString(egostrings.SQLIdentifier(column.SQLName))
	}

	sql.WriteString(") values(")

	for index := range r.Columns {
		if index > 0 {
			sql.WriteString(", ")
		}

		sql.WriteString(fmt.Sprintf("$%d", index+1))
	}

	sql.WriteString(")")

	return sql.String()
}

// Generate the SQL to update one or more resources. The generated
// code does not include filters which must be added to the SQL if
// needed. This includes a set clause for each column and value in
// the resource type.
func (r ResHandle) updateSQL() string {
	sql := strings.Builder{}

	sql.WriteString(fmt.Sprintf("update %s set ", r.Table))

	for index, column := range r.Columns {
		if index > 0 {
			sql.WriteString(", ")
		}

		sql.WriteString(egostrings.SQLIdentifier(column.SQLName))
		sql.WriteString(fmt.Sprintf(" = $%d", index+1))
	}

	return sql.String()
}

// Generate the SQL to delete one or more resources from the
// table.
func (r ResHandle) deleteRowSQL() string {
	return fmt.Sprintf("delete from %s ", r.Table)
}
