package database

import (
	"testing"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/router"
)

// TestOpen_PostgresUsesDSNSchemaNotIdentity is a regression test for the bug
// where the table names sent to a PostgreSQL server were qualified with the
// caller's Ego identity (session.User) instead of the DSN's own configured
// schema. Two Ego identities sharing the same restricted DSN must resolve to
// the identical Postgres schema -- the DSN's schema, not either identity's
// name -- and a DSN that leaves Schema unset must default to "public"
// (matching PostgreSQL's own default and defs.DSN.Schema's doc comment),
// not to whichever identity happened to open it.
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
	}

	// A DSN with no configured schema must default to "public" -- not to
	// the caller's Ego identity.
	db, _ := Open(sessionFor("admin"), "default-schema", dsns.DSNReadAction)
	if db == nil {
		t.Fatal("Open(default-schema) returned a nil *Database")
	}

	if db.User != defs.DefaultSchema {
		t.Errorf("Open(default-schema): db.User = %q, want %q", db.User, defs.DefaultSchema)
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
