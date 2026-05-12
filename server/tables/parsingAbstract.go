package tables

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	runtime_strings "github.com/tucats/ego/runtime/strings"
	"github.com/tucats/ego/server/tables/parsing"
)

// formAbstractUpdateQuery builds an UPDATE statement for the abstract row update path.
// It returns the SQL string, the (possibly extended) parameter slice, and any error.
// The row ID, when present, is appended as the last $N parameter rather than being
// embedded as a string literal in the WHERE clause.
func formAbstractUpdateQuery(u *url.URL, provider string, user string, items []string, values []any) (string, []any, error) {
	var (
		result      strings.Builder
		filterCount int
		hasRowID    = -1
	)

	if u == nil {
		return "", nil, nil
	}

	for pos, name := range items {
		if name == defs.RowIDName {
			hasRowID = pos
		}
	}

	parts, ok := runtime_strings.ParseURLPattern(u.Path, "/tables/{{name}}/rows")
	if !ok {
		return "", nil, nil
	}

	tableItem, ok := parts["name"]
	if !ok {
		return "", nil, nil
	}

	// Get the table name and filter list
	table, _ := parsing.FullName(provider, user, data.String(tableItem))

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

		result.WriteString(strconv.Quote(key))
		result.WriteString(fmt.Sprintf(" = $%d", filterCount))
	}

	where, err := parsing.WhereClause(parsing.FiltersFromURL(u))
	if err != nil {
		return "", nil, err
	}

	// If the items we are updating includes a non-empty rowID, append it as a
	// numbered parameter instead of embedding it as a string literal. This
	// prevents SQL injection through a crafted row ID value.
	params := values

	if hasRowID >= 0 {
		idString := data.String(values[hasRowID])

		if idString != "" {
			paramIdx := filterCount + 1

			clause := fmt.Sprintf("%s = $%d", defs.RowIDName, paramIdx)
			if where == "" {
				where = "WHERE " + clause
			} else {
				where = where + " AND " + clause
			}

			params = append(params, idString)
		}
	}

	// If we have a filter string now, add it to the query.
	if where != "" {
		result.WriteString(" " + where)
	}

	return result.String(), params, nil
}

func formAbstractInsertQuery(u *url.URL, provider string, user string, columns []string, values []any) (string, []any) {
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
	table, _ := parsing.FullName(provider, user, data.String(tableItem))

	result.WriteString(insertVerb)
	result.WriteString(" INTO ")
	result.WriteString(table)

	for i, key := range columns {
		if i == 0 {
			result.WriteRune('(')
		} else {
			result.WriteRune(',')
		}

		result.WriteString(strconv.Quote(key))
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
