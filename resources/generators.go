package resources

import (
	"fmt"
	"strconv"
	"strings"
)

func (r ResHandle) readRowSQL() string {
	sql := strings.Builder{}

	sql.WriteString("select ")

	for index, column := range r.Columns {
		if index > 0 {
			sql.WriteRune(',')
		}

		sql.WriteString(strconv.Quote(column.SQLName))
	}

	sql.WriteString(fmt.Sprintf(" from \"%s\" ", r.Table))

	return sql.String()
}

func (r ResHandle) createTableSQL() string {
	sql := strings.Builder{}

	sql.WriteString(fmt.Sprintf("create table \"%s\" (", r.Table))

	for index, column := range r.Columns {
		if index > 0 {
			sql.WriteRune(',')
		}

		sql.WriteString(strconv.Quote(column.SQLName))
		sql.WriteRune(' ')
		sql.WriteString(column.SQLType)

		if column.Nullable {
			sql.WriteString(" nullable")
		}

		if column.Primary {
			sql.WriteString(" primary key")
		}
	}

	sql.WriteRune(')')

	return sql.String()
}

func (r ResHandle) doesTableExistSQL() string {
	sql := fmt.Sprintf("select * from \"%s\" where 1=0", r.Table)

	return sql
}

func (r ResHandle) insertSQL() string {
	sql := strings.Builder{}

	sql.WriteString(fmt.Sprintf("insert into %s(", r.Table))

	for index, column := range r.Columns {
		if index > 0 {
			sql.WriteString(", ")
		}

		sql.WriteString(strconv.Quote(column.SQLName))
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

func (r ResHandle) updateSQL() string {
	sql := strings.Builder{}

	sql.WriteString(fmt.Sprintf("update %s set ", r.Table))

	for index, column := range r.Columns {
		if index > 0 {
			sql.WriteString(", ")
		}

		sql.WriteString(strconv.Quote(column.SQLName))
		sql.WriteString(fmt.Sprintf(" = $%d", index+1))
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
