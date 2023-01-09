package tables

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/functions"
)

func formAbstractUpdateQuery(u *url.URL, user string, items []string, values []interface{}) string {
	if u == nil {
		return ""
	}

	hasRowID := -1

	for pos, name := range items {
		if name == defs.RowIDName {
			hasRowID = pos
		}
	}

	parts, ok := functions.ParseURLPattern(u.Path, "/tables/{{name}}/rows")
	if !ok {
		return ""
	}

	tableItem, ok := parts["name"]
	if !ok {
		return ""
	}

	// Get the table name and filter list
	table, _ := fullName(user, data.String(tableItem))

	var result strings.Builder

	result.WriteString(updateVerb)
	result.WriteRune(' ')

	result.WriteString(table)

	// Loop over the item names and add SET clauses for each one. We always
	// ignore the rowid value because you cannot update it on an UPDATE call;
	// it is only set on an insert.
	filterCount := 0

	for _, key := range items {
		if filterCount == 0 {
			result.WriteString(" SET ")
		} else {
			result.WriteString(", ")
		}

		filterCount++

		result.WriteString("\"" + key + "\"")
		result.WriteString(fmt.Sprintf(" = $%d", filterCount))
	}

	where := whereClause(filtersFromURL(u))

	// If the items we are updating includes a non-empty rowID, then graft it onto
	// the filter string.
	if hasRowID >= 0 {
		idString := data.String(values[hasRowID])
		if idString != "" {
			if where == "" {
				where = "WHERE " + defs.RowIDName + " = '" + idString + "'"
			} else {
				where = where + " " + defs.RowIDName + " = '" + idString + "'"
			}
		}
	}

	// If we have a filter string now, add it to the query.
	if where != "" {
		result.WriteString(" " + where)
	}

	return result.String()
}

func formAbstractInsertQuery(u *url.URL, user string, columns []string, values []interface{}) (string, []interface{}) {
	if u == nil {
		return "", nil
	}

	parts, ok := functions.ParseURLPattern(u.Path, "/tables/{{name}}/rows")
	if !ok {
		return "", nil
	}

	tableItem, ok := parts["name"]
	if !ok {
		return "", nil
	}

	// Get the table name.
	table, _ := fullName(user, data.String(tableItem))

	var result strings.Builder

	result.WriteString(insertVerb)
	result.WriteString(" INTO ")
	result.WriteString(table)

	for i, key := range columns {
		if i == 0 {
			result.WriteRune('(')
		} else {
			result.WriteRune(',')
		}

		result.WriteString("\"" + key + "\"")
	}

	result.WriteString(") VALUES (")

	for i := range values {
		if i > 0 {
			result.WriteString(",")
		}

		result.WriteString(fmt.Sprintf("$%d", i+1))
	}

	result.WriteRune(')')

	return result.String(), values
}
