package database

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/router"

	_ "modernc.org/sqlite"
)

// TestOpen_PostgresUsesDSNSchemaNotIdentity is a regression test for the bug
// where the table names sent to a PostgreSQL server were qualified with the
// caller's Ego identity (session.User) instead of the DSN's own configured
// schema. Two Ego identities sharing the same restricted DSN must resolve to
// the identical Postgres schema -- the DSN's schema, not either identity's
// name -- and a DSN that leaves Schema unset must default to "public"
// (matching PostgreSQL's own default and defs.DSN.Schema's doc comment),
// not to whichever identity happened to open it.
//
// It also covers RestrictSchema: a DSN with an explicit schema must set it
// (so sqlparse.RestrictToSchema's callers sandbox raw SQL to that schema),
// while a DSN that left Schema blank -- and so was defaulted to "public"
// above -- must not, since it never opted into a schema sandbox at all.
func TestOpen_PostgresUsesDSNSchemaNotIdentity(t *testing.T) {
	svc, err := dsns.NewFileService(defs.MemoryProvider)
	if err != nil {
		t.Fatalf("create test DSN service: %v", err)
	}

	dsns.DSNService = svc

	writeDSN := func(name, schema string) {
		t.Helper()

		if err := dsns.DSNService.WriteDSN(1, "admin", defs.DSN{
			Name:     name,
			Provider: defs.PostgresProvider,
			Database: "somedb",
			Host:     "localhost",
			Schema:   schema,
		}); err != nil {
			t.Fatalf("write DSN %q: %v", name, err)
		}
	}

	writeDSN("explicit-schema", "myschema")
	writeDSN("default-schema", "")

	// Two different Ego identities, admin standing granted via session.Admin
	// so the test exercises only the schema resolution, not DSN authorization.
	sessionFor := func(user string) *router.Session {
		return &router.Session{ID: 1, User: user, Admin: true}
	}

	// A DSN with an explicit schema must produce that schema for every
	// caller, regardless of the caller's own Ego identity.
	for _, user := range []string{"admin", "tom", "someoneelse"} {
		db, _ := Open(sessionFor(user), "explicit-schema", dsns.DSNReadAction)
		if db == nil {
			t.Fatalf("Open(%q) returned a nil *Database", user)
		}

		if db.User != "myschema" {
			t.Errorf("Open(%q): db.User = %q, want %q (the DSN's configured schema)", user, db.User, "myschema")
		}

		if db.Schema != "myschema" {
			t.Errorf("Open(%q): db.Schema = %q, want %q", user, db.Schema, "myschema")
		}

		if !db.RestrictSchema {
			t.Errorf("Open(%q): db.RestrictSchema = false, want true for a DSN with an explicit schema", user)
		}
	}

	// A DSN with no configured schema must default to "public" -- not to
	// the caller's Ego identity -- and must not restrict raw SQL to any
	// particular schema, since it never named one of its own.
	db, _ := Open(sessionFor("admin"), "default-schema", dsns.DSNReadAction)
	if db == nil {
		t.Fatal("Open(default-schema) returned a nil *Database")
	}

	if db.User != defs.DefaultSchema {
		t.Errorf("Open(default-schema): db.User = %q, want %q", db.User, defs.DefaultSchema)
	}

	if db.RestrictSchema {
		t.Error("Open(default-schema): db.RestrictSchema = true, want false for a DSN with no configured schema")
	}
}

// TestOpen_ConcurrentRequestsShareOnePool_NoUseAfterClose guards the risky
// part of the per-DSN connection pool cache: Close/CloseTX must not close a
// pooled *sql.DB, because it is shared with every other request currently
// using the same DSN. If that guard were ever lost, this test would flake
// with "sql: database is closed" as one goroutine's deferred Close() tears
// the pool down out from under the others' concurrent queries.
func TestOpen_ConcurrentRequestsShareOnePool_NoUseAfterClose(t *testing.T) {
	svc, err := dsns.NewFileService(defs.MemoryProvider)
	if err != nil {
		t.Fatalf("create test DSN service: %v", err)
	}

	dsns.DSNService = svc

	path := filepath.Join(t.TempDir(), "concurrent.db")

	if err := dsns.DSNService.WriteDSN(1, "admin", defs.DSN{
		Name:     "concurrent-dsn",
		Provider: defs.SqliteProvider,
		Database: path,
	}); err != nil {
		t.Fatalf("write DSN: %v", err)
	}

	session := &router.Session{ID: 1, User: "admin", Admin: true}

	// Force internal/cli/settings' lazily-initialized CurrentConfiguration to
	// exist before any concurrent access, exactly as happens in the real
	// server (config is loaded once, single-threaded, before the HTTP
	// listener starts accepting requests). Without this, 50 goroutines can
	// race the very first settings.Get call's lazy init -- a pre-existing
	// issue in that package (unsynchronized check-then-init of a package-level
	// global), unrelated to this test's purpose, and not one this test is
	// responsible for guarding.
	settings.Get(defs.DBPoolEnabledSetting)

	const goroutines = 50

	errs := make([]error, goroutines)

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()

			db, err := Open(session, "concurrent-dsn", dsns.DSNReadAction)
			if err != nil {
				errs[i] = err

				return
			}

			defer db.Close()

			rows, err := db.Query("SELECT 1")
			if err != nil {
				errs[i] = err

				return
			}

			rows.Close()
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: %v", i, err)
		}
	}
}

func TestMasked_InvalidURL(t *testing.T) {
	input := "invalid_url"
	expected := input

	result := redactURLString(input)

	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}

func TestMasked_SensitiveInfoInPath(t *testing.T) {
	input := "postgres://user:password@localhost/dbname/sensitive_info"
	expected := "postgres://user:xxxxx@localhost/dbname/sensitive_info"

	result := redactURLString(input)

	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}

func TestMasked_Sqlite(t *testing.T) {
	input := "sqlite3://foo.db"
	expected := "sqlite3://foo.db"

	result := redactURLString(input)

	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}
