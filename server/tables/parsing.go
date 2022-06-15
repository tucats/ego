package tables

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

const (
	SQLDialect = 0
	EgoDialect = 1
)

// @tomcole probably need to dump this entirely and work on variadic substitution arguments in query!
func sqlEscape(source string) string {
	var result strings.Builder

	for _, ch := range source {
		if ch == '\'' || ch == ';' {
			return "INVALID-NAME"
		}

		result.WriteRune(ch)
	}

	return result.String()
}

// stripQuotes removes double quotes from the input string. Leading and trailing double-quotes
// are removed, as are internal "." quoted boudnaries. This prevents a name from being put through
// the fullName() processor multiple times and accumulate extra quots.
func stripQuotes(input string) string {
	return strings.TrimPrefix(
		strings.TrimSuffix(
			strings.ReplaceAll(input, "\".\"", "."),
			"\""),
		"\"")
}

func queryParameters(source string, args map[string]string) string {
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
			args[defs.TableParameterName] = quote + stripQuotes(tableName[dot+1:]) + quote
			args[defs.SchemaParameterName] = quote + stripQuotes(tableName[:dot]) + quote
		}
	}

	// Skip through the substition strings provided and do any replace
	// needed.
	result := source

	for k, v := range args {
		v = sqlEscape(v)
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}

	return result
}

func formWhereExpressions(filters []string) string {
	var result strings.Builder

	for i, clause := range filters {
		tokens := tokenizer.New(clause)
		if tokens.AtEnd() {
			continue
		}

		if i > 0 {
			result.WriteString(" AND ")
		}

		for {
			clause, err := filterClause(tokens, SQLDialect)
			if err != nil {
				return syntaxErrorPrefix + err.Error()
			}

			result.WriteString(clause)

			if !tokens.IsNext(",") {
				break
			}

			result.WriteString(" AND ")
		}
	}

	return result.String()
}

func formCondition(condition string) string {
	var result strings.Builder

	tokens := tokenizer.New(condition)
	if tokens.AtEnd() {
		return ""
	}

	for {
		clause, err := filterClause(tokens, EgoDialect)
		if err != nil {
			return syntaxErrorPrefix + err.Error()
		}

		result.WriteString(clause)

		if !tokens.IsNext(",") {
			break
		}

		result.WriteString(" && ")
	}

	return result.String()
}

func filterClause(tokens *tokenizer.Tokenizer, dialect int) (string, error) {
	var result strings.Builder

	operator := tokens.Next()
	isName := tokenizer.IsSymbol(operator)

	if operator == "true" || operator == "false" {
		isName = false
	}

	if !tokens.IsNext("(") {
		// Assume it's a constant value of some kind. Convert Ego strings to SQL strings
		isString := false

		if strings.HasPrefix(operator, "\"") && strings.HasSuffix(operator, "\"") {
			operator = stripQuotes(operator)
			isString = true
		} else {
			if strings.HasPrefix(operator, "'") && strings.HasSuffix(operator, "'") {
				operator = strings.TrimPrefix(strings.TrimSuffix(operator, "'"), "'")
				isString = true
			}
		}

		operator = sqlEscape(operator)

		if isString {
			switch dialect {
			case SQLDialect:
				operator = "'" + operator + "'"
			case EgoDialect:
				operator = "\"" + operator + "\""
			}
		}

		if isName && dialect == SQLDialect {
			operator = "\"" + operator + "\""
		}

		return operator, nil
	}

	prefix := ""
	infix := ""
	listAllowed := false

	// Contains is weird, so handle it separately. Note that we pay attention to the *ALL form
	// as meaning all the cases must be true, versus the default of any of the cases are true.
	if util.InList(strings.ToUpper(operator), "CONTAINS", "HAS", "HASANY", "CONTAINSALL", "HASALL") {
		var conjunction string

		switch dialect {
		case SQLDialect:
			conjunction = " OR "

		case EgoDialect:
			conjunction = " || "
		}

		if util.InList(strings.ToUpper(operator), "CONTAINSALL", "HASALL") {
			switch dialect {
			case SQLDialect:
				conjunction = " AND "

			case EgoDialect:
				conjunction = " && "
			}
		}

		term, e := filterClause(tokens, dialect)
		if !errors.Nil(e) {
			return "", e
		}

		valueCount := 0

		for tokens.IsNext(",") {
			if valueCount > 0 {
				result.WriteString(conjunction)
			}
			valueCount++

			value, e := filterClause(tokens, dialect)
			if !errors.Nil(e) {
				return "", e
			}

			switch dialect {
			case SQLDialect:
				// Building a string like:
				//    position('evil' in classification) > 0
				result.WriteString("POSITION(")
				result.WriteString(value)
				result.WriteString(" IN ")
				result.WriteString(term)
				result.WriteString(") > 0")

			case EgoDialect:
				result.WriteString("strings.Index(")
				result.WriteString(term)
				result.WriteString(",")
				result.WriteString(value)
				result.WriteString(") >= 0 ")
			}
		}

		if !tokens.IsNext(")") {
			return "", errors.New(errors.ErrMissingParenthesis)
		}

		return result.String(), nil
	}

	// Handle regular old monadic and diadic operators as a group.
	switch strings.ToUpper(operator) {
	case "EQ":
		switch dialect {
		case SQLDialect:
			infix = "="

		case EgoDialect:
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
		case SQLDialect:
			infix = " AND "

		case EgoDialect:
			infix = "&&"
		}

		listAllowed = true

	case "OR":
		switch dialect {
		case SQLDialect:
			infix = " OR "

		case EgoDialect:
			infix = "||"
		}

		listAllowed = true

	case "NOT":
		switch dialect {
		case SQLDialect:
			prefix = " NOT "

		case EgoDialect:
			prefix = " !"
		}

	default:
		return "", errors.New(errors.ErrUnexpectedToken).Context(operator)
	}

	if prefix != "" {
		term, _ := filterClause(tokens, dialect)
		result.WriteString(prefix + " " + term)
	} else {
		termCount := 0
		term, _ := filterClause(tokens, dialect)
		result.WriteString("(")
		for {
			termCount++
			result.WriteString(term)
			if !tokens.IsNext(",") {
				if termCount < 2 {
					return "", errors.New(errors.ErrInvalidList)
				}
				if termCount > 2 && !listAllowed {
					return "", errors.New(errors.ErrInvalidList)
				}

				break
			}

			result.WriteString(" " + infix + " ")

			term, _ = filterClause(tokens, dialect)
		}

		result.WriteString(")")
	}

	if !tokens.IsNext(")") {
		return "", errors.New(errors.ErrMissingParenthesis)
	}

	return result.String(), nil
}

func columnsFromURL(u *url.URL) string {
	parms := u.Query()
	result := strings.Builder{}

	for parm, values := range parms {
		if parm == defs.ColumnParameterName {
			for _, name := range values {
				if result.Len() > 0 {
					result.WriteRune(',')
				}

				result.WriteString(name)
			}
		}
	}

	return result.String()
}

func columnList(columnsParameter string) string {
	var result strings.Builder

	if columnsParameter == "" {
		return "*"
	}

	names := strings.Split(columnsParameter, ",")

	for _, name := range names {
		if len(name) == 0 {
			continue
		}

		if result.Len() > 0 {
			result.WriteRune(',')
		}

		result.WriteString("\"" + name + "\"")
	}

	if result.Len() == 0 {
		return "*"
	}

	return result.String()
}

func fullName(user, table string) (string, bool) {
	wasFullyQualified := true
	user = stripQuotes(user)
	table = stripQuotes(table)

	if dot := strings.Index(table, "."); dot < 0 {
		table = "\"" + user + "\".\"" + table + "\""
		wasFullyQualified = false
	} else {
		parts := strings.Split(table, ".")
		table = ""
		for n, part := range parts {
			if n > 0 {
				table = table + "."
			}
			table = table + "\"" + part + "\""
		}
	}

	return table, wasFullyQualified
}

// filtersFromURL extracts the filter parameters from the
// urL, and creates an array of strings for each filter
// expression found.
func filtersFromURL(u *url.URL) []string {
	result := make([]string, 0)

	q := u.Query()
	for param, values := range q {
		if strings.EqualFold(param, defs.FilterParameterName) {
			result = append(result, values...)
		}
	}

	return result
}

// whereClause accepts a list of filter parameters, and converts them
// to a SQL WHERE clause (including the 'WHERE' token).
func whereClause(filters []string) string {
	if len(filters) == 0 {
		return ""
	}

	clause := formWhereExpressions(filters)

	return " WHERE " + clause
}

// With a default user name string, and the current URL, determine if the
// username should be overridden with one from the URL.
func requestForUser(user string, u *url.URL) string {
	values := u.Query()
	for k, v := range values {
		if strings.EqualFold(k, defs.UserParameterName) {
			if len(v) > 0 {
				user = v[0]
			}
		}
	}

	return user
}

func pagingClauses(u *url.URL) string {
	var result strings.Builder

	if u == nil {
		return ""
	}

	values := u.Query()
	for k, v := range values {
		if keywordMatch(k, "limit", "count") {
			limit := 0

			if len(v) == 1 {
				if i, err := strconv.Atoi(v[0]); err == nil {
					limit = i
				}
			}

			if limit != 0 {
				result.WriteString(" LIMIT ")
				result.WriteString(strconv.Itoa(limit))
			}
		}

		if keywordMatch(k, "start", "offset") {
			start := 0

			if len(v) == 1 {
				if i, err := strconv.Atoi(v[0]); err == nil {
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

func sortList(u *url.URL) string {
	var result strings.Builder

	if u == nil {
		return ""
	}

	values := u.Query()
	ascending := true

	for k, v := range values {
		if keywordMatch(k, "sort", "order", "sort-by", "order-by") {
			for i, name := range v {
				if strings.HasPrefix(name, "~") {
					name = strings.TrimPrefix(name, "~")
					ascending = false
				}

				if i == 0 {
					result.WriteString(" ORDER BY ")
				} else {
					result.WriteString(",")
				}

				parts := strings.Split(name, ",")
				name := ""

				for n, part := range parts {
					if n > 0 {
						name = name + ","
					}

					name = name + "\"" + part + "\""
				}

				result.WriteString(name)
			}
		}
	}

	if result.Len() > 0 && !ascending {
		result.WriteString(" DESC")
	}

	return result.String()
}

func formSelectorDeleteQuery(u *url.URL, filter []string, columns string, table string, user string, verb string) string {
	var result strings.Builder

	// Get the table name. If it doesn't already have a schema part, then assign
	// the username as the schema.
	table, _ = fullName(user, table)

	result.WriteString(verb + " ")

	if verb == selectVerb {
		result.WriteString(columnList(columns))
	}

	result.WriteString(" FROM " + table)

	if where := whereClause(filter); where != "" {
		result.WriteString(where)
	}

	if sort := sortList(u); sort != "" && verb == selectVerb {
		result.WriteString(sort)
	}

	if paging := pagingClauses(u); paging != "" && verb == selectVerb {
		result.WriteString(paging)
	}

	return result.String()
}

func formUpdateQuery(u *url.URL, user string, items map[string]interface{}) (string, []interface{}) {
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

	// Get the table name and filter list
	table, _ := fullName(user, datatypes.GetString(tableItem))

	var result strings.Builder

	result.WriteString(updateVerb)
	result.WriteRune(' ')

	result.WriteString(table)

	keys := util.InterfaceMapKeys(items)
	keyCount := len(keys)

	if _, found := items[defs.RowIDName]; found {
		keyCount--
	}

	values := make([]interface{}, keyCount)

	// Loop over the item names and add SET clauses for each one. We always
	// ignore the rowid value because you cannot update it on an UPDATE call;
	// it is only set on an insert.
	filterCount := 0

	for _, key := range keys {
		if key == defs.RowIDName {
			continue
		}

		values[filterCount] = items[key]

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
	if id, found := items[defs.RowIDName]; found {
		idString := datatypes.GetString(id)
		if idString != "" {
			if where == "" {
				where = "WHERE " + defs.RowIDName + " = '" + idString + "'"
			} else {
				where = where + " " + defs.RowIDName + " = '" + idString + "'"
			}
		}
	}

	if where == "" && settings.GetBool(defs.TablesServerEmptyFilterError) {
		return syntaxErrorPrefix + "operation invalid with empty filter", nil
	}

	// If we have a filter string now, add it to the query.
	if where != "" {
		result.WriteString(" " + where)
	}

	return result.String(), values
}

func formInsertQuery(table string, user string, items map[string]interface{}) (string, []interface{}) {
	var result strings.Builder

	fullyQualifiedName, _ := fullName(user, table)

	result.WriteString(insertVerb)
	result.WriteString(" INTO ")

	result.WriteString(fullyQualifiedName)

	keys := util.InterfaceMapKeys(items)
	values := make([]interface{}, len(items))

	for i, key := range keys {
		if i == 0 {
			result.WriteRune('(')
		} else {
			result.WriteRune(',')
		}

		result.WriteString("\"" + key + "\"")
	}

	result.WriteString(") VALUES (")

	for i, key := range keys {
		values[i] = items[key]

		if i > 0 {
			result.WriteString(",")
		}

		result.WriteString(fmt.Sprintf("$%d", i+1))
	}

	result.WriteRune(')')

	return result.String(), values
}

func formCreateQuery(u *url.URL, user string, hasAdminPrivileges bool, items []defs.DBColumn, sessionID int32, w http.ResponseWriter) string {
	if u == nil {
		return ""
	}

	parts, ok := functions.ParseURLPattern(u.Path, "/tables/{{name}}")
	if !ok {
		return ""
	}

	tableItem, ok := parts["name"]
	if !ok {
		return ""
	}

	// Get the table name. If it doesn't already have a schema part, then assign
	// the username as the schema.
	table, wasFullyQualified := fullName(user, datatypes.GetString(tableItem))
	// This is a multipart name. You must be an administrator to do this
	if !wasFullyQualified && !hasAdminPrivileges {
		util.ErrorResponse(w, sessionID, "No privilege to create table in another user's domain", http.StatusForbidden)
	}

	var result strings.Builder

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

	if !hasRowID {
		items = append(items, defs.DBColumn{
			Name: defs.RowIDName,
			Type: datatypes.StringTypeName,
		})
	}

	for i, column := range items {
		if i == 0 {
			result.WriteRune('(')
		} else {
			result.WriteString(", ")
		}

		result.WriteString("\"" + column.Name + "\"")
		result.WriteRune(' ')

		nativeType := mapColumnType(column.Type)
		result.WriteString(nativeType)

		if column.Unique {
			result.WriteString(" UNIQUE")
		}

		if !column.Nullable {
			result.WriteString(" NOT NULL ")
		}
	}

	result.WriteRune(')')

	return result.String()
}

// mapColumnType converts native Ego types into the equivalent Postgres data types.
func mapColumnType(native string) string {
	types := map[string]string{
		datatypes.StringTypeName: "CHAR VARYING",
		"int32":                  "INT32",
		datatypes.IntTypeName:    "INT",
		datatypes.BoolTypeName:   "BOOLEAN",
		"boolean":                "BOOLEAN",
		"float32":                "REAL",
		"float64":                "DOUBLE PRECISION",
		"timestamp":              "TIMESTAMP WITH TIME ZONE",
		"time":                   "TIME",
		"date":                   "DATE",
	}

	native = strings.ToLower(native)
	if newType, ok := types[native]; ok {
		return newType
	}

	return native
}

func tableNameParts(user string, name string) []string {
	fullyQualified, _ := fullName(user, name)

	parts := strings.Split(fullyQualified, ".")
	for i, part := range parts {
		parts[i] = stripQuotes(part)
	}

	return parts
}

func keywordMatch(k string, list ...string) bool {
	for _, item := range list {
		if strings.EqualFold(k, item) {
			return true
		}
	}

	return false
}

func tableNameFromRequest(r *http.Request) (string, *errors.EgoError) {
	u, _ := url.Parse(r.URL.Path)

	return tableNameFromURL(u)
}

func tableNameFromURL(u *url.URL) (string, *errors.EgoError) {
	parts, ok := functions.ParseURLPattern(u.Path, "/tables/{{name}}/rows")
	if !ok {
		return "", errors.NewMessage("Invalid URL").Context(u.Path)
	}

	tableItem, ok := parts["name"]
	if !ok {
		return "", errors.NewMessage("Missing table name in URL").Context(u.Path)
	}

	return datatypes.GetString(tableItem), nil
}
