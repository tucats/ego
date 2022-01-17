package dbtables

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/tokenizer"
)

// @tomcole probably need to dump this entirely and work on variadic substitution arguments in query!
func sqlEscape(source string) string {
	var result strings.Builder

	for _, ch := range source {
		if ch == '\'' || ch == '"' || ch == ';' {
			return "INVALID-NAME"
		}

		result.WriteRune(ch)
	}

	return result.String()
}

func queryParameters(source string, args map[string]string) string {
	// Before anything else, let's see if the table name was specified,
	// and it contains a "dot" notation. If so, replace the schema name
	// with the dot name prefix.
	if tableName, ok := args[defs.TableParameterName]; ok {
		dot := strings.Index(tableName, ".")
		if dot >= 0 {
			args[defs.TableParameterName] = tableName[dot+1:]
			args[defs.SchemaParameterName] = tableName[:dot]
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

func formWhereClause(filters []string) string {
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
			clause, err := whereClause(tokens)
			if err != nil {
				return "SYNTAX-ERROR:" + err.Error()
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

func whereClause(tokens *tokenizer.Tokenizer) (string, error) {
	var result strings.Builder

	operator := tokens.Next()

	if !tokens.IsNext("(") {
		// Assume it's a constant value of some kind. Convert Ego strings to SQL strings
		isString := false

		if strings.HasPrefix(operator, "\"") && strings.HasSuffix(operator, "\"") {
			operator = strings.TrimPrefix(strings.TrimSuffix(operator, "\""), "\"")
			isString = true
		} else {
			if strings.HasPrefix(operator, "'") && strings.HasSuffix(operator, "'") {
				operator = strings.TrimPrefix(strings.TrimSuffix(operator, "'"), "'")
				isString = true
			}
		}

		operator = sqlEscape(operator)

		if isString {
			operator = "'" + operator + "'"
		}

		return operator, nil
	}

	prefix := ""
	infix := ""

	switch strings.ToUpper(operator) {
	case "EQ":
		infix = "="

	case "LT":
		infix = "<"

	case "LE":
		infix = "<="

	case "GT":
		infix = ">"

	case "GE":
		infix = ">="

	case "AND":
		infix = "AND"

	case "OR":
		infix = "OR"

	case "NOT":
		prefix = "NOT"
	}

	if prefix != "" {
		term, _ := whereClause(tokens)
		result.WriteString(prefix + " " + term)
	} else {
		term, _ := whereClause(tokens)
		result.WriteString(term + " ")
		result.WriteString(infix + " ")
		if !tokens.IsNext(",") {
			return "", errors.New(errors.ErrInvalidList)
		}
		term, _ = whereClause(tokens)
		result.WriteString(term)
	}

	if !tokens.IsNext(")") {
		return "", errors.New(errors.ErrMissingParenthesis)
	}

	return result.String(), nil
}

func columnList(u *url.URL) string {
	var result strings.Builder

	if u == nil {
		return "*"
	}

	values := u.Query()
	for k, v := range values {
		if keywordMatch(k, "column", defs.ColumnParameterName) {
			for _, name := range v {
				if result.Len() > 0 {
					result.WriteRune(',')
				}

				result.WriteString(name)
			}
		}
	}

	if result.Len() == 0 {
		return "*"
	}

	return result.String()
}

func fullName(user, table string) (string, bool) {
	wasFullyQualified := true

	if dot := strings.Index(table, "."); dot < 0 {
		table = user + "." + table
		wasFullyQualified = false
	}

	return table, wasFullyQualified
}

func filterList(u *url.URL) string {
	var result strings.Builder

	if u == nil {
		return ""
	}

	values := u.Query()
	for k, v := range values {
		if strings.EqualFold(k, defs.FilterParameterName) {
			clause := formWhereClause(v)

			if result.Len() > 0 {
				result.WriteString(" AND ")
			}

			result.WriteString(clause)
		}
	}

	if result.Len() == 0 {
		return ""
	}

	return " WHERE " + result.String()
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

				result.WriteString(name)
			}
		}
	}

	if result.Len() > 0 && !ascending {
		result.WriteString(" DESC")
	}

	return result.String()
}

func formSelectorDeleteQuery(u *url.URL, user string, verb string) string {
	if u == nil {
		return ""
	}

	parts, ok := functions.ParseURLPattern(u.Path, "/tables/{{name}}/rows")
	if !ok {
		return ""
	}

	tableItem, ok := parts["name"]
	if !ok {
		return ""
	}

	// Get the table name. If it doesn't already have a schema part, then assign
	// the username as the schema.
	table, _ := fullName(user, datatypes.GetString(tableItem))

	columns := columnList(u)
	where := filterList(u)
	sort := sortList(u)
	paging := pagingClauses(u)

	var result strings.Builder

	result.WriteString(verb + " ")

	if verb == selectVerb {
		result.WriteString(columns)
	}

	result.WriteString(" FROM " + table)

	if where != "" {
		result.WriteString(where)
	}

	if sort != "" && verb == selectVerb {
		result.WriteString(sort)
	}

	if paging != "" && verb == selectVerb {
		result.WriteString(paging)
	}

	return result.String()
}

func formUpdateQuery(u *url.URL, user string, data map[string]interface{}) (string, []interface{}) {
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
	where := filterList(u)

	var result strings.Builder

	result.WriteString(updateVerb)
	result.WriteRune(' ')

	result.WriteString(table)

	keys := make([]string, 0)
	values := make([]interface{}, len(data))

	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for i, key := range keys {
		values[i] = data[key]

		if i == 0 {
			result.WriteString(" SET ")
		} else {
			result.WriteString(", ")
		}

		result.WriteString(key)
		result.WriteString(fmt.Sprintf(" = $%d", i+1))
	}

	if where != "" {
		result.WriteString(" " + where)
	}

	return result.String(), values
}

func formInsertQuery(u *url.URL, user string, data map[string]interface{}) (string, []interface{}) {
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
	table, _ := fullName(user, datatypes.GetString(tableItem))

	var result strings.Builder

	result.WriteString(insertVerb)
	result.WriteString(" INTO ")

	result.WriteString(table)

	keys := make([]string, 0)
	values := make([]interface{}, len(data))

	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for i, key := range keys {
		if i == 0 {
			result.WriteRune('(')
		} else {
			result.WriteRune(',')
		}

		result.WriteString(key)
	}

	result.WriteString(") VALUES (")

	for i, key := range keys {
		values[i] = data[key]

		if i > 0 {
			result.WriteString(",")
		}

		result.WriteString(fmt.Sprintf("$%d", i+1))
	}

	result.WriteRune(')')

	return result.String(), values
}

func formCreateQuery(u *url.URL, user string, hasAdminPrivileges bool, data []defs.DBColumn, sessionID int32, w http.ResponseWriter) string {
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
		ErrorResponse(w, sessionID, "No privilege to create table in another user's domain", http.StatusForbidden)
	}

	var result strings.Builder

	result.WriteString("CREATE TABLE ")
	result.WriteString(table)

	// See if the column data already contains a row ID value; if not,
	// add it in to the table definition.
	hasRowID := false

	for _, column := range data {
		if column.Name == defs.RowIDName {
			hasRowID = true

			break
		}
	}

	if !hasRowID {
		data = append(data, defs.DBColumn{
			Name: defs.RowIDName,
			Type: "string",
		})
	}

	for i, column := range data {
		if i == 0 {
			result.WriteRune('(')
		} else {
			result.WriteString(", ")
		}

		result.WriteString(column.Name)
		result.WriteRune(' ')

		nativeType := mapColumnType(column.Type)
		result.WriteString(nativeType)

		if column.Nullable {
			result.WriteString(" NULL")
		}
	}

	result.WriteRune(')')

	return result.String()
}

// mapColumnType converts native Ego types into the equivalent Postgres data types.
func mapColumnType(native string) string {
	types := map[string]string{
		"string":    "CHAR VARYING",
		"int32":     "INT32",
		"int":       "INT",
		"bool":      "BOOLEAN",
		"boolean":   "BOOLEAN",
		"float32":   "REAL",
		"float64":   "DOUBLE PRECISION",
		"timestamp": "TIMESTAMP WITH TIME ZONE",
		"time":      "TIME",
		"date":      "DATE",
	}

	native = strings.ToLower(native)
	if newType, ok := types[native]; ok {
		return newType
	}

	return native
}

func tableNameParts(user string, name string) []string {
	fullyQualified, _ := fullName(user, name)

	return strings.Split(fullyQualified, ".")
}

func keywordMatch(k string, list ...string) bool {
	for _, item := range list {
		if strings.EqualFold(k, item) {
			return true
		}
	}

	return false
}
