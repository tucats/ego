package tables

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/server/tables/parsing"
	"github.com/tucats/ego/internal/util/strings"
)

// formAbstractUpdateQuery builds an UPDATE statement for the abstract row update path.
// It returns the SQL string, the (possibly extended) parameter slice, and any error.
// The row ID, when present, is appended as the last $N parameter rather than being
// embedded as a string literal in the WHERE clause.
//
// An issue was found here where "table" used to be re-derived by 
// matching u.Path against the pattern "/tables/{{name}}/rows" via
// runtime_strings.ParseURLPattern. That matcher requires the URL to 
// have no more path segments than the pattern, so it could only ever
// match the legacy, DSN-less shape "/tables/{table}/rows" (4 segments).
// The actual DSN-scoped route this function is called from, 
// "/dsns/{dsn}/tables/{table}/rows" (6 // segments), always failed to 
// match -- silently returning ("", nil, nil):  an empty SQL string, 
// no error. The caller (UpdateAbstractRows) then ran db.Exec("") and 
// called RowsAffected() on the result, which SQLite returns as nil for 
// a no-op empty statement, so that call panicked (a
// 500 masking what should have been a successful, silent no-op that
// updated nothing). The caller already computes the correct,
// provider-qualified table name once via parsing.FullName() before
// calling this function -- there is no reason to re-derive it from the
// URL a second time, incorrectly, so it is now passed straight in.
func formAbstractUpdateQuery(u *url.URL, table string, items []string, values []any) (string, []any, error) {
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

		result.WriteString(egostrings.SQLIdentifier(key))
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
		result.WriteString(" ")
		result.WriteString(where)
	}

	return result.String(), params, nil
}

// formAbstractInsertQuery builds an INSERT statement for the abstract row
// insert path. "table" is the caller's already-resolved, provider-qualified
// table name -- see the longer explanation on formAbstractUpdateQuery above
// for why this is no longer re-derived from a URL here.
func formAbstractInsertQuery(table string, columns []string, values []any) (string, []any) {
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

		result.WriteString(egostrings.SQLIdentifier(key))
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
