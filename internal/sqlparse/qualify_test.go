package sqlparse

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
)

// TestQualifyTables covers every statement kind that names a table, view, or
// index: the *ast.TableRef-based kinds (SELECT/INSERT/UPDATE/DELETE/CREATE
// TABLE/DROP TABLE/ALTER TABLE), reached generically via ast.Walk, and the
// plain-string-field kinds (CREATE/DROP INDEX, CREATE/DROP VIEW) that are
// qualified explicitly. An already-qualified reference must be left alone,
// and an empty schema argument must be a no-op.
func TestQualifyTables(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "insert",
			sql:  "INSERT INTO names (age, id) VALUES (66, 101)",
			want: "INSERT INTO \"foo\".\"names\" (\"age\", \"id\")\nVALUES (66, 101)",
		},
		{
			name: "select",
			sql:  "SELECT * FROM names WHERE id = 1",
			want: "SELECT *\nFROM \"foo\".\"names\"\nWHERE \"id\" = 1",
		},
		{
			name: "select with join",
			sql:  "SELECT * FROM a JOIN b ON a.id = b.id",
			want: "SELECT *\nFROM \"foo\".\"a\"\nJOIN \"foo\".\"b\" ON \"a\".\"id\" = \"b\".\"id\"",
		},
		{
			name: "already-qualified reference is untouched",
			sql:  "SELECT * FROM other.names",
			want: "SELECT *\nFROM \"other\".\"names\"",
		},
		{
			name: "update",
			sql:  "UPDATE names SET age = 1 WHERE id = 2",
			want: "UPDATE \"foo\".\"names\"\nSET \"age\" = 1\nWHERE \"id\" = 2",
		},
		{
			name: "delete",
			sql:  "DELETE FROM names WHERE id = 2",
			want: "DELETE FROM \"foo\".\"names\"\nWHERE \"id\" = 2",
		},
		{
			name: "create table",
			sql:  "CREATE TABLE names (id INT)",
			want: "CREATE TABLE \"foo\".\"names\" (\n    \"id\" INT\n)",
		},
		{
			name: "drop table",
			sql:  "DROP TABLE names",
			want: "DROP TABLE \"foo\".\"names\"",
		},
		{
			name: "alter table",
			sql:  "ALTER TABLE names RENAME TO people",
			want: "ALTER TABLE \"foo\".\"names\" RENAME TO \"people\"",
		},
		{
			name: "create index qualifies the table, not the index name",
			sql:  "CREATE INDEX idx1 ON names (id)",
			want: "CREATE INDEX \"idx1\" ON \"foo\".\"names\" (\"id\")",
		},
		{
			name: "drop index",
			sql:  "DROP INDEX idx1",
			want: "DROP INDEX \"foo\".\"idx1\"",
		},
		{
			name: "create view qualifies both the view and its body",
			sql:  "CREATE VIEW v1 AS SELECT * FROM names",
			want: "CREATE VIEW \"foo\".\"v1\" AS\nSELECT *\nFROM \"foo\".\"names\"",
		},
		{
			name: "drop view",
			sql:  "DROP VIEW v1",
			want: "DROP VIEW \"foo\".\"v1\"",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := New(c.sql, PostgreSQL)
			if err != nil {
				t.Fatalf("parse %q: %v", c.sql, err)
			}

			p.QualifyTables("foo")

			if got := p.Format(); got != c.want {
				t.Errorf("QualifyTables(%q):\n got:  %q\n want: %q", c.sql, got, c.want)
			}
		})
	}
}

// TestQualifyTables_EmptySchemaIsNoOp confirms that calling QualifyTables
// with an empty schema leaves the statement exactly as it parsed, since
// there is nothing to fill in.
func TestQualifyTables_EmptySchemaIsNoOp(t *testing.T) {
	const sql = "SELECT * FROM names"

	p, err := New(sql, PostgreSQL)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}

	before := p.Format()

	p.QualifyTables("")

	if got := p.Format(); got != before {
		t.Errorf("QualifyTables(\"\") changed the statement:\n before: %q\n after:  %q", before, got)
	}
}

// TestRestrictToSchema covers every statement kind RestrictToSchema checks:
// the *ast.TableRef-based kinds reached via ast.Walk, and the plain-string-
// field DDL kinds (CREATE/DROP INDEX, CREATE/DROP VIEW). An unqualified
// reference and one that already names the allowed schema must both pass; a
// reference naming any other schema (including something like pg_catalog)
// must be rejected.
func TestRestrictToSchema(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{name: "unqualified reference is allowed", sql: "SELECT * FROM names"},
		{name: "reference to the allowed schema is allowed", sql: "SELECT * FROM foo.names"},
		{name: "reference to another schema is rejected", sql: "SELECT * FROM other.names", wantErr: true},
		{name: "reference to a system schema is rejected", sql: "SELECT * FROM pg_catalog.pg_tables", wantErr: true},
		{name: "join across schemas is rejected", sql: "SELECT * FROM foo.a JOIN other.b ON a.id = b.id", wantErr: true},
		{name: "insert into another schema is rejected", sql: "INSERT INTO other.names (id) VALUES (1)", wantErr: true},
		{name: "update in another schema is rejected", sql: "UPDATE other.names SET id = 1", wantErr: true},
		{name: "delete from another schema is rejected", sql: "DELETE FROM other.names", wantErr: true},
		{name: "create table in another schema is rejected", sql: "CREATE TABLE other.names (id INT)", wantErr: true},
		{name: "drop index in another schema is rejected", sql: "DROP INDEX other.idx1", wantErr: true},
		{name: "create view in another schema is rejected", sql: "CREATE VIEW other.v1 AS SELECT * FROM foo.names", wantErr: true},
		{name: "create view body referencing another schema is rejected", sql: "CREATE VIEW foo.v1 AS SELECT * FROM other.names", wantErr: true},
		{name: "drop view in another schema is rejected", sql: "DROP VIEW other.v1", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := New(c.sql, PostgreSQL)
			if err != nil {
				t.Fatalf("parse %q: %v", c.sql, err)
			}

			err = p.RestrictToSchema("foo")
			if c.wantErr && err == nil {
				t.Fatalf("RestrictToSchema(%q): expected an error, got none", c.sql)
			}

			if !c.wantErr && err != nil {
				t.Fatalf("RestrictToSchema(%q): unexpected error: %v", c.sql, err)
			}

			if err != nil {
				ee, ok := err.(*errors.Error)
				if !ok {
					t.Fatalf("expected *errors.Error, got %T", err)
				}

				if !ee.Equal(errors.ErrSQLSchemaRestricted) {
					t.Errorf("got error %v, want ErrSQLSchemaRestricted", err)
				}
			}
		})
	}
}

// TestRestrictToSchema_EmptySchemaIsNoOp confirms that calling
// RestrictToSchema with an empty schema never rejects anything, matching a
// DSN with no schema of its own, which allows any explicit schema.
func TestRestrictToSchema_EmptySchemaIsNoOp(t *testing.T) {
	const sql = "SELECT * FROM other.names"

	p, err := New(sql, PostgreSQL)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}

	if err := p.RestrictToSchema(""); err != nil {
		t.Errorf("RestrictToSchema(\"\") returned an error: %v", err)
	}
}
