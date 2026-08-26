package sqlparse

import "testing"

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
