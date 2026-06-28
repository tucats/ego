package tables

import (
	"reflect"
	"testing"
)

func Test_splitSQLStatements(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []string
	}{
		{
			name: "single statement",
			sql:  `select * from "admin"."users"`,
			want: []string{
				`select * from "admin"."users"`,
			},
		},
		{
			name: "single statement with trailing semicolon",
			sql:  `select * from "admin"."users";`,
			want: []string{
				`select * from "admin"."users"`,
			},
		},
		{
			name: "single statement with quoted semicolon",
			sql:  `select * from "admin"."us;ers";`,
			want: []string{
				`select * from "admin"."us;ers"`,
			},
		},
		{
			name: "multiple statements",
			sql:  `select * from "admin"."users"; drop table "admin"."users"`,
			want: []string{
				`select * from "admin"."users"`,
				`drop table "admin"."users"`,
			},
		},
		{
			name: "multiple statements with trailing semicolon",
			sql:  `select * from "admin"."users"; drop table "admin"."users";`,
			want: []string{
				`select * from "admin"."users"`,
				`drop table "admin"."users"`,
			},
		},
		{
			name: "multiple statements with empty statement",
			sql:  `select * from "admin"."users";;;drop table "admin"."users"`,
			want: []string{
				`select * from "admin"."users"`,
				`drop table "admin"."users"`,
			},
		},
		{
			name: "single-quoted string value",
			sql:  `INSERT INTO t (name) VALUES ('Alice')`,
			want: []string{
				`INSERT INTO t (name) VALUES ('Alice')`,
			},
		},
		{
			name: "single-quoted string with space",
			sql:  `UPDATE t SET note = 'hello world' WHERE id = 1`,
			want: []string{
				`UPDATE t SET note = 'hello world' WHERE id = 1`,
			},
		},
		{
			name: "SQL escaped single quote (apostrophe in string)",
			sql:  `INSERT INTO t (name) VALUES ('it''s fine')`,
			want: []string{
				`INSERT INTO t (name) VALUES ('it''s fine')`,
			},
		},
		{
			name: "SQL escaped single quote at end of name",
			sql:  `SELECT * FROM t WHERE name = 'O''Brien'`,
			want: []string{
				`SELECT * FROM t WHERE name = 'O''Brien'`,
			},
		},
		{
			name: "semicolon inside single-quoted string is not a separator",
			sql:  `SELECT * FROM t WHERE note = 'a;b'; SELECT 1`,
			want: []string{
				`SELECT * FROM t WHERE note = 'a;b'`,
				`SELECT 1`,
			},
		},
		{
			name: "double-quoted identifier with escaped double-quote",
			sql:  `SELECT "my""col" FROM t`,
			want: []string{
				`SELECT "my""col" FROM t`,
			},
		},
		{
			name: "SQL line comment is stripped",
			sql:  "SELECT 1 -- this is a comment\nSELECT 2",
			want: []string{
				"SELECT 1 \nSELECT 2",
			},
		},
		{
			name: "SQL block comment is stripped",
			sql:  `SELECT /* ignore me */ 1`,
			want: []string{
				`SELECT  1`,
			},
		},
		{
			name: "Ego-style # comment lines are stripped",
			sql:  "# comment\nSELECT 1",
			want: []string{
				`SELECT 1`,
			},
		},
		{
			name: "Ego-style // comment lines are stripped",
			sql:  "// comment\nSELECT 1",
			want: []string{
				`SELECT 1`,
			},
		},
		{
			name: "multi-statement with single-quoted strings",
			sql:  `INSERT INTO t (name) VALUES ('Alice'); INSERT INTO t (name) VALUES ('O''Brien')`,
			want: []string{
				`INSERT INTO t (name) VALUES ('Alice')`,
				`INSERT INTO t (name) VALUES ('O''Brien')`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitSQLStatements(tt.sql)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitSQLStatements() =\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}
