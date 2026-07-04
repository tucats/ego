package parsing

import (
	"net/url"
	"testing"

	"github.com/tucats/ego/internal/defs"
)

// TestSQLEscape_RejectsEmbeddedDoubleQuote is a regression/hardening test:
// SQLEscape used to reject an embedded "'" and ";" but not an embedded '"',
// even though several callers (e.g. QueryParameters templates like
// `DROP TABLE "{{table}}";`) embed the escaped value inside double quotes.
// A value containing a '"' must now be rejected the same way a value
// containing a "'" already is, so it can never break out of that
// double-quote-delimited identifier.
func TestSQLEscape_RejectsEmbeddedDoubleQuote(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "ordinary identifier", input: "mytable", wantErr: false},
		{name: "embedded single quote is rejected", input: "my'table", wantErr: true},
		{name: "embedded semicolon is rejected", input: "my;table", wantErr: true},
		{name: "embedded double quote is rejected", input: `my"table`, wantErr: true},
		{name: "embedded double quote plus comment sequence is rejected", input: `evil" ; DROP TABLE users; --`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SQLEscape(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SQLEscape(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func Test_formWhereClause(t *testing.T) {
	tests := []struct {
		name    string
		filters []string
		want    string
		wantErr string
	}{
		{
			name:    "signed constant",
			filters: []string{"eq(age,-1)"},
			want:    "(\"age\" = -1)",
		},
		{
			name:    "bogus expression",
			filters: []string{"faux(name,\"string\")"},
			wantErr: `unexpected token: Identifier "faux"`,
		},
		{
			name:    "nested expression",
			filters: []string{"or(eq(name,\"Tom\"),eq(name,\"Mary\"))"},
			want:    "((\"name\" = 'Tom')  OR  (\"name\" = 'Mary'))",
		},
		{
			name:    "string constant",
			filters: []string{"eq(name,\"Tom\")"},
			want:    "(\"name\" = 'Tom')",
		},
		{
			name:    "simple equality",
			filters: []string{"eq(age,55)"},
			want:    "(\"age\" = 55)",
		},
		{
			name:    "unary not",
			filters: []string{"not(age)"},
			want:    " NOT  \"age\"",
		},
		{
			name:    "simple list",
			filters: []string{"lt(age,18)", "gt(age,65)"},
			want:    "(\"age\" < 18) AND (\"age\" > 65)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := formWhereExpressions(tt.filters)

			errMessage := ""
			if err != nil {
				errMessage = err.Error()
			}

			if errMessage != tt.wantErr {
				t.Errorf("formWhereClause() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("formWhereClause() got %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_columnList(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "no columns",
			arg:  "https://localhost:8500/tables/data",
			want: "*",
		},
		{
			name: "one column",
			arg:  "https://localhost:8500/tables/data?columns=age",
			want: "\"age\"",
		},
		{
			name: "multiple columns",
			arg:  "https://localhost:8500/tables/data?columns=name&columns=age",
			want: "\"name\",\"age\"",
		},
		{
			name: "column list",
			arg:  "https://localhost:8500/tables/data?columns=name,age",
			want: "\"name\",\"age\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.arg)
			list := ColumnsFromURL(u)

			if got := ColumnList(list); got != tt.want {
				t.Errorf("columnList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_filterList(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "compound list",
			arg:  "https://localhost:8500/tables/data?filter=and(EQ(a,1),EQ(b,2),EQ(c,3))",
			want: `WHERE (("a" = 1)  AND  ("b" = 2)  AND  ("c" = 3))`,
		},
		{
			name: "compound contains list",
			arg:  "https://localhost:8500/tables/data?filter=contains(foo, 'abc', 'def')",
			want: `WHERE POSITION('abc' IN "foo") > 0 OR POSITION('def' IN "foo") > 0`,
		},
		{
			name: "compound hasall list",
			arg:  "https://localhost:8500/tables/data?filter=hasall(foo, 'abc', 'def')",
			want: `WHERE POSITION('abc' IN "foo") > 0 AND POSITION('def' IN "foo") > 0`,
		},
		{
			name: "compound list",
			arg:  "https://localhost:8500/tables/data?filter=and(EQ(a,1),EQ(b,2),EQ(c,3))",
			want: `WHERE (("a" = 1)  AND  ("b" = 2)  AND  ("c" = 3))`,
		},
		{
			name: "filter list",
			arg:  "https://localhost:8500/tables/data?filter=eq(name,\"Tom\"),eq(age,55)",
			want: "WHERE (\"name\" = 'Tom') AND (\"age\" = 55)",
		},
		{
			name: "no filter",
			arg:  "https://localhost:8500/tables/data",
			want: "",
		},
		{
			name: "one filter",
			arg:  "https://localhost:8500/tables/data?filter=eq(age,55)",
			want: "WHERE (\"age\" = 55)",
		},
		{
			name: "multiple filters",
			arg:  "https://localhost:8500/tables/data?filter=eq(name,\"Tom\")&filter=eq(name,\"Mary\")",
			want: "WHERE (\"name\" = 'Tom') AND (\"name\" = 'Mary')",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.arg)
			f := FiltersFromURL(u)

			if got, _ := WhereClause(f); got != tt.want {
				t.Errorf("filterList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sortList(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "sort list",
			arg:  "https://localhost:8500/tables/data?sort=tom,age",
			want: "ORDER BY tom,age",
		},
		{
			name: "no sort",
			arg:  "https://localhost:8500/tables/data",
			want: "",
		},
		{
			name: "one sort",
			arg:  "https://localhost:8500/tables/data?order=age",
			want: "ORDER BY age",
		},
		{
			name: "one sort list",
			arg:  "https://localhost:8500/tables/data?order=age,name",
			want: "ORDER BY age,name",
		},
		{
			name: "multiple sorts",
			arg:  "https://localhost:8500/tables/data?sort=name&sort=age",
			want: "ORDER BY name,age",
		},
		{
			name: "descending sort",
			arg:  "https://localhost:8500/tables/data?sort=~age",
			want: "ORDER BY age DESC",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.arg)
			if got := SortList(u); got != tt.want {
				t.Errorf("sortList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_formQuery(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    string
		wantErr string
	}{
		{
			name:    "Bogus filter",
			arg:     "https://localhost:8500/tables/data?filter=faux(name,\"string\")",
			want:    "",
			wantErr: `unexpected token: Identifier "faux"`,
		},
		{
			name:    "bogus term list",
			arg:     "https://localhost:8500/tables/data?filter=EQ(name,)",
			want:    "",
			wantErr: "missing parenthesis",
		},
		{
			name:    "bogus term value",
			arg:     "https://localhost:8500/tables/data?filter=EQ(--3,)",
			want:    "",
			wantErr: "invalid list",
		},
		{
			name:    "missing term value",
			arg:     "https://localhost:8500/tables/data?filter=EQ(a)",
			want:    "",
			wantErr: "invalid list",
		},
		{
			name: "no query parameters",
			arg:  "https://localhost:8500/tables/data",
			want: "SELECT * FROM \"admin\".\"data\"  LIMIT 1000",
		},
		{
			name: "column specification",
			arg:  "https://localhost:8500/tables/data?columns=name,age",
			want: "SELECT \"name\",\"age\" FROM \"admin\".\"data\"  LIMIT 1000",
		},
		{
			name: "column and sort specification",
			arg:  "https://localhost:8500/tables/data?order=age&columns=name,age",
			want: "SELECT \"name\",\"age\" FROM \"admin\".\"data\" ORDER BY age  LIMIT 1000",
		},
		{
			name: "column, filter, and sort specification",
			arg:  "https://localhost:8500/tables/data?order=age&columns=name,age&filter=GE(age,18)",
			want: "SELECT \"name\",\"age\" FROM \"admin\".\"data\" WHERE (\"age\" >= 18) ORDER BY age  LIMIT 1000",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.arg)
			c := ColumnsFromURL(u)
			n, _ := TableNameFromURL(u)
			f := FiltersFromURL(u)

			got, err := FormSelectorDeleteQuery(u, f, c, n, "admin", "SELECT", "postgres")
			if got != tt.want {
				t.Errorf("formQuery() = %v, want %v", got, tt.want)
			}

			errMessage := ""
			if err != nil {
				errMessage = err.Error()
			}

			if errMessage != tt.wantErr {
				t.Errorf("formQuery() = %v", err)
			}
		})
	}
}

func Test_fullName(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		table    string
		provider string
		want     string
	}{
		{
			name:     "simple one-part name",
			user:     "admin",
			table:    "Accounts",
			provider: "postgres",
			want:     "\"admin\".\"Accounts\"",
		},
		{
			name:     "two-part name",
			user:     "admin",
			table:    "mary.Payroll",
			provider: "postgres",
			want:     "\"mary\".\"Payroll\"",
		},
		{
			name:     "sqlite3 one-part name",
			user:     "admin",
			table:    "Accounts",
			provider: "sqlite",
			want:     "\"Accounts\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := FullName(tt.provider, tt.user, tt.table)
			if got != tt.want {
				t.Errorf("fullName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_formCondition(t *testing.T) {
	tests := []struct {
		name      string
		condition string
		want      string
	}{
		{
			name:      "simple equality",
			condition: "EQ(rows,3)",
			want:      "(rows == 3)",
		},
		{
			name:      "Nested booleans",
			condition: `AND(EQ(rows,3),EQ(name, "Tom"))`,
			want:      `((rows == 3) && (name == "Tom"))`,
		},
		{
			name:      "contains",
			condition: `CONTAINS(name,"Tom")`,
			want:      `strings.Index(name,"Tom") >= 0 `,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := FormCondition(tt.condition); got != tt.want {
				t.Errorf("formCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMapColumnType_TimeTypes verifies that MapColumnType emits the correct DDL type
// string for date/time columns on both SQLite and PostgreSQL.
//
// For SQLite we expect the semantic names TIMESTAMP, TIME, and DATE rather than the
// generic TEXT.  SQLite treats these as TEXT affinity at the storage level, but declaring
// them with their semantic names allows schema introspection to recover the original intent
// so that CoerceToColumnType can convert values back to time.Time on the read path.
//
// For PostgreSQL we expect the full standard DDL names used by lib/pq.
func TestMapColumnType_TimeTypes(t *testing.T) {
	tests := []struct {
		name     string
		nativeType string // Ego / portable type name supplied by the caller
		provider string
		wantDDL  string  // expected SQL DDL type string
	}{
		// --- SQLite: semantic names preserved ---
		{
			name:       "SQLite timestamp -> TIMESTAMP",
			nativeType: "timestamp",
			provider:   defs.SqliteProvider,
			wantDDL:    "TIMESTAMP",
		},
		{
			name:       "SQLite time -> TIME",
			nativeType: "time",
			provider:   defs.SqliteProvider,
			wantDDL:    "TIME",
		},
		{
			name:       "SQLite date -> DATE",
			nativeType: "date",
			provider:   defs.SqliteProvider,
			wantDDL:    "DATE",
		},
		{
			name:       "SQLite deprecated alias: timestamp -> TIMESTAMP",
			nativeType: "timestamp",
			provider:   defs.DeprecatedSqliteProvider,
			wantDDL:    "TIMESTAMP",
		},

		// --- PostgreSQL: full DDL type names ---
		{
			name:       "PostgreSQL timestamp -> TIMESTAMP WITH TIME ZONE",
			nativeType: "timestamp",
			provider:   defs.PostgresProvider,
			wantDDL:    "TIMESTAMP WITH TIME ZONE",
		},
		{
			name:       "PostgreSQL time -> TIME",
			nativeType: "time",
			provider:   defs.PostgresProvider,
			wantDDL:    "TIME",
		},
		{
			name:       "PostgreSQL date -> DATE",
			nativeType: "date",
			provider:   defs.PostgresProvider,
			wantDDL:    "DATE",
		},

		// --- Non-time types must be unaffected ---
		{
			name:       "SQLite string still maps to TEXT",
			nativeType: "string",
			provider:   defs.SqliteProvider,
			wantDDL:    "TEXT",
		},
		{
			name:       "PostgreSQL string still maps to CHAR VARYING",
			nativeType: "string",
			provider:   defs.PostgresProvider,
			wantDDL:    "CHAR VARYING",
		},
	}

	for _, tt := range tests {
		got := MapColumnType(tt.nativeType, tt.provider)
		if got != tt.wantDDL {
			t.Errorf("%s: MapColumnType(%q, %q) = %q, want %q",
				tt.name, tt.nativeType, tt.provider, got, tt.wantDDL)
		}
	}
}

