package parsing

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/util/strings"
	"github.com/tucats/ego/internal/errors"
	runtime_strings "github.com/tucats/ego/internal/runtime/strings"
	"github.com/tucats/ego/internal/language/tokenizer"
	"github.com/tucats/ego/internal/util"
)

func FormSelectorDeleteQuery(u *url.URL, filter []string, columns string, table string, user string, verb string, provider string) (string, error) {
	var result strings.Builder

	// Get the table name. If it doesn't already have a schema part, then assign
	// the username as the schema.
	table, _ = FullName(provider, user, table)

	result.WriteString(verb)

	if verb == selectVerb {
		writeSpaceString(&result, ColumnList(columns))
	}

	writeSpaceString(&result, "FROM "+table)

	where, err := WhereClause(filter)
	if err != nil {
		return "", err
	}

	if where != "" {
		writeSpaceString(&result, where)
	}

	if sort := SortList(u); sort != "" && verb == selectVerb {
		writeSpaceString(&result, sort)
	}

	if paging := PagingClauses(u); paging != "" && verb == selectVerb {
		writeSpaceString(&result, paging)
	}

	return result.String(), nil
}

func FormUpdateQuery(u *url.URL, user, provider string, columns []defs.DBColumn, items map[string]any) (string, []any, error) {
	var result strings.Builder

	if u == nil {
		return "", nil, errors.ErrURLNotFound
	}

	// Two possible URL patterns: /tables/{{name}}/rows or /dsns/{{dsn}}/tables/{{name}}/rows
	parts, ok := runtime_strings.ParseURLPattern(u.Path, "/tables/{{name}}/rows")
	if !ok {
		parts, ok = runtime_strings.ParseURLPattern(u.Path, "/dsns/{{dsn}}/tables/{{name}}/rows")
		if !ok {
			return "", nil, errors.ErrInvalidURL
		}
	}

	tableItem, ok := parts["name"]
	if !ok {
		return "", nil, errors.ErrMissingTableName
	}

	// Get the table name and make sure it is fully qualified
	table, _ := FullName(provider, user, data.String(tableItem))

	result.WriteString(updateVerb)
	writeSpaceString(&result, table)

	keys := util.InterfaceMapKeys(items)
	keyCount := len(keys)

	if _, found := items[defs.RowIDName]; found {
		keyCount--
	}

	values := make([]any, keyCount)

	// Loop over the item names and add SET clauses for each one. We always
	// ignore the rowid value because you cannot update it on an UPDATE call;
	// it is only set on an insert.
	filterCount := 0

	for _, key := range keys {
		if key == defs.RowIDName {
			continue
		}

		if v, ok := items[key]; ok && v == nil {
			// Explicit null — bind nil so the database stores NULL.
			values[filterCount] = nil
		} else {
			// Step 1: convert the raw value to the correct Go type for this column.
			v, err := CoerceToColumnType(key, items[key], columns)
			if err != nil {
				return "", nil, err
			}

			// Step 2: format any time.Time value for the target provider.
			// (RFC 3339 string for SQLite, native time.Time for PostgreSQL.)
			v = bindTimeValue(v, provider)

			values[filterCount] = v
		}

		if filterCount == 0 {
			writeSpaceString(&result, "SET ")
		} else {
			result.WriteString(",")
		}

		filterCount++

		result.WriteString(fmt.Sprintf("%s=$%d", egostrings.SQLIdentifier(key), filterCount))
	}

	where, err := WhereClause(FiltersFromURL(u))
	if err != nil {
		return "", nil, err
	}

	// If the items we are updating includes a non-empty rowID, then graft it onto
	// the filter string.
	if id, found := items[defs.RowIDName]; found {
		idString := data.String(id)
		if idString != "" {
			if where == "" {
				where = "WHERE " + defs.RowIDName + " = '" + idString + "'"
			} else {
				where = where + " " + defs.RowIDName + " = '" + idString + "'"
			}
		}
	}

	if where == "" && settings.GetBool(defs.TablesServerEmptyFilterError) {
		return "", nil, errors.ErrTaskFilterRequired
	}

	// If we have a filter string now, add it to the query.
	if where != "" {
		writeSpaceString(&result, where)
	}

	return result.String(), values, nil
}

func writeSpaceString(b *strings.Builder, s string) {
	if !strings.HasSuffix(b.String(), " ") {
		b.WriteRune(' ')
	}

	b.WriteString(s)
}

// FormInsertQuery builds a parameterized SQL INSERT statement for the named table and
// returns the query string together with the ordered slice of parameter values that must
// be passed to db.Exec.
//
// The function performs two transformations on each value:
//  1. CoerceToColumnType converts the raw Go value (typically decoded from JSON, so often
//     a string or float64) into the Go type expected by the database driver.
//  2. bindTimeValue converts any resulting time.Time to the correct driver representation:
//     an RFC 3339 string for SQLite (TEXT column) or a native time.Time for PostgreSQL
//     (TIMESTAMP WITH TIME ZONE column).
func FormInsertQuery(table string, user string, provider string, columns []defs.DBColumn, items map[string]any) (string, []any, error) {
	var (
		err    error
		result strings.Builder
	)

	fullyQualifiedName, _ := FullName(provider, user, table)

	result.WriteString(insertVerb)
	result.WriteString(" INTO ")
	result.WriteString(fullyQualifiedName)

	keys := util.InterfaceMapKeys(items)
	values := make([]any, len(items))

	// Write the column-name list: INSERT INTO "table"("col1","col2",...)
	for i, key := range keys {
		if i == 0 {
			result.WriteRune('(')
		} else {
			result.WriteRune(',')
		}

		result.WriteString(egostrings.SQLIdentifier(key))
	}

	result.WriteString(") VALUES (")

	// Build the placeholder list ($1,$2,...) and populate the values slice.
	for i, key := range keys {
		v := items[key]

		if v != nil {
			// Step 1: convert the raw value to the proper Go type for this column
			// (e.g. parse "2006-01-02T15:04:05Z" into time.Time for a timestamp column).
			v, err = CoerceToColumnType(key, v, columns)
			if err != nil {
				return "", nil, err
			}

			// Step 2: format any time.Time value in the way the target provider expects
			// (RFC 3339 string for SQLite, native time.Time for PostgreSQL).
			v = bindTimeValue(v, provider)
		}

		values[i] = v

		if i > 0 {
			result.WriteString(",")
		}

		result.WriteString(fmt.Sprintf("$%d", i+1))
	}

	result.WriteRune(')')

	return result.String(), values, err
}

// CoerceToColumnType looks up the named column in the columns slice and converts the
// supplied value v to the Go type that matches the column's declared SQL type.  This
// ensures that the correct driver-level type is bound to each placeholder ($1, $2, …)
// when the query is eventually executed.
//
// For most scalar types the conversion is straightforward (string, int, float64, bool).
// For date/time types the logic is slightly more involved; see the inline comments.
//
// Returns the (possibly converted) value and any conversion error.  Returns
// errors.ErrInvalidColumnName if key is not found in the columns list (the row ID
// pseudo-column is exempt from this check).
func CoerceToColumnType(key string, v any, columns []defs.DBColumn) (any, error) {
	var (
		err   error
		found bool
	)

	// Walk the column list looking for a column whose name matches key.
	for _, column := range columns {
		if column.Name == key {
			// Lower-case the column type so the comparisons below are case-insensitive.
			// The type names in DBColumn.Type are normalized in getColumnInfo(), but we
			// guard here too so this function stays safe to call with un-normalized metadata.
			switch strings.ToLower(column.Type) {
			case "char", "string", "nullstring":
				// Plain string — just ensure the Go type is string.
				v = data.String(v)

			case "float", "double", "float64", "nullfloat64":
				v, err = data.Float64(v)
				if err != nil {
					return nil, err
				}

			case "float32", "single", "nullfloat32":
				v, err = data.Float32(v)
				if err != nil {
					return nil, err
				}

			case "bool", "boolean", "nullbool":
				v, err = data.Bool(v)
				if err != nil {
					return nil, err
				}

			case "int", "integer", "nullint":
				v, err = data.Int(v)
				if err != nil {
					return nil, err
				}

			case "int32", "nullint32":
				v, err = data.Int32(v)
				if err != nil {
					return nil, err
				}

			case "int64", "nullint64":
				v, err = data.Int64(v)
				if err != nil {
					return nil, err
				}

			// Time-related columns.  All variants of the column type name (portable
			// lowercase names produced by getColumnInfo, plus raw driver names that might
			// appear in non-normalized metadata) are collapsed into one case.
			//
			// Three sub-cases:
			//  1. nil   — produce a zero time.Time{} so the caller sees a typed zero value
			//             rather than an untyped nil.  Note: FormInsertQuery / FormUpdateQuery
			//             guard against nil before calling here, so this branch is mainly a
			//             safety net for other call sites.
			//  2. time.Time — the value is already the correct Go type (e.g. returned
			//             directly by the PostgreSQL driver on a SELECT).  Pass it through
			//             unchanged.
			//  3. anything else — treat it as a string and parse it with dateparse.ParseAny,
			//             which accepts RFC 3339, RFC 822, common US/ISO date formats, etc.
			//             This is the path taken when a JSON string such as
			//             "2006-01-02T15:04:05Z" arrives from a REST client.
			case "timestamp", "timestamptz", "timestamp with time zone",
				"time", "time with time zone",
				"date", "datetime":
				if v == nil {
					// Produce a typed zero value rather than leaving v as untyped nil.
					v = time.Time{}
				} else if _, isTime := v.(time.Time); isTime {
					// The PostgreSQL driver (lib/pq) decodes TIMESTAMP WITH TIME ZONE
					// columns as native Go time.Time values.  Nothing to do here.
					break
				} else {
					// Parse a string representation.  dateparse.ParseAny handles a wide
					// variety of formats so callers are not locked into RFC 3339.
					v, err = dateparse.ParseAny(data.String(v))
					if err != nil {
						return nil, errors.New(err)
					}
				}
			}

			found = true

			break
		}
	}

	if !found && key != defs.RowIDName {
		return nil, errors.ErrInvalidColumnName.Context(key)
	}

	return v, nil
}

// bindTimeValue converts a time.Time to the appropriate Go value for the target
// database provider before it is placed into the SQL parameter slice.
//
// Provider-specific behavior:
//   - SQLite: stores all date/time values as RFC 3339 strings (TEXT affinity).
//     We format to UTC RFC 3339 so the stored text is unambiguous and can be
//     round-tripped by dateparse.ParseAny on the read path.
//   - PostgreSQL: the lib/pq driver accepts a native time.Time directly for
//     TIMESTAMP WITH TIME ZONE, TIME, and DATE columns.
//
// If v is not a time.Time the function returns v unchanged, making it safe to
// call unconditionally after CoerceToColumnType.
//
// The function cannot return an error because it is an inner helper called for
// every value in an INSERT/UPDATE parameter list.  If an unrecognized provider is
// supplied the time.Time is passed through as-is, which is the safest fallback
// (most drivers accept native time.Time).  The calling handler should have
// validated the provider before reaching this point.
//
// To add a new provider: add a case in the switch below.
func bindTimeValue(v any, provider string) any {
	t, ok := v.(time.Time)
	if !ok {
		// Not a time value — nothing to convert.
		return v
	}

	switch {
	case strings.EqualFold(provider, defs.SqliteProvider) ||
		strings.EqualFold(provider, defs.DeprecatedSqliteProvider):
		// SQLite stores dates/times as TEXT.  Format as UTC RFC 3339 so the stored
		// string is consistent and parses back cleanly via dateparse.ParseAny.
		return t.UTC().Format(time.RFC3339)

	case strings.EqualFold(provider, defs.PostgresProvider):
		// lib/pq binds time.Time natively to TIMESTAMP WITH TIME ZONE / DATE / TIME.
		return t

	default:
		// Unknown provider — pass the time.Time value through unchanged.
		// Most SQL drivers accept time.Time via the driver.Value interface.
		return t
	}
}

func FormCreateQuery(u *url.URL, user string, hasAdminPrivileges bool, items []defs.DBColumn, sessionID int, w http.ResponseWriter, provider string, useRowID bool) (string, error) {
	var (
		err    error
		result strings.Builder
	)

	if u == nil {
		return "", errors.ErrURLNotFound
	}

	// Two possible URL patterns: /tables/{{name}} or /dsns/{{dsn}}/tables/{{name}}
	parts, ok := runtime_strings.ParseURLPattern(u.Path, "/tables/{{name}}")
	if !ok {
		parts, ok = runtime_strings.ParseURLPattern(u.Path, "/dsns/{{dsn}}/tables/{{name}}")
		if !ok {
			return "", errors.ErrInvalidURL
		}
	}

	tableItem, ok := parts["name"]
	if !ok {
		return "", errors.ErrInvalidURL
	}

	// Resolve the table name to the form expected by the target provider.
	// SQLite has no schema concept, so the name is used as-is.
	// PostgreSQL qualifies it with the user/schema name.
	// To support a new provider: add a case and produce the appropriate fully-qualified name.
	table := data.String(tableItem)
	wasFullyQualified := false

	switch provider {
	case defs.SqliteProvider, defs.DeprecatedSqliteProvider:
		// SQLite: no schema qualification needed; use the plain (unquoted) table name.

	case defs.PostgresProvider:
		table, wasFullyQualified = FullName(provider, user, data.String(tableItem))
		// Multi-part names (schema.table) require admin privileges when the schema
		// is not the current user's own schema.
		if !wasFullyQualified && !hasAdminPrivileges {
			util.ErrorResponse(w, sessionID, errors.ErrNoPrivilegeForOperation.Error(), http.StatusForbidden)

			return "", errors.ErrNoPrivilegeForOperation
		}

	default:
		return "", errors.ErrUnsupportedDatabase.Context(provider)
	}

	result.WriteString("CREATE TABLE ")
	result.WriteString(table)

	// See if the column data already contains a row ID value; if not,
	// add it in to the table definition.
	hasRowID := false

	for _, column := range items {
		if column.Name == defs.RowIDName {
			hasRowID = true

			break
		}
	}

	if useRowID && !hasRowID {
		items = append(items, defs.DBColumn{
			Name: defs.RowIDName,
			Type: data.StringTypeName,
			Unique: defs.BoolValue{
				Specified: true,
				Value:     true,
			},
		})
	}

	for i, column := range items {
		if i == 0 {
			result.WriteRune('(')
		} else {
			result.WriteString(", ")
		}

		result.WriteString(egostrings.SQLIdentifier(column.Name))
		result.WriteRune(' ')

		nativeType := MapColumnType(column.Type, provider)
		result.WriteString(nativeType)

		if column.Unique.Specified {
			if column.Unique.Value {
				result.WriteString(" UNIQUE")
			}
		}

		if column.Nullable.Specified {
			if !column.Nullable.Value {
				result.WriteString(" NOT NULL ")
			} else {
				result.WriteString(" NULL ")
			}
		}
	}

	result.WriteRune(')')

	return result.String(), err
}

func formWhereExpressions(filters []string) (string, error) {
	var result strings.Builder

	for i, clause := range filters {
		tokens := tokenizer.New(clause, true)
		if tokens.AtEnd() {
			continue
		}

		if i > 0 {
			result.WriteString(" AND ")
		}

		for {
			clause, err := filterClause(tokens, sqlDialect)
			if err != nil {
				return "", err
			}

			result.WriteString(clause)

			if !tokens.IsNext(tokenizer.CommaToken) {
				break
			}

			result.WriteString(" AND ")
		}
	}

	return result.String(), nil
}

func FormCondition(condition string) (string, error) {
	var (
		err    error
		result strings.Builder
	)

	tokens := tokenizer.New(condition, true)
	if tokens.AtEnd() {
		return "", err
	}

	for {
		clause, err := filterClause(tokens, egoDialect)
		if err != nil {
			return SyntaxErrorPrefix + err.Error(), err
		}

		result.WriteString(clause)

		if !tokens.IsNext(tokenizer.CommaToken) {
			break
		}

		result.WriteString(" && ")
	}

	return result.String(), err
}

func QueryParameters(source string, args map[string]string) (string, error) {
	var err error

	quote := "\""
	if q, found := args["quote"]; found {
		quote = q
	}

	// Before anything else, let's see if the table name was specified,
	// and it contains a "dot" notation. If so, replace the schema name
	// with the dot name prefix.
	if tableName, ok := args[defs.TableParameterName]; ok {
		dot := strings.Index(tableName, ".")
		if dot >= 0 {
			args[defs.TableParameterName] = quote + StripQuotes(tableName[dot+1:]) + quote
			args[defs.SchemaParameterName] = quote + StripQuotes(tableName[:dot]) + quote
		}
	}

	// Skip through the substitution strings provided and do any replace
	// needed.
	result := source

	for k, v := range args {
		v, err := SQLEscape(v)
		if err != nil {
			return "", err
		}

		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}

	return result, err
}

func filterClause(tokens *tokenizer.Tokenizer, dialect int) (string, error) {
	var result strings.Builder

	operator := tokens.Next()

	// Handle case of signed constant
	if operator.Spelling() == ("+") || operator.Spelling() == ("-") {
		operator = tokens.NewToken(tokenizer.ValueTokenClass, operator.Spelling()+tokens.Next().Spelling())
	}

	// Handle case of NULL constant
	if operator.Spelling() == (".") || tokens.Peek(1).Spelling() == ("nil") {
		tokens.Next()
		operator = tokens.NewToken(tokenizer.ValueTokenClass, "NULL")
	}

	isName := operator.IsIdentifier()

	if operator.IsIdentifier() && (operator.Spelling() == "true" || operator.Spelling() == "false") {
		isName = false
	}

	if !tokens.IsNext(tokenizer.StartOfListToken) {
		// Assume it's a constant value of some kind. Convert Ego strings to SQL strings.
		// Note that we have to test for the case of a value token that contains a single-
		// quoted string. If found, identify as a string.
		isString := operator.IsString()
		if !isString && operator.IsClass(tokenizer.ValueTokenClass) {
			isString = strings.HasPrefix(operator.Spelling(), "'")
		}

		operatorSpelling, err := SQLEscape(operator.Spelling())
		if err != nil {
			return "", err
		}

		if isString {
			switch dialect {
			case sqlDialect:
				operatorSpelling = "'" + operatorSpelling + "'"
			case egoDialect:
				operatorSpelling = strconv.Quote(operatorSpelling)
			}
		}

		if isName && dialect == sqlDialect {
			operatorSpelling = egostrings.SQLIdentifier(operatorSpelling)
		}

		return operatorSpelling, nil
	}

	prefix := ""
	infix := ""
	listAllowed := false

	// Contains is weird, so handle it separately. Note that we pay attention to the *ALL form
	// as meaning all the cases must be true, versus the default of any of the cases are true.
	if util.InList(strings.ToUpper(operator.Spelling()), "CONTAINS", "HAS", "HASANY", "CONTAINSALL", "HASALL") {
		var conjunction string

		switch dialect {
		case sqlDialect:
			conjunction = " OR "

		case egoDialect:
			conjunction = " || "
		}

		if util.InList(strings.ToUpper(operator.Spelling()), "CONTAINSALL", "HASALL") {
			switch dialect {
			case sqlDialect:
				conjunction = " AND "

			case egoDialect:
				conjunction = " && "
			}
		}

		term, e := filterClause(tokens, dialect)
		if e != nil {
			return "", errors.New(e)
		}

		valueCount := 0

		for tokens.IsNext(tokenizer.CommaToken) {
			if valueCount > 0 {
				result.WriteString(conjunction)
			}

			valueCount++

			value, e := filterClause(tokens, dialect)
			if e != nil {
				return "", errors.New(e)
			}

			switch dialect {
			case sqlDialect:
				// Building a string like:
				//    position('evil' in classification) > 0
				result.WriteString("POSITION(")
				result.WriteString(value)
				result.WriteString(" IN ")
				result.WriteString(term)
				result.WriteString(") > 0")

			case egoDialect:
				result.WriteString("strings.Index(")
				result.WriteString(term)
				result.WriteString(",")
				result.WriteString(value)
				result.WriteString(") >= 0 ")
			}
		}

		if !tokens.IsNext(tokenizer.EndOfListToken) {
			return tokens.GetSource(), errors.ErrMissingParenthesis
		}

		return result.String(), nil
	}

	// Handle regular old monadic and diadic operators as a group.
	switch strings.ToUpper(operator.Spelling()) {
	case "EQ":
		switch dialect {
		case sqlDialect:
			infix = "="

		case egoDialect:
			infix = "=="
		}

	case "LT":
		infix = "<"

	case "LE":
		infix = "<="

	case "GT":
		infix = ">"

	case "GE":
		infix = ">="

	case "AND":
		switch dialect {
		case sqlDialect:
			infix = " AND "

		case egoDialect:
			infix = "&&"
		}

		listAllowed = true

	case "OR":
		switch dialect {
		case sqlDialect:
			infix = " OR "

		case egoDialect:
			infix = "||"
		}

		listAllowed = true

	case "NOT":
		switch dialect {
		case sqlDialect:
			prefix = " NOT "

		case egoDialect:
			prefix = " !"
		}

	default:
		return "", errors.ErrUnexpectedToken.Context(operator)
	}

	if prefix != "" {
		term, _ := filterClause(tokens, dialect)

		result.WriteString(prefix)
		result.WriteString(" ")
		result.WriteString(term)
	} else {
		termCount := 0
		term, _ := filterClause(tokens, dialect)

		result.WriteString("(")

		for {
			termCount++

			result.WriteString(term)

			if !tokens.IsNext(tokenizer.CommaToken) {
				if termCount < 2 {
					return "", errors.ErrInvalidList
				}

				if termCount > 2 && !listAllowed {
					return "", errors.ErrInvalidList
				}

				break
			}

			// special case for testing for NULL values
			nextToken := tokens.Peek(1)
			if infix == "=" && nextToken.Spelling() == "." && tokens.Peek(2).Spelling() == "nil" {
				tokens.Advance(2)
				result.WriteString(" IS NULL ")

				term = ""
			} else {
				result.WriteString(" ")
				result.WriteString(infix)
				result.WriteString(" ")

				term, _ = filterClause(tokens, dialect)
			}
		}

		result.WriteString(")")
	}

	if !tokens.IsNext(tokenizer.EndOfListToken) {
		return tokens.GetSource(), errors.ErrMissingParenthesis
	}

	return result.String(), nil
}

// whereClause accepts a list of filter parameters, and converts them
// to a SQL WHERE clause (including the 'WHERE' token).
func WhereClause(filters []string) (string, error) {
	if len(filters) == 0 {
		return "", nil
	}

	clause, err := formWhereExpressions(filters)
	if err != nil {
		return "", err
	}

	return "WHERE " + clause, nil
}

// DefaultRowLimit is the SQL LIMIT applied when no "limit" query parameter is given.
const DefaultRowLimit = 1000

func PagingClauses(u *url.URL) string {
	var result strings.Builder

	if u == nil {
		return ""
	}

	limit := DefaultRowLimit

	values := u.Query()
	for k, v := range values {
		if KeywordMatch(k, "limit", "count") {
			if len(v) == 1 {
				if i, err := egostrings.Atoi(v[0]); err == nil && i > 0 {
					limit = i
				}
			}
		}
	}

	result.WriteString(" LIMIT ")
	result.WriteString(strconv.Itoa(limit))

	for k, v := range values {
		if KeywordMatch(k, "start", "offset") {
			start := 0

			if len(v) == 1 {
				if i, err := egostrings.Atoi(v[0]); err == nil {
					start = i
				}
			}

			if start != 0 {
				result.WriteString(" OFFSET ")
				// Note that offset is zero-based, so subtract 1
				result.WriteString(strconv.Itoa(start - 1))
			}
		}
	}

	return result.String()
}
