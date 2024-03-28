package tables

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	runtime_strings "github.com/tucats/ego/runtime/strings"
	"github.com/tucats/ego/server/tables/parsing"
)

func formAbstractUpdateQuery(u *url.URL, user string, items []string, values []interface{}) (string, error) {
	var (
		result      strings.Builder
		filterCount int
		hasRowID    = -1
	)

	if u == nil {
		return "", nil
	}

	for pos, name := range items {
		if name == defs.RowIDName {
			hasRowID = pos
		}
	}

	parts, ok := runtime_strings.ParseURLPattern(u.Path, "/tables/{{name}}/rows")
	if !ok {
		return "", nil
	}

	tableItem, ok := parts["name"]
	if !ok {
		return "", nil
	}

	// Get the table name and filter list
	table, _ := parsing.FullName(user, data.String(tableItem))

	result.WriteString(updateVerb)
	result.WriteRune(' ')

	result.WriteString(table)

	// Loop over the item names and add SET clauses for each one. We always
	// ignore the rowid value because you cannot update it on an UPDATE call;
	// it is only set on an insert.
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

	where, err := parsing.WhereClause(parsing.FiltersFromURL(u))
	if err != nil {
		return "", err
	}

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

	return result.String(), nil
}

func formAbstractInsertQuery(u *url.URL, user string, columns []string, values []interface{}) (string, []interface{}) {
	var result strings.Builder

	if u == nil {
		return "", nil
	}

	parts, ok := runtime_strings.ParseURLPattern(u.Path, "/tables/{{name}}/rows")
	if !ok {
		return "", nil
	}

	tableItem, ok := parts["name"]
	if !ok {
		return "", nil
	}

	// Get the table name.
	table, _ := parsing.FullName(user, data.String(tableItem))

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
