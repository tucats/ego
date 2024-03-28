package parsing

import (
	"net/url"
	"reflect"
	"testing"
)

func TestFormUpdateQuery(t *testing.T) {
	type args struct {
		urlstring string
		items     map[string]interface{}
		user      string
		provider  string
	}

	// Test cases
	tests := []struct {
		name       string
		args       args
		want       string
		wantValues []interface{}
		wantErr    string
	}{
		{
			name: "simple update query of one field with bogus filter",
			args: args{
				urlstring: "http://example.com/tables/data/rows?filter=FAUX(id,1)",
				items:     map[string]interface{}{"owned_by": "John"},
				user:      "admin",
				provider:  "sqlite3",
			},
			wantErr: "unexpected token: Identifier(FAUX)",
		},
		{
			name: "simple update query of one field",
			args: args{
				urlstring: "http://example.com/tables/data/rows?filter=EQ(id,1)",
				items:     map[string]interface{}{"owned_by": "John"},
				user:      "admin",
				provider:  "sqlite3",
			},
			want:       `UPDATE data SET "owned_by"=$1 WHERE ("id" = 1)`,
			wantValues: []interface{}{"John"},
			wantErr:    "",
		},
		{
			name: "simple update query of two fields",
			args: args{
				urlstring: "http://example.com/tables/data/rows?filter=EQ(id,1)",
				items:     map[string]interface{}{"name": "John", "age": 30},
				user:      "admin",
				provider:  "sqlite3",
			},
			want:       `UPDATE data SET "age"=$1,"name"=$2 WHERE ("id" = 1)`,
			wantValues: []interface{}{30, "John"},
			wantErr:    "",
		},
		{
			name: "update query with multiple filters",
			args: args{
				urlstring: `http://example.com/tables/data/rows?filter=AND(EQ(name,"John"),EQ(id,1))`,
				items:     map[string]interface{}{"name": "John", "age": 30},
				user:      "admin",
				provider:  "sqlite3",
			},
			want:       `UPDATE data SET "age"=$1,"name"=$2 WHERE (("name" = 'John')  AND  ("id" = 1))`,
			wantValues: []interface{}{30, "John"},
			wantErr:    "",
		},
	}

	for _, tt := range tests {
		expectedQuery := tt.want

		u, _ := url.Parse(tt.args.urlstring)

		query, values, err := FormUpdateQuery(u, tt.args.user, tt.args.provider, tt.args.items)

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

		if !reflect.DeepEqual(values, tt.wantValues) {
			t.Errorf("%s, Unexpected values. Expected: %v, Got: %v", tt.name, tt.wantValues, values)
		}
	}
}

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
			want:    `DELETE FROM users WHERE ("name" = 'John')`,
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
			want:    `DELETE FROM users WHERE ("name" = 'John') AND ("age" > 30)`,
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
