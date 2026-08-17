package sqlparse

import (
	"strings"
	"testing"

	"github.com/tucats/ego/internal/sqlparse/ast"
)

// TestFormatRoundTrips runs every statement in validStatementCases (shared
// with TestParseValidStatements in parser_test.go) through New and Format,
// then re-parses Format's output and checks it succeeds and still produces
// the same statement Kind. This is the main correctness check for Format:
// rather than hand-asserting exact output text for dozens of constructs
// (brittle, and not really what matters), it checks the one property that
// actually matters for a formatter — that it always produces valid,
// equivalent-shaped SQL — across the same breadth of syntax the parser
// tests already cover.
func TestFormatRoundTrips(t *testing.T) {
	for _, tc := range validStatementCases {
		for _, dialect := range []int{SQLite, PostgreSQL} {
			t.Run(tc.name, func(t *testing.T) {
				p, err := New(tc.sql, dialect)
				if err != nil {
					t.Fatalf("New: unexpected error: %v\nsql: %s", err, tc.sql)
				}

				formatted := p.Format()

				p2, err := New(formatted, dialect)
				if err != nil {
					t.Fatalf("Format produced unparseable SQL: %v\n--- formatted ---\n%s\n--- original ---\n%s", err, formatted, tc.sql)
				}

				if p2.Statement().Kind() != tc.kind {
					t.Fatalf("got kind %s after round-trip, want %s\n--- formatted ---\n%s", p2.Statement().Kind(), tc.kind, formatted)
				}
			})
		}
	}
}

func TestFormatUppercasesKeywords(t *testing.T) {
	p, err := New(`select a, b from t where a = 1 order by a limit 5`, SQLite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := p.Format()

	for _, kw := range []string{"SELECT", "FROM", "WHERE", "ORDER BY", "LIMIT"} {
		if !strings.Contains(got, kw) {
			t.Errorf("expected formatted output to contain %q, got:\n%s", kw, got)
		}
	}

	if strings.Contains(got, "select") || strings.Contains(got, "from") {
		t.Errorf("expected no lower-case keywords in formatted output, got:\n%s", got)
	}
}

func TestFormatUsesLineBreaksForClauses(t *testing.T) {
	p, err := New(`SELECT a FROM t WHERE a = 1 GROUP BY a HAVING a > 0 ORDER BY a LIMIT 1`, SQLite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := p.Format()

	for _, clause := range []string{"FROM", "WHERE", "GROUP BY", "HAVING", "ORDER BY", "LIMIT"} {
		idx := strings.Index(got, clause)
		if idx <= 0 {
			t.Fatalf("expected %q in output, got:\n%s", clause, got)
		}

		if got[idx-1] != '\n' {
			t.Errorf("expected %q to start a new line, got:\n%s", clause, got)
		}
	}
}

func TestFormatIndentsSubqueries(t *testing.T) {
	p, err := New(`SELECT * FROM (SELECT id FROM users) AS sub`, SQLite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := p.Format()

	if !strings.Contains(got, "\n    SELECT id") {
		t.Errorf("expected the nested SELECT to be indented one level, got:\n%s", got)
	}
}

func TestFormatIdentifierQuoting(t *testing.T) {
	// PostgreSQL: every identifier is quoted, preserving mixed case that
	// would otherwise be folded to lower case on the next parse.
	p, err := New(`SELECT MyColumn FROM MyTable`, PostgreSQL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := p.Format()

	if !strings.Contains(got, `"MyColumn"`) || !strings.Contains(got, `"MyTable"`) {
		t.Errorf("expected quoted mixed-case identifiers under PostgreSQL, got:\n%s", got)
	}

	// sqlite3: an ordinary bare identifier is never quoted.
	p, err = New(`SELECT MyColumn FROM MyTable`, SQLite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got = p.Format()

	if strings.Contains(got, `"`) {
		t.Errorf("expected no quoting for bare identifiers under sqlite3, got:\n%s", got)
	}

	// sqlite3: an identifier that isn't a bare word must still be quoted.
	p, err = New(`SELECT "weird col" FROM t`, SQLite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got = p.Format()

	if !strings.Contains(got, `"weird col"`) {
		t.Errorf("expected a non-bare identifier to stay quoted under sqlite3, got:\n%s", got)
	}
}

func TestNewInvalidDialect(t *testing.T) {
	_, err := New(`SELECT 1`, 99)
	if err == nil {
		t.Fatal("expected an error for an invalid dialect, got nil")
	}
}

func TestNewAndStatement(t *testing.T) {
	p, err := New(`SELECT 1`, SQLite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Statement().Kind() != ast.KindSelectStmt {
		t.Fatalf("got kind %s, want SelectStmt", p.Statement().Kind())
	}
}
