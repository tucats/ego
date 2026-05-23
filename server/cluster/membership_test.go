package cluster

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	"github.com/tucats/ego/defs"
)

// createTestTable creates the cluster membership table in the given database.
func createTestTable(t *testing.T, db *sql.DB) {
	t.Helper()

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cluster (
			name      TEXT NOT NULL,
			node_id   TEXT NOT NULL PRIMARY KEY,
			host      TEXT NOT NULL,
			port      INTEGER NOT NULL,
			scheme    TEXT NOT NULL DEFAULT 'https',
			joined_at TEXT NOT NULL,
			last_seen TEXT NOT NULL,
			state     TEXT NOT NULL DEFAULT 'active'
		)`)
	if err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
}

// testMembershipOps runs the full suite of membership operations against
// whatever *sql.DB is provided. This is called once for SQLite and once
// for PostgreSQL.
func testMembershipOps(t *testing.T, db *sql.DB) {
	t.Helper()

	ClusterName = "test-cluster"
	now := time.Now().UTC().Format(time.RFC3339)

	createTestTable(t, db)

	// ListMembers on an empty table must return an empty slice, not an error.
	members, err := ListMembers(db)
	if err != nil {
		t.Fatalf("ListMembers (empty): %v", err)
	}

	if len(members) != 0 {
		t.Fatalf("ListMembers (empty): expected 0 members, got %d", len(members))
	}

	// Insert a member via upsertMember.
	m1 := defs.ClusterMember{
		Name: "test-cluster", NodeID: "node-aaa",
		Host: "host1", Port: 4040, Scheme: "https",
		JoinedAt: now, LastSeen: now, State: "active",
	}

	if err := upsertMember(db, m1); err != nil {
		t.Fatalf("upsertMember (insert): %v", err)
	}

	members, err = ListMembers(db)
	if err != nil {
		t.Fatalf("ListMembers after insert: %v", err)
	}

	if len(members) != 1 || members[0].NodeID != "node-aaa" {
		t.Fatalf("after insert: expected 1 member 'node-aaa', got %v", members)
	}

	// Upsert the same node_id with a different state — must update, not duplicate.
	m1updated := m1
	m1updated.State = "updated-state"
	m1updated.LastSeen = time.Now().UTC().Format(time.RFC3339)

	if err := upsertMember(db, m1updated); err != nil {
		t.Fatalf("upsertMember (update/conflict): %v", err)
	}

	members, err = ListMembers(db)
	if err != nil {
		t.Fatalf("ListMembers after upsert update: %v", err)
	}

	if len(members) != 1 || members[0].State != "updated-state" {
		t.Fatalf("after upsert update: expected state 'updated-state', got %v", members)
	}

	// UpdateLastSeen must not error.
	if err := UpdateLastSeen(db, "node-aaa"); err != nil {
		t.Fatalf("UpdateLastSeen: %v", err)
	}

	// RemoveMember must flip state to "removed".
	if err := RemoveMember(db, "node-aaa"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	members, err = ListMembers(db)
	if err != nil {
		t.Fatalf("ListMembers after RemoveMember: %v", err)
	}

	if len(members) != 1 || members[0].State != "removed" {
		t.Fatalf("after RemoveMember: expected state 'removed', got %v", members)
	}
}

func TestMembershipSQLite(t *testing.T) {
	dbProvider = defs.SqliteProvider

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open sqlite: %v", err)
	}

	defer db.Close()

	testMembershipOps(t, db)
}

func TestMembershipPostgres(t *testing.T) {
	connStr := "postgres://postgres@localhost/ego_db3_test?sslmode=disable"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("postgres not available (%v); skipping", err)
	}

	if err := db.Ping(); err != nil {
		t.Skipf("postgres not reachable (%v); skipping", err)
	}

	defer db.Close()

	// Drop and recreate the cluster table for a clean test run.
	db.Exec(`DROP TABLE IF EXISTS cluster`)

	dbProvider = defs.PostgresProvider

	testMembershipOps(t, db)
}
