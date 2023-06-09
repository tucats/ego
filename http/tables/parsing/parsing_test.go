package parsing

import (
	"net/url"
	"testing"
)

func Test_formWhereClause(t *testing.T) {
	tests := []struct {
		name    string
		filters []string
		want    string
	}{
		{
			name:    "bogus expression",
			filters: []string{"faux(name,\"string\")"},
			want:    "SYNTAX-ERROR:unexpected token: Identifier(faux)",
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

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formWhereExpressions(tt.filters); got != tt.want {
				t.Errorf("formWhereClause() = %v, want %v", got, tt.want)
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

		// TODO: Add test cases.
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
			want: ` WHERE (("a" = 1)  AND  ("b" = 2)  AND  ("c" = 3))`,
		},
		{
			name: "compound contains list",
			arg:  "https://localhost:8500/tables/data?filter=contains(foo, 'abc', 'def')",
			want: ` WHERE POSITION('abc' IN "foo") > 0 OR POSITION('def' IN "foo") > 0`,
		},
		{
			name: "compound hasall list",
			arg:  "https://localhost:8500/tables/data?filter=hasall(foo, 'abc', 'def')",
			want: ` WHERE POSITION('abc' IN "foo") > 0 AND POSITION('def' IN "foo") > 0`,
		},
		{
			name: "compound list",
			arg:  "https://localhost:8500/tables/data?filter=and(EQ(a,1),EQ(b,2),EQ(c,3))",
			want: ` WHERE (("a" = 1)  AND  ("b" = 2)  AND  ("c" = 3))`,
		},
		{
			name: "filter list",
			arg:  "https://localhost:8500/tables/data?filter=eq(name,\"Tom\"),eq(age,55)",
			want: " WHERE (\"name\" = 'Tom') AND (\"age\" = 55)",
		},
		{
			name: "no filter",
			arg:  "https://localhost:8500/tables/data",
			want: "",
		},
		{
			name: "one filter",
			arg:  "https://localhost:8500/tables/data?filter=eq(age,55)",
			want: " WHERE (\"age\" = 55)",
		},
		{
			name: "multiple filters",
			arg:  "https://localhost:8500/tables/data?filter=eq(name,\"Tom\")&filter=eq(name,\"Mary\")",
			want: " WHERE (\"name\" = 'Tom') AND (\"name\" = 'Mary')",
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.arg)
			f := FiltersFromURL(u)

			if got := WhereClause(f); got != tt.want {
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
			want: " ORDER BY \"tom\",\"age\"",
		},
		{
			name: "no sort",
			arg:  "https://localhost:8500/tables/data",
			want: "",
		},
		{
			name: "one sort",
			arg:  "https://localhost:8500/tables/data?order=age",
			want: " ORDER BY \"age\"",
		},
		{
			name: "one sort list",
			arg:  "https://localhost:8500/tables/data?order=age,name",
			want: " ORDER BY \"age\",\"name\"",
		},
		{
			name: "multiple sorts",
			arg:  "https://localhost:8500/tables/data?sort=name&sort=age",
			want: " ORDER BY \"name\",\"age\"",
		},
		{
			name: "descending sort",
			arg:  "https://localhost:8500/tables/data?sort=~age",
			want: " ORDER BY \"age\" DESC",
		},

		// TODO: Add test cases.
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
		name string
		arg  string
		want string
	}{
		{
			name: "no query parameters",
			arg:  "https://localhost:8500/tables/data",
			want: "SELECT * FROM \"admin\".\"data\"",
		},
		{
			name: "column specification",
			arg:  "https://localhost:8500/tables/data?columns=name,age",
			want: "SELECT \"name\",\"age\" FROM \"admin\".\"data\"",
		},
		{
			name: "column and sort specification",
			arg:  "https://localhost:8500/tables/data?order=age&columns=name,age",
			want: "SELECT \"name\",\"age\" FROM \"admin\".\"data\" ORDER BY \"age\"",
		},
		{
			name: "column, filter, and sort specification",
			arg:  "https://localhost:8500/tables/data?order=age&columns=name,age&filter=GE(age,18)",
			want: "SELECT \"name\",\"age\" FROM \"admin\".\"data\" WHERE (\"age\" >= 18) ORDER BY \"age\"",
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.arg)
			c := ColumnsFromURL(u)
			n, _ := TableNameFromURL(u)
			f := FiltersFromURL(u)

			if got := FormSelectorDeleteQuery(u, f, c, n, "admin", "SELECT", "postgres"); got != tt.want {
				t.Errorf("formQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_fullName(t *testing.T) {
	tests := []struct {
		name  string
		user  string
		table string
		want  string
	}{
		{
			name:  "simple one-part name",
			user:  "admin",
			table: "Accounts",
			want:  "\"admin\".\"Accounts\"",
		},
		{
			name:  "two-part name",
			user:  "admin",
			table: "mary.Payroll",
			want:  "\"mary\".\"Payroll\"",
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := FullName(tt.user, tt.table)
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
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormCondition(tt.condition); got != tt.want {
				t.Errorf("formCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}
