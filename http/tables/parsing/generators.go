package parsing

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	runtime_strings "github.com/tucats/ego/runtime/strings"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

func FormSelectorDeleteQuery(u *url.URL, filter []string, columns string, table string, user string, verb string) string {
	var result strings.Builder

	// Get the table name. If it doesn't already have a schema part, then assign
	// the username as the schema.
	table, _ = FullName(user, table)

	result.WriteString(verb + " ")

	if verb == selectVerb {
		result.WriteString(ColumnList(columns))
	}

	result.WriteString(" FROM " + table)

	if where := WhereClause(filter); where != "" {
		result.WriteString(where)
	}

	if sort := SortList(u); sort != "" && verb == selectVerb {
		result.WriteString(sort)
	}

	if paging := PagingClauses(u); paging != "" && verb == selectVerb {
		result.WriteString(paging)
	}

	return result.String()
}

func FormUpdateQuery(u *url.URL, user string, items map[string]interface{}) (string, []interface{}) {
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

	// Get the table name and filter list
	table, _ := FullName(user, data.String(tableItem))

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

	where := WhereClause(FiltersFromURL(u))

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
		return SyntaxErrorPrefix + "operation invalid with empty filter", nil
	}

	// If we have a filter string now, add it to the query.
	if where != "" {
		result.WriteString(" " + where)
	}

	return result.String(), values
}

func FormInsertQuery(table string, user string, items map[string]interface{}) (string, []interface{}) {
	var result strings.Builder

	fullyQualifiedName, _ := FullName(user, table)

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

func FormCreateQuery(u *url.URL, user string, hasAdminPrivileges bool, items []defs.DBColumn, sessionID int, w http.ResponseWriter) string {
	if u == nil {
		return ""
	}

	parts, ok := runtime_strings.ParseURLPattern(u.Path, "/tables/{{name}}")
	if !ok {
		return ""
	}

	tableItem, ok := parts["name"]
	if !ok {
		return ""
	}

	// Get the table name. If it doesn't already have a schema part, then assign
	// the username as the schema.
	table, wasFullyQualified := FullName(user, data.String(tableItem))
	// This is a multipart name. You must be an administrator to do this
	if !wasFullyQualified && !hasAdminPrivileges {
		util.ErrorResponse(w, sessionID, "No privilege to create table in another user's domain", http.StatusForbidden)

		return ""
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
			Type: data.StringTypeName,
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

		nativeType := MapColumnType(column.Type)
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

func formWhereExpressions(filters []string) string {
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
				return SyntaxErrorPrefix + err.Error()
			}

			result.WriteString(clause)

			if !tokens.IsNext(tokenizer.CommaToken) {
				break
			}

			result.WriteString(" AND ")
		}
	}

	return result.String()
}

func FormCondition(condition string) string {
	var result strings.Builder

	tokens := tokenizer.New(condition, true)
	if tokens.AtEnd() {
		return ""
	}

	for {
		clause, err := filterClause(tokens, egoDialect)
		if err != nil {
			return SyntaxErrorPrefix + err.Error()
		}

		result.WriteString(clause)

		if !tokens.IsNext(tokenizer.CommaToken) {
			break
		}

		result.WriteString(" && ")
	}

	return result.String()
}

func QueryParameters(source string, args map[string]string) string {
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

	// Skip through the substition strings provided and do any replace
	// needed.
	result := source

	for k, v := range args {
		v = SQLEscape(v)
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}

	return result
}

func filterClause(tokens *tokenizer.Tokenizer, dialect int) (string, error) {
	var result strings.Builder

	operator := tokens.Next()
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

		operatorSpelling := SQLEscape(operator.Spelling())

		if isString {
			switch dialect {
			case sqlDialect:
				operatorSpelling = "'" + operatorSpelling + "'"
			case egoDialect:
				operatorSpelling = "\"" + operatorSpelling + "\""
			}
		}

		if isName && dialect == sqlDialect {
			operatorSpelling = "\"" + operatorSpelling + "\""
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
			return "", errors.NewError(e)
		}

		valueCount := 0

		for tokens.IsNext(tokenizer.CommaToken) {
			if valueCount > 0 {
				result.WriteString(conjunction)
			}
			valueCount++

			value, e := filterClause(tokens, dialect)
			if e != nil {
				return "", errors.NewError(e)
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
			return "", errors.ErrMissingParenthesis
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
		result.WriteString(prefix + " " + term)
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

			result.WriteString(" " + infix + " ")

			term, _ = filterClause(tokens, dialect)
		}

		result.WriteString(")")
	}

	if !tokens.IsNext(tokenizer.EndOfListToken) {
		return "", errors.ErrMissingParenthesis
	}

	return result.String(), nil
}

// whereClause accepts a list of filter parameters, and converts them
// to a SQL WHERE clause (including the 'WHERE' token).
func WhereClause(filters []string) string {
	if len(filters) == 0 {
		return ""
	}

	clause := formWhereExpressions(filters)

	return " WHERE " + clause
}

func PagingClauses(u *url.URL) string {
	var result strings.Builder

	if u == nil {
		return ""
	}

	values := u.Query()
	for k, v := range values {
		if KeywordMatch(k, "limit", "count") {
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

		if KeywordMatch(k, "start", "offset") {
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
