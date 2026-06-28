package parsing

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/util/strings"
	"github.com/tucats/ego/internal/errors"
	runtime_strings "github.com/tucats/ego/internal/runtime/strings"
)

const (
	sqlDialect = 0
	egoDialect = 1
)

func SQLEscape(source string) (string, error) {
	var (
		err    error
		result strings.Builder
	)

	if strings.HasPrefix(source, "'") {
		source = strings.TrimPrefix(strings.TrimSuffix(source, "'"), "'")
	} else if strings.HasPrefix(source, "\"") {
		source = strings.TrimPrefix(strings.TrimSuffix(source, "\""), "\"")
	}

	for idx, ch := range source {
		if idx > 0 && idx < len(source)-1 && ch == '\'' {
			return "INVALID-NAME", errors.ErrInvalidSQLName
		}

		if ch == ';' {
			return "INVALID-NAME", errors.ErrInvalidSQLName
		}

		result.WriteRune(ch)
	}

	return result.String(), err
}

// StripQuotes removes double quotes from the input string. Leading and trailing double-quotes
// are removed, as are internal "." quoted boundaries. This prevents a name from being put through
// the FullName() processor multiple times and accumulating extra quotes.
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
		if !strings.HasPrefix(strings.ToLower(name), "count(") {
			name = strings.TrimSpace(name)

			if len(name) == 0 {
				continue
			}

			name = egostrings.SQLIdentifier(name)
		}

		if result.Len() > 0 {
			result.WriteRune(',')
		}

		result.WriteString(name)
	}

	if result.Len() == 0 {
		return "*"
	}

	return result.String()
}

// FullName returns the fully-qualified SQL table name for the given provider,
// user (schema), and unqualified table name.
//
// SQLite has no schema concept: all tables share a single flat namespace, so
// only the quoted table name is returned.  PostgreSQL uses the user name as the
// schema prefix (e.g. "alice"."orders").
//
// If the table name already contains a "." separator it is treated as already
// schema-qualified and is returned as-is (with each part individually quoted).
//
// The function cannot return an error because it is called in many contexts where
// only a string is expected.  If an unrecognized provider is supplied, the function
// falls back to the PostgreSQL schema-qualified form (the safer choice, as a wrong
// schema name produces a clear database error rather than a silent wrong result).
// Higher-level handlers that have an error return path should validate the provider
// before calling this function.
//
// To add support for a new provider: add a case in the switch below and produce the
// appropriate fully-qualified name.
func FullName(provider, user, table string) (string, bool) {
	wasFullyQualified := true
	user = StripQuotes(user)
	table = StripQuotes(table)

	if dot := strings.Index(table, "."); dot < 0 {
		// The table name has no dot separator, so we may need to add a schema prefix.
		switch {
		case strings.EqualFold(provider, defs.SqliteProvider) ||
			strings.EqualFold(provider, defs.DeprecatedSqliteProvider):
			// SQLite: no schema prefix; just quote the table name.
			table = egostrings.SQLIdentifier(table)

		case strings.EqualFold(provider, defs.PostgresProvider):
			// PostgreSQL: prefix the table name with the user/schema name.
			table = egostrings.SQLIdentifier(user) + "." + egostrings.SQLIdentifier(table)

		default:
			// Unknown provider — fall back to PostgreSQL-style schema qualification.
			// The caller is responsible for rejecting unsupported providers before
			// reaching here when a strict error is required.
			table = egostrings.SQLIdentifier(user) + "." + egostrings.SQLIdentifier(table)
		}

		wasFullyQualified = false
	} else {
		parts := strings.Split(table, ".")
		table = ""

		for n, part := range parts {
			if n > 0 {
				table = table + "."
			}

			table = table + egostrings.SQLIdentifier(part)
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
					result.WriteString("ORDER BY ")
				} else {
					result.WriteString(",")
				}

				parts := strings.Split(name, ",")
				name := ""

				for n, part := range parts {
					if n > 0 {
						name = name + ","
					}

					name = name + part
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

// MapColumnType converts a portable Ego column type name into the SQL DDL type string
// appropriate for the target database provider.
//
// Each provider uses different type names: SQLite uses "TEXT", "INTEGER", and "REAL"
// affinity names, while PostgreSQL has richer type vocabulary ("CHAR VARYING",
// "DOUBLE PRECISION", "TIMESTAMP WITH TIME ZONE", etc.).
//
// For date/time types on SQLite: SQLite does not have a native date/time type, but we
// declare columns with their semantic names (TIMESTAMP, TIME, DATE) rather than TEXT.
// SQLite applies TEXT affinity to these names at the storage level, but preserving the
// declared name allows schema introspection to recover the semantic type.  Actual values
// are stored as UTC RFC 3339 strings.
//
// The function cannot return an error because it is invoked deep in DDL-generation code
// where only a string is expected.  If an unrecognized provider is supplied the native
// type name is returned unchanged, which preserves caller intent without panicking.
// Higher-level handlers that have an error return path should validate the provider before
// calling this function.
//
// To add a new provider: add a case in the switch below with a type-name map for that
// provider's DDL vocabulary.
func MapColumnType(native, provider string) string {
	var types map[string]string

	switch {
	case strings.EqualFold(provider, defs.SqliteProvider) ||
		strings.EqualFold(provider, defs.DeprecatedSqliteProvider):
		// SQLite type affinity rules: TEXT, INTEGER, REAL, BLOB, NUMERIC.
		// TIMESTAMP, TIME, and DATE are stored as TEXT affinity; the semantic names are
		// preserved so schema introspection can recover them via getColumnInfo().
		types = map[string]string{
			data.StringTypeName: "TEXT",
			data.Int32TypeName:  "INTEGER",
			data.IntTypeName:    "INTEGER",
			data.BoolTypeName:   "INTEGER", // SQLite has no native BOOLEAN; store as 0/1
			"boolean":           "INTEGER",
			"float32":           "REAL",
			"float64":           "REAL",
			"timestamp":         "TIMESTAMP", // RFC 3339 text; semantic name retained for introspection
			"time":              "TIME",       // RFC 3339 text; semantic name retained for introspection
			"date":              "DATE",       // RFC 3339 text; semantic name retained for introspection
		}

	case strings.EqualFold(provider, defs.PostgresProvider):
		// PostgreSQL DDL type names.
		types = map[string]string{
			data.StringTypeName: "CHAR VARYING",
			data.Int32TypeName:  "INTEGER",
			data.IntTypeName:    "INT",
			data.BoolTypeName:   "BOOLEAN",
			"boolean":           "BOOLEAN",
			"float32":           "REAL",
			"float64":           "DOUBLE PRECISION",
			"timestamp":         "TIMESTAMP WITH TIME ZONE",
			"time":              "TIME",
			"date":              "DATE",
		}

	default:
		// Unknown provider — return the native type name unchanged rather than
		// silently producing a wrong DDL type.  Callers that can propagate errors
		// should have validated the provider before reaching here.
		return strings.ToLower(native)
	}

	native = strings.ToLower(native)
	if newType, ok := types[native]; ok {
		return newType
	}

	return native
}

func TableNameParts(provider, user, name string) []string {
	fullyQualified, _ := FullName(provider, user, name)

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
			return "", errors.ErrInvalidURL.Context(u.Path)
		}
	}

	tableItem, ok := parts["name"]
	if !ok {
		return "", errors.ErrInvalidURL.Context(u.Path)
	}

	return data.String(tableItem), nil
}
