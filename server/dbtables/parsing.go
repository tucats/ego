package dbtables

import (
	"net/url"
	"strings"

	"github.com/tucats/ego/datatypes"
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
		if strings.EqualFold(k, "column") || strings.EqualFold(k, "columns") {
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

func filterList(u *url.URL) string {
	var result strings.Builder

	if u == nil {
		return ""
	}

	values := u.Query()
	for k, v := range values {
		if strings.EqualFold(k, "filter") || strings.EqualFold(k, "filters") {
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
	return "WHERE " + result.String()
}

func sortList(u *url.URL) string {
	var result strings.Builder

	if u == nil {
		return ""
	}

	values := u.Query()
	ascending := true

	for k, v := range values {
		if strings.EqualFold(k, "sort") || strings.EqualFold(k, "order") {
			for i, name := range v {
				if strings.HasPrefix(name, "~") {
					name = strings.TrimPrefix(name, "~")
					ascending = false
				}

				if i == 0 {
					result.WriteString("ORDER BY ")
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

func formQuery(u *url.URL, user string, verb string) string {
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

	table := datatypes.GetString(tableItem)
	if user != "" {
		table = user + "." + table
	}
	columns := columnList(u)
	where := filterList(u)
	sort := sortList(u)

	var result strings.Builder

	result.WriteString(verb + " ")
	if verb == selectVerb {
		result.WriteString(columns)
	}

	result.WriteString(" FROM " + table)

	if where != "" {
		result.WriteString(" " + where)
	}

	if sort != "" && verb == selectVerb {
		result.WriteString(" " + sort)
	}

	return result.String()
}
