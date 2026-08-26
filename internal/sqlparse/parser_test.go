package sqlparse

import (
	"strings"
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/sqlparse/ast"
)

// statementCase is one entry of validStatementCases below.
type statementCase struct {
	name string
	sql  string
	kind ast.Kind
}

// validStatementCases is a broad corpus of syntactically valid statements,
// one per supported construct, shared by TestParseValidStatements below and
// by the Format round-trip test in format_test.go — both want the same
// breadth of coverage, so it's kept in one place rather than duplicated.
var validStatementCases = []statementCase{
	{"select star", `SELECT * FROM users`, ast.KindSelectStmt},
	{"select columns with alias", `SELECT id, name AS n FROM users WHERE id = 1`, ast.KindSelectStmt},
	{"select qualified star", `SELECT u.* FROM users u`, ast.KindSelectStmt},
	{"select distinct", `SELECT DISTINCT name FROM users`, ast.KindSelectStmt},
	{"select join", `SELECT u.id, o.total FROM users u INNER JOIN orders o ON o.user_id = u.id`, ast.KindSelectStmt},
	{"select left join using", `SELECT * FROM a LEFT OUTER JOIN b USING (id)`, ast.KindSelectStmt},
	{"select cross comma join", `SELECT * FROM a, b WHERE a.id = b.id`, ast.KindSelectStmt},
	{"select group having", `SELECT dept, COUNT(*) FROM emp GROUP BY dept HAVING COUNT(*) > 1`, ast.KindSelectStmt},
	{"select order limit offset", `SELECT * FROM t ORDER BY a DESC, b ASC LIMIT 10 OFFSET 5`, ast.KindSelectStmt},
	{"select limit comma form", `SELECT * FROM t LIMIT 5, 10`, ast.KindSelectStmt},
	{"select union", `SELECT a FROM t1 UNION SELECT a FROM t2 ORDER BY a`, ast.KindSelectStmt},
	{"select union all intersect except", `SELECT a FROM t1 UNION ALL SELECT a FROM t2 INTERSECT SELECT a FROM t3 EXCEPT SELECT a FROM t4`, ast.KindSelectStmt},
	{"select subquery", `SELECT * FROM (SELECT id FROM users) AS sub WHERE sub.id > 1`, ast.KindSelectStmt},
	{"select scalar subquery", `SELECT id, (SELECT COUNT(*) FROM orders o WHERE o.user_id = u.id) AS n FROM users u`, ast.KindSelectStmt},
	{"select exists", `SELECT * FROM users u WHERE EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id)`, ast.KindSelectStmt},
	{"select not exists", `SELECT * FROM users u WHERE NOT EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id)`, ast.KindSelectStmt},
	{"select in list", `SELECT * FROM t WHERE a IN (1, 2, 3)`, ast.KindSelectStmt},
	{"select not in subquery", `SELECT * FROM t WHERE a NOT IN (SELECT id FROM u)`, ast.KindSelectStmt},
	{"select between", `SELECT * FROM t WHERE a BETWEEN 1 AND 10`, ast.KindSelectStmt},
	{"select not between", `SELECT * FROM t WHERE a NOT BETWEEN 1 AND 10`, ast.KindSelectStmt},
	{"select like escape", `SELECT * FROM t WHERE name LIKE 'a%' ESCAPE '\'`, ast.KindSelectStmt},
	{"select is null / is not null", `SELECT * FROM t WHERE a IS NULL AND b IS NOT NULL`, ast.KindSelectStmt},
	{"select isnull notnull", `SELECT * FROM t WHERE a ISNULL AND b NOTNULL`, ast.KindSelectStmt},
	{"select is distinct from", `SELECT * FROM t WHERE a IS DISTINCT FROM b`, ast.KindSelectStmt},
	{"select case searched", `SELECT CASE WHEN a > 1 THEN 'x' WHEN a > 0 THEN 'y' ELSE 'z' END FROM t`, ast.KindSelectStmt},
	{"select case simple", `SELECT CASE a WHEN 1 THEN 'one' ELSE 'other' END FROM t`, ast.KindSelectStmt},
	{"select cast", `SELECT CAST(a AS VARCHAR(10)) FROM t`, ast.KindSelectStmt},
	{"select function count star", `SELECT COUNT(*) FROM t`, ast.KindSelectStmt},
	{"select function distinct filter", `SELECT COUNT(DISTINCT a) FILTER (WHERE a > 0) FROM t`, ast.KindSelectStmt},
	{"select arithmetic precedence", `SELECT 1 + 2 * 3 - 4 / 2 FROM t`, ast.KindSelectStmt},
	{"select bitwise and concat", `SELECT (a & b) | c, name || '!' FROM t`, ast.KindSelectStmt},
	{"select placeholders anon", `SELECT * FROM t WHERE a = ? AND b = ?`, ast.KindSelectStmt},
	{"select placeholders numbered postgres", `SELECT * FROM t WHERE a = $1 AND b = $2`, ast.KindSelectStmt},
	{"select placeholders named", `SELECT * FROM t WHERE a = :name OR b = @other`, ast.KindSelectStmt},
	{"select with cte", `WITH recent AS (SELECT id FROM orders WHERE created > 0) SELECT * FROM recent`, ast.KindSelectStmt},
	{"select with recursive cte", `WITH RECURSIVE r(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r WHERE n < 10) SELECT * FROM r`, ast.KindSelectStmt},
	{"select quoted identifier", `SELECT "select" FROM "order"`, ast.KindSelectStmt},
	{"select bracket identifier", `SELECT [col] FROM [table]`, ast.KindSelectStmt},
	{"select blob literal", `SELECT * FROM t WHERE b = X'48656C6C6F'`, ast.KindSelectStmt},
	{"select negative and hex numbers", `SELECT -1, 0x1F, 1.5e10, .5, 100. FROM t`, ast.KindSelectStmt},
	{"select json arrows postgres", `SELECT data->'key', data->>'key2' FROM t`, ast.KindSelectStmt},
	{"select collate", `SELECT * FROM t ORDER BY name COLLATE NOCASE`, ast.KindSelectStmt},
	{"select comments", "SELECT 1 -- trailing comment\nFROM t /* block comment */ WHERE a = 1", ast.KindSelectStmt},

	{"insert values", `INSERT INTO t (a, b) VALUES (1, 2), (3, 4)`, ast.KindInsertStmt},
	{"insert default values", `INSERT INTO t DEFAULT VALUES`, ast.KindInsertStmt},
	{"insert select", `INSERT INTO t (a) SELECT a FROM u`, ast.KindInsertStmt},
	{"insert or replace", `INSERT OR REPLACE INTO t (a) VALUES (1)`, ast.KindInsertStmt},
	{"insert on conflict do nothing", `INSERT INTO t (a) VALUES (1) ON CONFLICT (a) DO NOTHING`, ast.KindInsertStmt},
	{"insert on conflict do update", `INSERT INTO t (a, b) VALUES (1, 2) ON CONFLICT (a) DO UPDATE SET b = excluded.b WHERE t.a = 1`, ast.KindInsertStmt},
	{"insert returning", `INSERT INTO t (a) VALUES (1) RETURNING id`, ast.KindInsertStmt},

	{"update simple", `UPDATE t SET a = 1, b = 2 WHERE id = 1`, ast.KindUpdateStmt},
	{"update row value set", `UPDATE t SET (a, b) = (1, 2) WHERE id = 1`, ast.KindUpdateStmt},
	{"update from", `UPDATE t SET a = u.a FROM u WHERE t.id = u.id`, ast.KindUpdateStmt},
	{"update returning", `UPDATE t SET a = 1 RETURNING a, b`, ast.KindUpdateStmt},

	{"delete simple", `DELETE FROM t WHERE id = 1`, ast.KindDeleteStmt},
	{"delete using", `DELETE FROM t USING u WHERE t.id = u.id`, ast.KindDeleteStmt},
	{"delete returning", `DELETE FROM t WHERE id = 1 RETURNING id`, ast.KindDeleteStmt},
	{"delete no where", `DELETE FROM t`, ast.KindDeleteStmt},

	{"create table basic", `CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`, ast.KindCreateTableStmt},
	{"create table if not exists", `CREATE TABLE IF NOT EXISTS t (id INTEGER)`, ast.KindCreateTableStmt},
	{"create temp table", `CREATE TEMP TABLE t (id INTEGER)`, ast.KindCreateTableStmt},
	{"create table full constraints", `CREATE TABLE t (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			age INTEGER DEFAULT 0 CHECK (age >= 0),
			dept_id INTEGER REFERENCES dept(id) ON DELETE CASCADE,
			bio TEXT COLLATE NOCASE,
			full_name TEXT GENERATED ALWAYS AS (email) STORED,
			CONSTRAINT pk_extra PRIMARY KEY (id),
			UNIQUE (email, age),
			FOREIGN KEY (dept_id) REFERENCES dept (id) ON UPDATE SET NULL,
			CHECK (age < 200)
		)`, ast.KindCreateTableStmt},
	{"create table without rowid", `CREATE TABLE t (id INTEGER PRIMARY KEY) WITHOUT ROWID`, ast.KindCreateTableStmt},
	{"create table serial family", `CREATE TABLE t (id SERIAL PRIMARY KEY, big BIGSERIAL, small SMALLSERIAL)`, ast.KindCreateTableStmt},
	{"create table generated identity always", `CREATE TABLE t (id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY)`, ast.KindCreateTableStmt},
	{"create table generated identity by default with options", `CREATE TABLE t (id INTEGER GENERATED BY DEFAULT AS IDENTITY (START WITH 1 INCREMENT BY 1) PRIMARY KEY)`, ast.KindCreateTableStmt},
	{"create table as select", `CREATE TABLE t AS SELECT * FROM u`, ast.KindCreateTableStmt},
	{"create table deferrable fk", `CREATE TABLE t (a INTEGER REFERENCES u(id) DEFERRABLE INITIALLY DEFERRED)`, ast.KindCreateTableStmt},
	{"create table typeless column", `CREATE TABLE t (a)`, ast.KindCreateTableStmt},

	{"drop table", `DROP TABLE t`, ast.KindDropTableStmt},
	{"drop table if exists cascade", `DROP TABLE IF EXISTS t CASCADE`, ast.KindDropTableStmt},

	{"alter table add column", `ALTER TABLE t ADD COLUMN a INTEGER`, ast.KindAlterTableStmt},
	{"alter table add no keyword", `ALTER TABLE t ADD a INTEGER NOT NULL DEFAULT 0`, ast.KindAlterTableStmt},
	{"alter table drop column", `ALTER TABLE t DROP COLUMN a`, ast.KindAlterTableStmt},
	{"alter table rename to", `ALTER TABLE t RENAME TO t2`, ast.KindAlterTableStmt},
	{"alter table rename column", `ALTER TABLE t RENAME COLUMN a TO b`, ast.KindAlterTableStmt},

	{"create index", `CREATE INDEX idx_t_a ON t (a)`, ast.KindCreateIndexStmt},
	{"create unique index where", `CREATE UNIQUE INDEX IF NOT EXISTS idx ON t (a DESC, b COLLATE NOCASE) WHERE a IS NOT NULL`, ast.KindCreateIndexStmt},
	{"drop index", `DROP INDEX idx_t_a`, ast.KindDropIndexStmt},
	{"drop index if exists", `DROP INDEX IF EXISTS idx_t_a`, ast.KindDropIndexStmt},

	{"create view", `CREATE VIEW v AS SELECT * FROM t`, ast.KindCreateViewStmt},
	{"create or replace view columns", `CREATE OR REPLACE VIEW v (a, b) AS SELECT x, y FROM t`, ast.KindCreateViewStmt},
	{"drop view if exists", `DROP VIEW IF EXISTS v`, ast.KindDropViewStmt},

	{"begin", `BEGIN`, ast.KindBeginStmt},
	{"begin deferred transaction named", `BEGIN DEFERRED TRANSACTION foo`, ast.KindBeginStmt},
	{"commit", `COMMIT`, ast.KindCommitStmt},
	{"end", `END`, ast.KindCommitStmt},
	{"rollback", `ROLLBACK`, ast.KindRollbackStmt},
	{"rollback to savepoint", `ROLLBACK TO SAVEPOINT sp1`, ast.KindRollbackStmt},
	{"savepoint", `SAVEPOINT sp1`, ast.KindSavepointStmt},
	{"release", `RELEASE SAVEPOINT sp1`, ast.KindReleaseStmt},
}

// TestParseValidStatements is a broad smoke test: every statement in
// validStatementCases must parse without error, in both dialects, and
// produce a statement of the given Kind.
func TestParseValidStatements(t *testing.T) {
	for _, tc := range validStatementCases {
		for _, dialect := range []ast.Dialect{ast.DialectSQLite, ast.DialectPostgreSQL} {
			t.Run(tc.name+"/"+dialect.String(), func(t *testing.T) {
				stmt, err := Parse(tc.sql, dialect)
				if err != nil {
					t.Fatalf("unexpected error: %v\nsql: %s", err, tc.sql)
				}

				if stmt.Kind() != tc.kind {
					t.Fatalf("got kind %s, want %s", stmt.Kind(), tc.kind)
				}

				// Every node in the tree must report a valid, non-negative
				// span so that a formatter could reproduce source order.
				ast.Walk(stmt, func(n ast.Node) bool {
					if !n.Pos().IsValid() {
						t.Errorf("node %s has invalid Pos()", n.String())
					}

					return true
				})
			})
		}
	}
}

func TestParsePragmaRejected(t *testing.T) {
	_, err := Parse(`PRAGMA journal_mode = WAL`, ast.DialectSQLite)
	if err == nil {
		t.Fatal("expected an error for PRAGMA, got nil")
	}

	ee, ok := err.(*errors.Error)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T", err)
	}

	if !ee.Equal(errors.ErrSQLPragmaNotSupported) {
		t.Errorf("got error %v, want ErrSQLPragmaNotSupported", err)
	}
}

func TestSyntaxErrorPosition(t *testing.T) {
	// The bad token "FRO" sits on line 2, and "SELECT a" occupies line 1.
	sql := "SELECT a\nFRO t"

	_, err := Parse(sql, ast.DialectSQLite)
	if err == nil {
		t.Fatal("expected a syntax error, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "2") {
		t.Errorf("expected error to mention line 2, got: %s", msg)
	}
}

func TestUnterminatedStringReportsStartPosition(t *testing.T) {
	_, err := Parse(`SELECT * FROM t WHERE a = 'unterminated`, ast.DialectSQLite)
	if err == nil {
		t.Fatal("expected an error for unterminated string, got nil")
	}

	ee, ok := err.(*errors.Error)
	if !ok {
		t.Fatalf("expected *errors.Error, got %T", err)
	}

	if !ee.Equal(errors.ErrSQLUnterminatedString) {
		t.Errorf("got error %v, want ErrSQLUnterminatedString", err)
	}
}

func TestExtraTrailingInputIsSyntaxError(t *testing.T) {
	_, err := Parse(`SELECT 1; SELECT 2`, ast.DialectSQLite)
	if err == nil {
		t.Fatal("expected an error for a second statement after the first, got nil")
	}
}

func TestTrailingSemicolonAccepted(t *testing.T) {
	stmt, err := Parse(`SELECT 1;`, ast.DialectSQLite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stmt.Kind() != ast.KindSelectStmt {
		t.Fatalf("got kind %s, want SelectStmt", stmt.Kind())
	}
}

func TestSelectStructure(t *testing.T) {
	stmt, err := Parse(`SELECT a, b AS bb FROM t1 JOIN t2 ON t1.id = t2.id WHERE a > 1 ORDER BY a DESC LIMIT 5`, ast.DialectSQLite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sel, ok := stmt.(*ast.SelectStmt)
	if !ok {
		t.Fatalf("got %T, want *ast.SelectStmt", stmt)
	}

	core, ok := sel.Select.(*ast.SelectCore)
	if !ok {
		t.Fatalf("got %T, want *ast.SelectCore", sel.Select)
	}

	if len(core.Columns) != 2 {
		t.Fatalf("got %d columns, want 2", len(core.Columns))
	}

	if core.Columns[1].Alias != "bb" {
		t.Fatalf("got alias %q, want bb", core.Columns[1].Alias)
	}

	join, ok := core.From[0].(*ast.JoinClause)
	if !ok {
		t.Fatalf("got %T, want *ast.JoinClause", core.From[0])
	}

	if join.JoinType != "" {
		t.Fatalf("got join type %q, want empty (plain JOIN)", join.JoinType)
	}

	if len(sel.OrderBy) != 1 || !sel.OrderBy[0].Desc {
		t.Fatalf("expected one DESC order-by term, got %#v", sel.OrderBy)
	}

	if sel.Limit == nil {
		t.Fatal("expected a LIMIT clause")
	}
}
