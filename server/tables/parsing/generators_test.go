package parsing

import (
	"net/url"
	"testing"
)

func TestFormSelectorDeleteQuery(t *testing.T) {
	type args struct {
		urlstring string
		filter    []string
		columns   string
		table     string
		user      string
		verb      string
		provider  string
	}

	// Test cases
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr string
	}{
		{
			name: "invalid query filter operator",
			args: args{
				urlstring: "http://example.com",
				filter:    []string{`FAUX("age", 30)`},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  "sqlite3",
			},
			wantErr: "unexpected token: Identifier(FAUX)",
		},
		{
			name: "invalid query filter term list",
			args: args{
				urlstring: "http://example.com",
				filter:    []string{`EQ("age", )`},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  "sqlite3",
			},
			wantErr: "missing parenthesis",
		},
		{
			name: "simple query",
			args: args{
				urlstring: "http://example.com",
				filter:    []string{},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  "sqlite3",
			},
			want:    `SELECT "id","name","age" FROM users`,
			wantErr: "",
		},
		{
			name: "query with one sort column",
			args: args{
				urlstring: "http://example.com/tables/data?order=age",
				filter:    []string{},
				columns:   "",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  "sqlite3",
			},
			want:    `SELECT * FROM users ORDER BY "age"`,
			wantErr: "",
		},
		{
			name: "query with two sort columns",
			args: args{
				urlstring: "http://example.com/tables/data?order=age,name",
				filter:    []string{},
				columns:   "",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  "sqlite3",
			},
			want:    `SELECT * FROM users ORDER BY "age","name"`,
			wantErr: "",
		},
		{
			name: "query with one filter",
			args: args{
				urlstring: "http://example.com",
				filter:    []string{"EQ(name,\"John\")"},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "DELETE",
				provider:  "sqlite3",
			},
			want:    `DELETE  FROM users WHERE ("name" = 'John')`,
			wantErr: "",
		},
		{
			name: "query with two filters",
			args: args{
				urlstring: "http://example.com",
				filter:    []string{"EQ(name,\"John\")", "GT(age, 30)"},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "DELETE",
				provider:  "sqlite3",
			},
			want:    `DELETE  FROM users WHERE ("name" = 'John') AND ("age" > 30)`,
			wantErr: "",
		},
		{
			name: "query with two filters and one sort column",
			args: args{
				urlstring: "http://example.com/tables/data?order=name",
				filter:    []string{"EQ(name,\"John\")", "GT(age, 30)"},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  "sqlite3",
			},
			want:    `SELECT "id","name","age" FROM users WHERE ("name" = 'John') AND ("age" > 30) ORDER BY "name"`,
			wantErr: "",
		},
	}

	for _, tt := range tests {
		expectedQuery := tt.want

		u, _ := url.Parse(tt.args.urlstring)

		query, err := FormSelectorDeleteQuery(u,
			tt.args.filter,
			tt.args.columns,
			tt.args.table,
			tt.args.user,
			tt.args.verb,
			tt.args.provider)

		emsg := ""
		if err != nil {
			emsg = err.Error()
		}

		if emsg != tt.wantErr {
			t.Errorf("%s, Unexpected error: %v", tt.name, err)

			continue
		}

		if query != expectedQuery {
			t.Errorf("%s, Unexpected query. Expected: %s, Got: %s", tt.name, expectedQuery, query)
		}
	}
}
