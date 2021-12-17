package dbtables

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
			name:    "nested expression",
			filters: []string{"or(eq(name,\"Tom\"),eq(name,\"Mary\"))"},
			want:    "name = 'Tom' OR name = 'Mary'",
		},
		{
			name:    "string constant",
			filters: []string{"eq(name,\"Tom\")"},
			want:    "name = 'Tom'",
		},
		{
			name:    "simple equality",
			filters: []string{"eq(age,55)"},
			want:    "age = 55",
		},
		{
			name:    "unary not",
			filters: []string{"not(age)"},
			want:    "NOT age",
		},
		{
			name:    "simple list",
			filters: []string{"lt(age,18)", "gt(age,65)"},
			want:    "age < 18 AND age > 65",
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formWhereClause(tt.filters); got != tt.want {
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
			want: "age",
		},
		{
			name: "multiple columns",
			arg:  "https://localhost:8500/tables/data?column=name&column=age",
			want: "name,age",
		},
		{
			name: "column list",
			arg:  "https://localhost:8500/tables/data?column=name,age",
			want: "name,age",
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.arg)
			if got := columnList(u); got != tt.want {
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
			name: "filter list",
			arg:  "https://localhost:8500/tables/data?filter=eq(name,\"Tom\"),eq(age,55)",
			want: "WHERE name = 'Tom' AND age = 55",
		},
		{
			name: "no filter",
			arg:  "https://localhost:8500/tables/data",
			want: "",
		},
		{
			name: "one filter",
			arg:  "https://localhost:8500/tables/data?filter=eq(age,55)",
			want: "WHERE age = 55",
		},
		{
			name: "multiple filters",
			arg:  "https://localhost:8500/tables/data?filter=eq(name,\"Tom\")&filter=eq(name,\"Mary\")",
			want: "WHERE name = 'Tom' AND name = 'Mary'",
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.arg)
			if got := filterList(u); got != tt.want {
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
			name: "multiple sorts",
			arg:  "https://localhost:8500/tables/data?sort=name&sort=age",
			want: "ORDER BY name,age",
		},
		{
			name: "descending sort",
			arg:  "https://localhost:8500/tables/data?sort=~age",
			want: "ORDER BY age DESC",
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.arg)
			if got := sortList(u); got != tt.want {
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
			want: "SELECT * FROM data",
		},
		{
			name: "column specification",
			arg:  "https://localhost:8500/tables/data?columns=name,age",
			want: "SELECT name,age FROM data",
		},
		{
			name: "column and sort specification",
			arg:  "https://localhost:8500/tables/data?order=age&columns=name,age",
			want: "SELECT name,age FROM data ORDER BY age",
		},
		{
			name: "column, filter, and sort specification",
			arg:  "https://localhost:8500/tables/data?order=age&columns=name,age&filter=GE(age,18)",
			want: "SELECT name,age FROM data WHERE age >= 18 ORDER BY age",
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.arg)
			if got := formQuery(u, "", selectVerb); got != tt.want {
				t.Errorf("formQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}
