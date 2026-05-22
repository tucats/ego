// Package cluster manages Ego server clustering. When multiple Ego server
// instances are started with the --cluster flag pointing at the same shared
// system database, they form a named cluster. Each node registers itself in
// a "cluster" table, periodically pings its peers to confirm they are alive,
// and broadcasts cache-invalidation notices whenever shared state changes
// (user records, DSN definitions, permissions, etc.).
//
// A server that is not started with --cluster operates in standalone mode and
// ignores all cluster logic. No configuration change is needed for standalone
// use — just omit the flag.
//
// Typical startup sequence (called from commands/server.go):
//
//	cluster.Initialize(c, systemDB)   // register this node; starts no goroutines
//	go cluster.StartHealthChecker(db) // begin peer pinging in background
//
// Typical shutdown (called from the SIGINT handler and any clean exit path):
//
//	cluster.Shutdown(db)              // mark this node "removed" in cluster table
package cluster

import (
	"database/sql"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/defs"
	_ "modernc.org/sqlite"
)

// ClusterName is the name of the cluster this node has joined. It is set by
// Initialize() from the --cluster flag. An empty string means standalone mode.
var ClusterName string

// NodeID is the stable UUID for this server process. It matches
// defs.InstanceID and is used as the primary key in the cluster table.
var NodeID string

// systemDB holds the shared system database handle so that helpers like
// Shutdown() can call it without needing it passed at every call site.
var systemDB *sql.DB

// ThisMember is populated by Initialize() and describes this node's own
// cluster row. Other parts of the package use it when sending HTTP requests
// to peers so that the sender identity is available without a database round-trip.
var ThisMember defs.ClusterMember

// Initialize registers this node in the cluster system table and sets the
// package-level ClusterName and NodeID variables. It is a no-op when the
// --cluster flag is absent, making it safe to call unconditionally from
// RunServer regardless of whether the operator intends to run a cluster.
//
// The cluster package opens its own database connection to the same system
// database that stores users, DSNs, and server start logs. The connection
// string is derived using the same logic as the auth package: the --users
// CLI option, then ego.server.userdata, then the default ego-system.db path.
func Initialize(c *cli.Context) error {
	clusterName, found := c.String("cluster")
	if !found || clusterName == "" {
		// No --cluster flag; run in standalone mode.
		return nil
	}

	ClusterName = clusterName
	NodeID = defs.InstanceID

	db, err := openSystemDB(c)
	if err != nil {
		return err
	}

	systemDB = db

	// Ensure the cluster membership table exists.
	if err := createClusterTable(db); err != nil {
		return err
	}

	// Determine the listening address for this node so peers can reach it.
	host := nodeHostname()
	port := nodePort(c)
	scheme := "https"

	if settings.GetBool(defs.InsecureServerSetting) || c.Boolean("not-secure") {
		scheme = "http"
	}

	ThisMember = defs.ClusterMember{
		Name:     ClusterName,
		NodeID:   NodeID,
		Host:     host,
		Port:     port,
		Scheme:   scheme,
		JoinedAt: time.Now().UTC().Format(time.RFC3339),
		LastSeen: time.Now().UTC().Format(time.RFC3339),
		State:    "active",
	}

	if err := upsertMember(db, ThisMember); err != nil {
		return err
	}

	// Register the cache-invalidation hook so that every local caches.Purge
	// call automatically broadcasts the flush to cluster peers. This is done
	// here rather than at import time so the hook is only active when the
	// server is actually running in cluster mode.
	caches.OnPurge = BroadcastCacheFlush

	ui.Log(ui.ServerLogger, "cluster.join", ui.A{
		"id":   NodeID,
		"name": ClusterName,
		"host": host,
		"port": port,
	})

	return nil
}

// Shutdown marks this node as "removed" in the cluster table. It should be
// called from every clean exit path (SIGINT handler, admin shutdown endpoint,
// os.Exit wrappers). If the node crashes without calling Shutdown, peers will
// detect the absence via health checking and evict it automatically after 90 s.
func Shutdown() {
	if ClusterName == "" || systemDB == nil {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := systemDB.Exec(
		`UPDATE cluster SET state = 'removed', last_seen = ? WHERE node_id = ?`,
		now, NodeID,
	)
	if err != nil {
		ui.Log(ui.ServerLogger, "cluster.shutdown.error", ui.A{
			"id":    NodeID,
			"error": err.Error(),
		})

		return
	}

	ui.Log(ui.ServerLogger, "cluster.leave", ui.A{
		"id":   NodeID,
		"name": ClusterName,
	})
}

// openSystemDB opens a connection to the Ego system database using the same
// path-resolution logic that the auth package uses. The precedence is:
//
//  1. --users CLI flag
//  2. ego.server.userdata configuration setting
//  3. Default: sqlite://<ego.runtime.path>/ego-system.db
//
// WAL mode and a busy-timeout are applied immediately after opening so that
// the cluster table operations can coexist with the auth and DSN writers.
func openSystemDB(c *cli.Context) (*sql.DB, error) {
	connStr, found := c.String("users")
	if !found || connStr == "" {
		connStr = settings.Get(defs.LogonUserdataSetting)
	}

	if connStr == "" {
		authPath := settings.Get(defs.EgoPathSetting)
		connStr = defs.DefaultUserdataScheme + "://" + filepath.Join(authPath, defs.DefaultUserdataFileName)
	}

	// Normalize the scheme. Both "sqlite3" and "sqlite" are accepted; the
	// modernc.org/sqlite driver registers under the name "sqlite".
	scheme := "sqlite"
	dbPath := connStr

	if strings.HasPrefix(connStr, "sqlite3://") {
		dbPath = strings.TrimPrefix(connStr, "sqlite3://")
	} else if strings.HasPrefix(connStr, "sqlite://") {
		dbPath = strings.TrimPrefix(connStr, "sqlite://")
	} else if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		scheme = defs.PostgresProvider
		dbPath = connStr
	}

	db, err := sql.Open(scheme, dbPath)
	if err != nil {
		return nil, err
	}

	if scheme == "sqlite" {
		db.Exec("PRAGMA journal_mode=WAL;")
		db.Exec("PRAGMA busy_timeout=5000;")

		// Verify the connection is actually usable. Stale WAL/SHM files left
		// behind when server processes are killed can cause SQLITE_CANTOPEN (14)
		// on the next open. Remove them and retry once if the ping fails.
		if pingErr := db.Ping(); pingErr != nil {
			db.Close()

			removed := false

			for _, suffix := range []string{"-wal", "-shm"} {
				p := dbPath + suffix

				if _, statErr := os.Stat(p); statErr == nil {
					if os.Remove(p) == nil {
						removed = true
					}
				}
			}

			if !removed {
				return nil, pingErr
			}

			db, err = sql.Open(scheme, dbPath)
			if err != nil {
				return nil, err
			}

			db.Exec("PRAGMA journal_mode=WAL;")
			db.Exec("PRAGMA busy_timeout=5000;")
		}
	}

	return db, nil
}

// createClusterTable creates the cluster membership table in the system
// database if it does not already exist. The table stores one row per known
// node, keyed on node_id. State transitions ('active' → 'removed') are written
// in-place so the history is not preserved — this table is an operational view,
// not an audit log.
func createClusterTable(db *sql.DB) error {
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

	return err
}

// nodeHostname returns the hostname for this node that peers can use to
// connect to it. It prefers the fully-qualified hostname from os.Hostname;
// if that fails it falls back to the first non-loopback IP address it can find.
func nodeHostname() string {
	if host, err := os.Hostname(); err == nil {
		// @TOMCOLE not sure if this is right. If the name has no dots,
		// it is probably not fully qualified and may not be resolvable to
		// match the certificate. So let's add ".local" to the end of it,
		// which is a common convention for local hostnames.
		if !strings.Contains(host, ".") {
			host += ".local"
		}

		return host
	}

	// Fallback: find the first non-loopback address.
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ipNet.IP.To4() != nil {
					return ipNet.IP.String()
				}
			}
		}
	}

	return "localhost"
}

// nodePort returns the port this server is listening on by reading the --port
// CLI option or falling back to the default (443 for HTTPS, 80 for HTTP).
func nodePort(c *cli.Context) int {
	if p, ok := c.Integer("port"); ok {
		return p
	}

	if s := os.Getenv(defs.EgoPortEnv); s != "" {
		if p, err := strconv.Atoi(s); err == nil {
			return p
		}
	}

	if settings.GetBool(defs.InsecureServerSetting) || c.Boolean("not-secure") {
		return 80
	}

	return 443
}
