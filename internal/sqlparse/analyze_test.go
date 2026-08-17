package sqlparse

import "testing"

func TestStatementKind(t *testing.T) {
	cases := []struct {
		sql  string
		want StatementKind
	}{
		{`SELECT 1`, StmtSelect},
		{`INSERT INTO t (a) VALUES (1)`, StmtInsert},
		{`UPDATE t SET a = 1`, StmtUpdate},
		{`DELETE FROM t`, StmtDelete},
		{`CREATE TABLE t (a INTEGER)`, StmtCreateTable},
		{`CREATE TABLE t AS SELECT * FROM u`, StmtCreateTable},
		{`DROP TABLE t`, StmtDropTable},
		{`ALTER TABLE t ADD COLUMN b INTEGER`, StmtAlterTable},
		{`CREATE INDEX idx ON t (a)`, StmtCreateIndex},
		{`DROP INDEX idx`, StmtDropIndex},
		{`CREATE VIEW v AS SELECT * FROM t`, StmtCreateView},
		{`DROP VIEW v`, StmtDropView},
		{`BEGIN`, StmtBegin},
		{`COMMIT`, StmtCommit},
		{`ROLLBACK`, StmtRollback},
		{`SAVEPOINT sp1`, StmtSavepoint},
		{`RELEASE sp1`, StmtRelease},
	}

	for _, tc := range cases {
		t.Run(tc.sql, func(t *testing.T) {
			p, err := New(tc.sql, SQLite)
			if err != nil {
				t.Fatalf("New: unexpected error: %v", err)
			}

			if got := p.StatementKind(); got != tc.want {
				t.Errorf("StatementKind() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestStatementKindString(t *testing.T) {
	if got := StmtCreateTable.String(); got != "CREATE TABLE" {
		t.Errorf("String() = %q, want %q", got, "CREATE TABLE")
	}

	if got := StatementKind(999).String(); got != "StatementKind(999)" {
		t.Errorf("String() = %q, want fallback form", got)
	}
}

func TestTablesSelect(t *testing.T) {
	p, err := New(`SELECT a FROM t1 JOIN t2 ON t1.id = t2.id WHERE t1.x IN (SELECT y FROM t3)`, SQLite)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	got := p.Tables()

	want := map[string]int{"t1": 1, "t2": 1, "t3": 1}
	assertTableUsages(t, got, want, UsageRead)
}

func TestTablesInsertSelect(t *testing.T) {
	p, err := New(`INSERT INTO t1 (a) SELECT a FROM t2`, SQLite)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	got := p.Tables()

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(got), got)
	}

	if got[0].Name != "t1" || got[0].Usage != UsageWrite {
		t.Errorf("got[0] = %+v, want {t1 write}", got[0])
	}

	if got[1].Name != "t2" || got[1].Usage != UsageRead {
		t.Errorf("got[1] = %+v, want {t2 read}", got[1])
	}
}

func TestTablesUpdateFrom(t *testing.T) {
	p, err := New(`UPDATE t1 SET a = t2.a FROM t2 WHERE t1.id = t2.id`, PostgreSQL)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	got := p.Tables()

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(got), got)
	}

	if got[0].Name != "t1" || got[0].Usage != UsageWrite {
		t.Errorf("got[0] = %+v, want {t1 write}", got[0])
	}

	if got[1].Name != "t2" || got[1].Usage != UsageRead {
		t.Errorf("got[1] = %+v, want {t2 read}", got[1])
	}
}

func TestTablesDeleteUsingSelfReference(t *testing.T) {
	p, err := New(`DELETE FROM t USING t AS old WHERE t.id = old.id`, PostgreSQL)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	got := p.Tables()

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(got), got)
	}

	if got[0].Name != "t" || got[0].Usage != UsageWrite {
		t.Errorf("got[0] = %+v, want {t write}", got[0])
	}

	if got[1].Name != "t" || got[1].Usage != UsageRead {
		t.Errorf("got[1] = %+v, want {t read}", got[1])
	}
}

func TestTablesCreateTableAsSelect(t *testing.T) {
	p, err := New(`CREATE TABLE t1 AS SELECT * FROM t2`, PostgreSQL)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	got := p.Tables()

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(got), got)
	}

	if got[0].Name != "t1" || got[0].Usage != UsageAdmin {
		t.Errorf("got[0] = %+v, want {t1 admin}", got[0])
	}

	if got[1].Name != "t2" || got[1].Usage != UsageRead {
		t.Errorf("got[1] = %+v, want {t2 read}", got[1])
	}
}

func TestTablesDropTable(t *testing.T) {
	p, err := New(`DROP TABLE main.t1`, SQLite)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	got := p.Tables()

	if len(got) != 1 || got[0].Name != "main.t1" || got[0].Usage != UsageAdmin {
		t.Errorf("got %+v, want [{main.t1 admin}]", got)
	}
}

func TestTablesCreateIndex(t *testing.T) {
	p, err := New(`CREATE INDEX idx ON t1 (a)`, SQLite)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	got := p.Tables()

	if len(got) != 1 || got[0].Name != "t1" || got[0].Usage != UsageAdmin {
		t.Errorf("got %+v, want [{t1 admin}]", got)
	}
}

func TestTablesDropIndexHasNoTable(t *testing.T) {
	p, err := New(`DROP INDEX idx`, SQLite)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	if got := p.Tables(); len(got) != 0 {
		t.Errorf("got %+v, want no entries", got)
	}
}

func TestTablesCreateView(t *testing.T) {
	p, err := New(`CREATE VIEW v1 AS SELECT * FROM t1`, SQLite)
	if err != nil {
		t.Fatalf("New: unexpected error: %v", err)
	}

	got := p.Tables()

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(got), got)
	}

	if got[0].Name != "v1" || got[0].Usage != UsageAdmin {
		t.Errorf("got[0] = %+v, want {v1 admin}", got[0])
	}

	if got[1].Name != "t1" || got[1].Usage != UsageRead {
		t.Errorf("got[1] = %+v, want {t1 read}", got[1])
	}
}

func TestTablesTransactionControlHasNoTables(t *testing.T) {
	for _, sql := range []string{"BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT sp1", "RELEASE sp1"} {
		p, err := New(sql, SQLite)
		if err != nil {
			t.Fatalf("New(%q): unexpected error: %v", sql, err)
		}

		if got := p.Tables(); len(got) != 0 {
			t.Errorf("Tables() for %q = %+v, want no entries", sql, got)
		}
	}
}

func TestUsageModeString(t *testing.T) {
	if got := UsageWrite.String(); got != "write" {
		t.Errorf("String() = %q, want %q", got, "write")
	}

	if got := UsageMode(999).String(); got != "UsageMode(999)" {
		t.Errorf("String() = %q, want fallback form", got)
	}
}

// assertTableUsages checks that got contains exactly the names in want (each
// with its expected count) and that every entry has the given usage mode.
func assertTableUsages(t *testing.T, got []TableUsage, want map[string]int, usage UsageMode) {
	t.Helper()

	counts := map[string]int{}

	for _, u := range got {
		if u.Usage != usage {
			t.Errorf("table %q has usage %s, want %s", u.Name, u.Usage, usage)
		}

		counts[u.Name]++
	}

	for name, n := range want {
		if counts[name] != n {
			t.Errorf("table %q appears %d times, want %d (got %+v)", name, counts[name], n, got)
		}
	}

	for name, n := range counts {
		if want[name] != n {
			t.Errorf("unexpected table %q appearing %d times (got %+v)", name, n, got)
		}
	}
}
