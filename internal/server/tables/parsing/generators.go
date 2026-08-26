package parsing

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/tokenizer"
	runtime_strings "github.com/tucats/ego/internal/runtime/strings"
	"github.com/tucats/ego/internal/util"
	egostrings "github.com/tucats/ego/internal/util/strings"
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
		return "", errors.ErrSQLBuild.Clone().Chain(errors.New(err))
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
		return "", nil, errors.ErrSQLBuild.Clone().Chain(errors.ErrURLNotFound)
	}

	// Two possible URL patterns: /tables/{{name}}/rows or /dsns/{{dsn}}/tables/{{name}}/rows
	parts, ok := runtime_strings.ParseURLPattern(u.Path, "/tables/{{name}}/rows")
	if !ok {
		parts, ok = runtime_strings.ParseURLPattern(u.Path, "/dsns/{{dsn}}/tables/{{name}}/rows")
		if !ok {
			return "", nil, errors.ErrSQLBuild.Clone().Chain(errors.ErrInvalidURL.Context(u.Path))
		}
	}

	tableItem, ok := parts["name"]
	if !ok {
		return "", nil, errors.ErrSQLBuild.Clone().Chain(errors.ErrMissingTableName)
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
				return "", nil, errors.ErrSQLBuild.Clone().Chain(errors.New(err))
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
		return "", nil, errors.ErrSQLBuild.Clone().Chain(errors.New(err))
	}

	// If the items we are updating includes a non-empty rowID, then graft it onto
	// the filter string. The rowID value comes straight from the caller's request
	// body, so it must be bound as a query parameter ($N) rather than concatenated
	// into the SQL text -- a rowID value containing a "'" would otherwise be able
	// to break out of the WHERE clause and inject arbitrary SQL (this was a real,
	// unguarded SQL injection: every other value in this function is already bound
	// via $N, but this one was concatenated as a raw string literal instead).
	if id, found := items[defs.RowIDName]; found {
		idString := data.String(id)
		if idString != "" {
			filterCount++

			values = append(values, idString)

			rowIDClause := fmt.Sprintf("%s = $%d", egostrings.SQLIdentifier(defs.RowIDName), filterCount)

			if where == "" {
				where = "WHERE " + rowIDClause
			} else {
				// The existing filter clause already starts with "WHERE ", so the
				// rowID clause must be joined with "AND" to form valid SQL.
				where = where + " AND " + rowIDClause
			}
		}
	}

	if where == "" && settings.GetBool(defs.TablesServerEmptyFilterError) {
		return "", nil, errors.ErrSQLBuild.Clone().Chain(errors.ErrTaskFilterRequired)
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
				return "", nil, errors.ErrSQLBuild.Clone().Chain(errors.New(err))
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
// This is the single canonical implementation, used by the server's REST insert and
// update handlers, by the server's row *read* path, and by the "ego table insert" and
// "ego table update" CLI commands.  The CLI used to carry a near-duplicate of it that
// had drifted -- it handled "int16" the server did not, and handled only "date" and
// "datetime" among the time types -- so a value could be converted one way by the
// client and another way by the server (TIME-2).
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
			// An explicit null in a column that permits one needs no conversion at
			// all; it is bound as SQL NULL.  This must be checked before the switch,
			// because the time cases below turn a nil into a zero time.Time, which
			// would store an actual timestamp of January 1, year 1 instead of NULL.
			if v == nil && column.Nullable.Specified && column.Nullable.Value {
				return nil, nil
			}

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
					return nil, errors.ErrSQLCoerce.Context(column.Name).Chain(errors.New(err))
				}

			case "float32", "single", "nullfloat32":
				v, err = data.Float32(v)
				if err != nil {
					return nil, errors.ErrSQLCoerce.Context(column.Name).Chain(errors.New(err))
				}

			case "bool", "boolean", "nullbool":
				v, err = data.Bool(v)
				if err != nil {
					return nil, errors.ErrSQLCoerce.Context(column.Name).Chain(errors.New(err))
				}

			case "int", "integer", "nullint":
				v, err = data.Int(v)
				if err != nil {
					return nil, errors.ErrSQLCoerce.Context(column.Name).Chain(errors.New(err))
				}

			case "int16", "nullint16":
				v, err = data.Int16(v)
				if err != nil {
					return nil, errors.ErrSQLCoerce.Context(column.Name).Chain(errors.New(err))
				}

			case "int32", "nullint32":
				v, err = data.Int32(v)
				if err != nil {
					return nil, errors.ErrSQLCoerce.Context(column.Name).Chain(errors.New(err))
				}

			case "int64", "nullint64":
				v, err = data.Int64(v)
				if err != nil {
					return nil, errors.ErrSQLCoerce.Context(column.Name).Chain(errors.New(err))
				}

			// Time-related columns.  All variants of the column type name (portable
			// lowercase names produced by getColumnInfo, plus raw driver names that might
			// appear in non-normalized metadata) are collapsed into one case.
			//
			// Three sub-cases:
			//  1. nil   — produce a zero time.Time{} so the caller sees a typed zero value
			//             rather than an untyped nil.  Note: FormInsertQuery / FormUpdateQuery
			//             guard against nil before calling here, so this branch is mainly a
			//             safety net for other call sites.  A nil in a *nullable* column
			//             was already returned as NULL above and never reaches this point.
			//  2. time.Time — the value is already the correct Go type (e.g. returned
			//             directly by the PostgreSQL driver on a SELECT).  Pass it through
			//             unchanged.
			//  3. anything else — treat it as a string and parse it.
			//
			// The parse is deliberately the *strict* one.  A value reaching here is on
			// its way into (or out of) a database column, and bindTimeValue below folds
			// it to a UTC instant before storing, discarding the original text.  So an
			// abbreviation resolved against the wrong zone is not a transient mistake:
			// it becomes the stored record, reads back cleanly forever after, and cannot
			// be repaired without knowing what timezone the server process happened to be
			// configured with at the moment of the write.  Rejecting the input is
			// recoverable for the caller; accepting it silently is not (TIME-2).
			//
			// RFC 3339 ("2006-01-02T15:04:05Z07:00") is the documented contract for
			// clients -- see docs/TABLES.md -- and always states its offset numerically,
			// so it is unaffected by any of this.
			case "timestamp", "timestamptz", "timestamp with time zone",
				"time", "time with time zone",
				"date", "datetime":
				if v == nil {
					// Produce a typed zero value rather than leaving v as untyped nil.
					v = time.Time{}
				} else if t, isTime := v.(time.Time); isTime {
					// The PostgreSQL driver (lib/pq) decodes TIMESTAMP WITH TIME ZONE
					// columns as native Go time.Time values, but in a Location
					// reflecting the Postgres session/server timezone (a fixed
					// numeric offset), not time.UTC -- unlike sqlite3, whose
					// on-disk representation is UTC RFC 3339 text, so parsing it
					// back naturally yields time.UTC (see bindTimeValue's write-side
					// t.UTC() call below, and TIME-2's "normalized to UTC, whatever
					// timezone the server runs in" contract). Passing v through
					// unchanged here used to leave that server-timezone offset on
					// the value, so encoding/json's time.Time.MarshalJSON (used for
					// this row's response) formatted it as e.g. "...-04:00" instead
					// of the documented "...Z" -- a real value, just not the
					// UTC-normalized one every other provider round-trips. .UTC()
					// converts the *representation* only; the instant is unchanged.
					v = t.UTC()

					break
				} else {
					v, err = util.StrictParseTimestamp(data.String(v))
					if err != nil {
						return nil, errors.ErrSQLCoerce.Context(column.Name).Chain(errors.New(err))
					}
				}
			}

			found = true

			break
		}
	}

	if !found && key != defs.RowIDName {
		return nil, errors.ErrSQLCoerce.Clone().Chain(errors.ErrInvalidColumnName.Context(key))
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

func FormCreateQuery(u *url.URL, user string, hasAdminPrivileges bool, items []defs.DBColumn, provider string, useRowID bool) (string, error) {
	var (
		err    error
		result strings.Builder
	)

	if u == nil {
		return "", errors.ErrSQLBuild.Clone().Chain(errors.ErrURLNotFound)
	}

	// Two possible URL patterns: /tables/{{name}} or /dsns/{{dsn}}/tables/{{name}}
	parts, ok := runtime_strings.ParseURLPattern(u.Path, "/tables/{{name}}")
	if !ok {
		parts, ok = runtime_strings.ParseURLPattern(u.Path, "/dsns/{{dsn}}/tables/{{name}}")
		if !ok {
			return "", errors.ErrSQLBuild.Clone().Chain(errors.ErrInvalidURL.Context(u.Path))
		}
	}

	tableItem, ok := parts["name"]
	if !ok {
		return "", errors.ErrSQLBuild.Clone().Chain(errors.ErrInvalidURL.Context(u.Path))
	}

	// Resolve the table name to the form expected by the target provider.
	// SQLite has no schema concept, so the name is used as-is.
	// PostgreSQL qualifies it with the user/schema name.
	// To support a new provider: add a case and produce the appropriate fully-qualified name.
	table := data.String(tableItem)
	wasFullyQualified := false

	switch provider {
	case defs.SqliteProvider, defs.DeprecatedSqliteProvider:
		// SQLite: no schema qualification needed, but the table name still comes
		// straight from the URL path and MUST be quoted as a SQL identifier before
		// being embedded in the CREATE TABLE statement below. Without this, a table
		// name containing spaces, parentheses, or a SQL comment sequence ("--")
		// could inject arbitrary DDL fragments into the statement.
		table = egostrings.SQLIdentifier(table)

	case defs.PostgresProvider:
		table, wasFullyQualified = FullName(provider, user, data.String(tableItem))
		// Multi-part names (schema.table) require admin privileges when the schema
		// is not the current user's own schema.
		//
		// This function used to write an error response directly here (it
		// took an http.ResponseWriter and session ID for exactly that
		// purpose) -- the one place in FormCreateQuery that touched the
		// response at all, every other error path here just returns an
		// error and lets the caller respond. TableCreate (tables.go), the
		// only caller, saw the non-nil error returned below and wrote its
		// own response on top of the one already sent: a second WriteHeader
		// (superfluous; the first status, 403, wins on the wire) and a
		// second JSON body concatenated onto the first, while the caller's
		// own code believed it had sent 400 (REST-3 7.5). Just return the
		// typed error now and let the caller classify it via
		// dberrors.PayloadStatus, which already maps ErrNoPrivilegeForOperation
		// to 403 -- consistent with every other error this function returns.
		if !wasFullyQualified && !hasAdminPrivileges {
			return "", errors.ErrNoPrivilegeForOperation
		}

	default:
		return "", errors.ErrSQLBuild.Clone().Chain(errors.ErrUnsupportedDatabase.Context(provider))
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
		return "", errors.ErrSQLWhere.Clone().Chain(errors.New(err))
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
