package parsing

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	runtime_strings "github.com/tucats/ego/runtime/strings"
)

const (
	sqlDialect = 0
	egoDialect = 1
)

func SQLEscape(source string) string {
	var result strings.Builder

	if strings.HasPrefix(source, "'") {
		source = strings.TrimPrefix(strings.TrimSuffix(source, "'"), "'")
	} else if strings.HasPrefix(source, "\"") {
		source = strings.TrimPrefix(strings.TrimSuffix(source, "\""), "\"")
	}

	for idx, ch := range source {
		if idx > 0 && idx < len(source)-1 && ch == '\'' {
			return "INVALID-NAME"
		}

		if ch == ';' {
			return "INVALID-NAME"
		}

		result.WriteRune(ch)
	}

	return result.String()
}

// StripQuotes removes double quotes from the input string. Leading and trailing double-quotes
// are removed, as are internal "." quoted boudnaries. This prevents a name from being put through
// the FullName() processor multiple times and accumulate extra quots.
func StripQuotes(input string) string {
	return strings.TrimPrefix(
		strings.TrimSuffix(
			strings.ReplaceAll(input, "\".\"", "."),
			"\""),
		"\"")
}

func ColumnsFromURL(u *url.URL) string {
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

func ColumnList(columnsParameter string) string {
	var result strings.Builder

	if columnsParameter == "" {
		return "*"
	}

	names := strings.Split(columnsParameter, ",")

	for _, name := range names {
		name = strings.TrimSpace(name)

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

func FullName(user, table string) (string, bool) {
	wasFullyQualified := true
	user = StripQuotes(user)
	table = StripQuotes(table)

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
func FiltersFromURL(u *url.URL) []string {
	result := make([]string, 0)

	q := u.Query()
	for param, values := range q {
		if strings.EqualFold(param, defs.FilterParameterName) {
			result = append(result, values...)
		}
	}

	return result
}

// With a default user name string, and the current URL, determine if the
// username should be overridden with one from the URL.
func RequestForUser(user string, u *url.URL) string {
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

func SortList(u *url.URL) string {
	var result strings.Builder

	if u == nil {
		return ""
	}

	values := u.Query()
	ascending := true

	for k, v := range values {
		if KeywordMatch(k, "sort", "order", "sort-by", "order-by") {
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

// mapColumnType converts native Ego types into the equivalent Postgres data types.
func MapColumnType(native string) string {
	types := map[string]string{
		data.StringTypeName: "CHAR VARYING",
		data.Int32TypeName:  "INT32",
		data.IntTypeName:    "INT",
		data.BoolTypeName:   "BOOLEAN",
		"boolean":           "BOOLEAN",
		"float32":           "REAL",
		"float64":           "DOUBLE PRECISION",
		"timestamp":         "TIMESTAMP WITH TIME ZONE",
		"time":              "TIME",
		"date":              "DATE",
	}

	native = strings.ToLower(native)
	if newType, ok := types[native]; ok {
		return newType
	}

	return native
}

func TableNameParts(user string, name string) []string {
	fullyQualified, _ := FullName(user, name)

	parts := strings.Split(fullyQualified, ".")
	for i, part := range parts {
		parts[i] = StripQuotes(part)
	}

	return parts
}

func KeywordMatch(k string, list ...string) bool {
	for _, item := range list {
		if strings.EqualFold(k, item) {
			return true
		}
	}

	return false
}

func TableNameFromRequest(r *http.Request) (string, error) {
	u, _ := url.Parse(r.URL.Path)

	return TableNameFromURL(u)
}

func TableNameFromURL(u *url.URL) (string, error) {
	parts, ok := runtime_strings.ParseURLPattern(u.Path, "/tables/{{name}}/rows")
	if !ok {
		parts, ok = runtime_strings.ParseURLPattern(u.Path, "/dsns/{{dsn}}/tables/{{name}}/rows")
		if !ok {
			return "", errors.Message("Invalid URL").Context(u.Path)
		}
	}

	tableItem, ok := parts["name"]
	if !ok {
		return "", errors.Message("Missing table name in URL").Context(u.Path)
	}

	return data.String(tableItem), nil
}
