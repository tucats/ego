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
				`select * from "admin" . "users"`,
			},
		},
		{
			name: "single statement with trailing semicolon",
			sql:  `select * from "admin"."users";`,
			want: []string{
				`select * from "admin" . "users"`,
			},
		},
		{
			name: "single statement with quoted semicolon",
			sql:  `select * from "admin"."us;ers";`,
			want: []string{
				`select * from "admin" . "us;ers"`,
			},
		},
		{
			name: "multiple statements",
			sql:  `select * from "admin"."users"; drop table "admin"."users"`,
			want: []string{
				`select * from "admin" . "users"`,
				`drop table "admin" . "users"`,
			},
		},
		{
			name: "multiple statements with trailing semicolon",
			sql:  `select * from "admin"."users"; drop table "admin"."users";`,
			want: []string{
				`select * from "admin" . "users"`,
				`drop table "admin" . "users"`,
			},
		},
		{
			name: "multiple statements with empty statement",
			sql:  `select * from "admin"."users";;;drop table "admin"."users"`,
			want: []string{
				`select * from "admin" . "users"`,
				`drop table "admin" . "users"`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitSQLStatements(tt.sql); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitSQLStatements() = %v, want %v", got, tt.want)
			}
		})
	}
}
