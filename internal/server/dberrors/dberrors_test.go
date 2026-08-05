package dberrors

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/lib/pq"
	egoerrors "github.com/tucats/ego/internal/errors"

	// Registers the "sqlite" driver used by newTestDB below.
	_ "modernc.org/sqlite"
)

// The SQLite half of these tests works by provoking real errors from a real
// database rather than by constructing driver errors by hand. *sqlite.Error has
// unexported fields, so it cannot be built directly -- but that turns out to be
// an advantage: it means these tests also check the result codes declared in
// dberrors.go against what the driver actually produces, which a hand-built
// fixture never would.

// newTestDB opens a scratch SQLite database with one table whose columns
// exercise each kind of constraint, and seeds a single row for the uniqueness
// tests to collide with.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// A file: URL with mode=memory keeps the database entirely in RAM, so the
	// test leaves nothing behind on disk and cannot collide with another test.
	db, err := sql.Open("sqlite", "file:dberrors_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("cannot open test database: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	schema := `CREATE TABLE t (
		id INTEGER PRIMARY KEY,
		u  INTEGER UNIQUE,
		nn INTEGER NOT NULL,
		c  INTEGER CHECK (c > 0)
	)`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("cannot create test table: %v", err)
	}

	if _, err := db.Exec("INSERT INTO t (id,u,nn,c) VALUES (1,1,1,1)"); err != nil {
		t.Fatalf("cannot seed test table: %v", err)
	}

	return db
}

// execError runs a statement that is expected to fail and returns its error.
func execError(t *testing.T, db *sql.DB, statement string) error {
	t.Helper()

	if _, err := db.Exec(statement); err != nil {
		return err
	}

	t.Fatalf("expected %q to fail, but it succeeded", statement)

	return nil
}

func TestClassify_SQLite(t *testing.T) {
	db := newTestDB(t)

	testCases := []struct {
		name      string
		statement string
		want      Class
	}{
		{
			name:      "missing table",
			statement: "SELECT * FROM nosuchtable",
			want:      NotFound,
		},
		{
			name:      "unique violation",
			statement: "INSERT INTO t (id,u,nn,c) VALUES (2,1,1,1)",
			want:      Conflict,
		},
		{
			name:      "primary key duplicate",
			statement: "INSERT INTO t (id,u,nn,c) VALUES (1,9,1,1)",
			want:      Conflict,
		},
		{
			name:      "not null violation",
			statement: "INSERT INTO t (id,u,nn,c) VALUES (3,3,NULL,1)",
			want:      InvalidValue,
		},
		{
			name:      "check violation",
			statement: "INSERT INTO t (id,u,nn,c) VALUES (4,4,1,-5)",
			want:      InvalidValue,
		},
		{
			// A syntax error carries the same generic result code as a missing
			// table, so this confirms the message check that separates them is
			// not simply classifying every generic error as NotFound.
			name:      "syntax error is not classified",
			statement: "THIS IS NOT SQL",
			want:      Unclassified,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := execError(t, db, testCase.statement)

			if got := Classify(err); got != testCase.want {
				t.Errorf("Classify(%v) = %v, want %v", err, got, testCase.want)
			}
		})
	}
}

func TestClassify_SQLiteSurvivesEgoWrapping(t *testing.T) {
	// Handlers frequently wrap a driver error with additional context before it
	// reaches a status decision. errors.As walks the chain, so classification
	// must survive that; if *errors.Error ever stopped implementing Unwrap,
	// every status would silently collapse to its caller's default.
	db := newTestDB(t)
	raw := execError(t, db, "INSERT INTO t (id,u,nn,c) VALUES (2,1,1,1)")

	wrapped := egoerrors.New(raw).Context("some.column")

	if got := Classify(wrapped); got != Conflict {
		t.Errorf("Classify(wrapped) = %v, want Conflict", got)
	}

	if got := ExecStatus(wrapped); got != http.StatusConflict {
		t.Errorf("ExecStatus(wrapped) = %d, want %d", got, http.StatusConflict)
	}
}

func TestClassify_Postgres(t *testing.T) {
	// lib/pq's error type has exported fields, so these can be built directly.
	// The SQLSTATE code is what a real PostgreSQL server sends.
	testCases := []struct {
		name string
		code string
		want Class
	}{
		{"undefined table", "42P01", NotFound},
		{"unique violation", "23505", Conflict},
		{"foreign key violation", "23503", Conflict},
		{"not null violation", "23502", InvalidValue},
		{"check violation", "23514", InvalidValue},
		{"syntax error is not classified", "42601", Unclassified},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := &pq.Error{Code: pq.ErrorCode(testCase.code), Message: "test"}

			if got := Classify(err); got != testCase.want {
				t.Errorf("Classify(SQLSTATE %s) = %v, want %v", testCase.code, got, testCase.want)
			}
		})
	}
}

func TestClassify_BothDriversAgree(t *testing.T) {
	// REST-1's headline defect was that a missing table produced a different
	// status depending on which database was behind the DSN, because each
	// handler recognized only one driver's wording. The same condition must now
	// classify identically whichever driver reports it.
	db := newTestDB(t)

	pairs := []struct {
		name      string
		sqliteSQL string
		pgCode    string
	}{
		{"missing table", "SELECT * FROM nosuchtable", "42P01"},
		{"unique violation", "INSERT INTO t (id,u,nn,c) VALUES (2,1,1,1)", "23505"},
		{"not null violation", "INSERT INTO t (id,u,nn,c) VALUES (3,3,NULL,1)", "23502"},
	}

	for _, pair := range pairs {
		t.Run(pair.name, func(t *testing.T) {
			sqliteClass := Classify(execError(t, db, pair.sqliteSQL))
			pgClass := Classify(&pq.Error{Code: pq.ErrorCode(pair.pgCode), Message: "test"})

			if sqliteClass != pgClass {
				t.Errorf("SQLite classified as %v but PostgreSQL as %v", sqliteClass, pgClass)
			}
		})
	}
}

func TestClassify_EgoErrors(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want Class
	}{
		{"permission", egoerrors.ErrNoPrivilegeForOperation, Permission},
		{"ambiguous timezone", egoerrors.ErrAmbiguousTimeZone, InvalidValue},
		{"invalid column name", egoerrors.ErrInvalidColumnName, InvalidValue},
		{"missing DSN", egoerrors.ErrNoSuchDSN, NotFound},
		{"missing transaction", egoerrors.ErrTransactionNotFound, NotFound},
		{"unrelated Ego error", egoerrors.ErrInvalidInteger, Unclassified},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := Classify(testCase.err); got != testCase.want {
				t.Errorf("Classify(%v) = %v, want %v", testCase.err, got, testCase.want)
			}
		})
	}
}

func TestClassify_PermissionIsLanguageIndependent(t *testing.T) {
	// The check this replaces was strings.Contains(err.Error(), "no privilege"),
	// and Error() renders using the process-wide language. Only the English
	// catalog entry contains that phrase, so on a server running in any other
	// language a permission denial was reported as 400 instead of 403.
	// Comparing identity has no such failure mode -- there is no text involved
	// at any point.
	err := egoerrors.ErrNoPrivilegeForOperation.Context("some.table")

	if got := Classify(err); got != Permission {
		t.Errorf("Classify() = %v, want Permission", got)
	}

	if got := PayloadStatus(err); got != http.StatusForbidden {
		t.Errorf("PayloadStatus() = %d, want %d", got, http.StatusForbidden)
	}
}

func TestClassify_Nil(t *testing.T) {
	if got := Classify(nil); got != Unclassified {
		t.Errorf("Classify(nil) = %v, want Unclassified", got)
	}
}

func TestStatusDefaults(t *testing.T) {
	// An unrecognized error takes the caller's default, and the two callers
	// differ deliberately: before the query runs, an unknown failure is the
	// payload's fault; after it runs, it is the server's.
	unknown := egoerrors.ErrInvalidInteger

	if got := PayloadStatus(unknown); got != http.StatusBadRequest {
		t.Errorf("PayloadStatus(unknown) = %d, want %d", got, http.StatusBadRequest)
	}

	if got := ExecStatus(unknown); got != http.StatusInternalServerError {
		t.Errorf("ExecStatus(unknown) = %d, want %d", got, http.StatusInternalServerError)
	}
}

func TestStatusMapping(t *testing.T) {
	// Each recognized class maps to one status regardless of which entry point
	// asked. docs/API.md documents this table for clients; the two must agree.
	db := newTestDB(t)

	testCases := []struct {
		name      string
		statement string
		want      int
	}{
		{"missing table is 404", "SELECT * FROM nosuchtable", http.StatusNotFound},
		{"uniqueness conflict is 409", "INSERT INTO t (id,u,nn,c) VALUES (2,1,1,1)", http.StatusConflict},
		{"rejected value is 400", "INSERT INTO t (id,u,nn,c) VALUES (3,3,NULL,1)", http.StatusBadRequest},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := execError(t, db, testCase.statement)

			if got := ExecStatus(err); got != testCase.want {
				t.Errorf("ExecStatus() = %d, want %d", got, testCase.want)
			}

			if got := PayloadStatus(err); got != testCase.want {
				t.Errorf("PayloadStatus() = %d, want %d", got, testCase.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The DSN layer (REST-2)
// ---------------------------------------------------------------------------

func TestClassify_MissingDSNIsNotFound(t *testing.T) {
	// A DSN named in a URL path that does not exist is as much a not-found
	// condition as a missing table. Before REST-2 this reached the handlers
	// unclassified and was reported as 400 by most of them, 500 by three, and
	// 200 by the @transaction endpoint.
	err := egoerrors.ErrNoSuchDSN.Context("nosuchdsn")

	if got := Classify(err); got != NotFound {
		t.Errorf("Classify() = %v, want NotFound", got)
	}

	if got := PayloadStatus(err); got != http.StatusNotFound {
		t.Errorf("PayloadStatus() = %d, want %d", got, http.StatusNotFound)
	}
}

func TestClassify_DSNPermissionOutranksNotFound(t *testing.T) {
	// database.Open returns ErrNoPrivilegeForOperation for a DSN that exists but
	// this user may not use, and ErrNoSuchDSN when it is not there at all. The
	// two must stay distinguishable: a caller denied access should be told so
	// (403) rather than told the DSN does not exist (404).
	denied := egoerrors.ErrNoPrivilegeForOperation.Context("somedsn")

	if got := Classify(denied); got != Permission {
		t.Errorf("Classify(denied) = %v, want Permission", got)
	}

	if got := PayloadStatus(denied); got != http.StatusForbidden {
		t.Errorf("PayloadStatus(denied) = %d, want %d", got, http.StatusForbidden)
	}
}

func TestClassify_MissingTransactionIsNotFound(t *testing.T) {
	// An unknown or expired transaction id names something that is not there,
	// so it takes the same class as a missing table or DSN.
	err := egoerrors.ErrTransactionNotFound.Context("bogus-id")

	if got := PayloadStatus(err); got != http.StatusNotFound {
		t.Errorf("PayloadStatus() = %d, want %d", got, http.StatusNotFound)
	}
}
