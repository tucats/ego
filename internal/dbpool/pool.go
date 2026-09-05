// Package dbpool maintains one shared, cached *sql.DB connection pool per
// DSN name, so table/SQL REST handlers reuse real database connections
// across requests instead of opening a brand-new connection pool on every
// request and closing it again at the end. See
// internal/server/tables/database/open.go for the caller.
//
// Concurrent first-time callers for the same not-yet-cached DSN name are
// coalesced so only one of them actually dials the database; a DSN whose
// connection attempt fails is remembered for a short backoff window so a
// burst of requests against an unreachable database fails fast instead of
// each paying a full connect timeout.
//
// Set DBPoolEnabledSetting to false to bypass all of this and restore the
// server's original per-request open/close behavior exactly, as a rollback
// if pooling misbehaves in some deployment.
package dbpool

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// sweepInterval is how often the idle-eviction background goroutine scans
// the cache for DSN pools that have not been used in a while. It is not
// itself configurable -- only how long a pool must be idle before it is
// evicted (DBPoolIdleEvictSetting) is.
const sweepInterval = 60 * time.Second

// entry is one cached pool and the last time it was handed out by Get.
type entry struct {
	db       *sql.DB
	lastUsed time.Time
}

var (
	mu       sync.Mutex
	entries  = map[string]*entry{}
	badUntil = map[string]time.Time{}
	inflight = map[string]chan struct{}{}

	sweeperStarted sync.Once
)

// Get returns the shared *sql.DB for the given DSN name, creating and
// caching one if this is the first request for that name. The scheme and
// connStr are exactly what the caller would otherwise pass to sql.Open --
// scheme selection and connection-string normalization stay the caller's
// responsibility (see database/open.go).
//
// The returned bool is true when the *sql.DB is a shared, cached handle that
// the caller must NOT close itself -- its lifecycle belongs to this package
// (idle eviction, DSN-change eviction, or server shutdown). It is false only
// when pooling is disabled, in which case the caller owns the handle exactly
// as before this package existed and is responsible for closing it.
//
// A non-nil error can mean the DSN has never connected successfully, or that
// it recently failed and is within its retry backoff window (see
// DBPoolRetrySetting) -- either way the caller should treat it exactly like
// any other failure to open the database.
func Get(name, scheme, connStr string) (*sql.DB, bool, error) {
	if !poolingEnabled() {
		db, err := openLegacy(scheme, connStr)

		return db, false, err
	}

	startSweeperOnce()

	for {
		mu.Lock()

		if e, ok := entries[name]; ok {
			e.lastUsed = time.Now()
			db := e.db
			mu.Unlock()

			return db, true, nil
		}

		if until, ok := badUntil[name]; ok {
			if time.Now().Before(until) {
				mu.Unlock()

				return nil, false, errors.ErrDSNPoolUnavailable.Context(name)
			}

			delete(badUntil, name)
		}

		if ch, ok := inflight[name]; ok {
			mu.Unlock()
			<-ch

			continue
		}

		ch := make(chan struct{})
		inflight[name] = ch
		mu.Unlock()

		db, err := createPool(name, scheme, connStr)

		mu.Lock()
		delete(inflight, name)

		if err != nil {
			badUntil[name] = time.Now().Add(retryBackoff())
			mu.Unlock()
			close(ch)

			return nil, false, err
		}

		entries[name] = &entry{db: db, lastUsed: time.Now()}
		mu.Unlock()
		close(ch)

		return db, true, nil
	}
}

// createPool opens a new pool, applies the configured pool limits and any
// provider-specific post-open setup, and performs one bounded connectivity
// check before the pool is cached -- so a DSN that names an unreachable host
// fails fast on first use rather than being cached in a broken state.
func createPool(name, scheme, connStr string) (*sql.DB, error) {
	if scheme == defs.SqliteProvider {
		connStr = sqliteConnString(connStr)
	}

	db, err := sql.Open(scheme, connStr)
	if err != nil {
		return nil, err
	}

	configurePool(db)

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout())
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()

		ui.Log(ui.DBLogger, "db.pool.ping.error", ui.A{
			"dsn":   name,
			"error": err,
		})

		return nil, err
	}

	ui.Log(ui.DBLogger, "db.pool.created", ui.A{
		"dsn": name,
	})

	return db, nil
}

// openLegacy replicates the server's original, pre-caching behavior exactly:
// a brand-new pool with the original hardcoded Postgres limits and no
// connectivity check. It exists solely so DBPoolEnabledSetting=false is a
// true rollback to prior behavior, not merely "caching off but otherwise
// changed".
func openLegacy(scheme, connStr string) (*sql.DB, error) {
	if scheme == defs.SqliteProvider {
		connStr = sqliteConnString(connStr)
	}

	db, err := sql.Open(scheme, connStr)
	if err != nil {
		return nil, err
	}

	if scheme == defs.PostgresProvider {
		db.SetMaxOpenConns(5)
		db.SetConnMaxLifetime(5 * time.Minute)
	}

	return db, nil
}

// sqliteConnString appends the busy_timeout and WAL journal-mode pragmas as
// modernc.org/sqlite DSN query parameters, so the driver applies them to
// every physical connection it opens -- including ones opened later, under
// concurrent load, once a cached pool is actually shared by many requests.
// Running them as a one-time Exec right after sql.Open (this server's
// original approach) only ever reached the single connection that Exec
// happened to grab; every other connection the pool later opened had no
// busy_timeout set at all, surfacing as intermittent "database is locked"
// errors under concurrent access -- exactly the failure mode a shared,
// cached pool makes newly reachable.
func sqliteConnString(connStr string) string {
	const pragmas = "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"

	if strings.Contains(connStr, "?") {
		return connStr + "&" + pragmas
	}

	return connStr + "?" + pragmas
}

// configurePool applies the admin-tunable pool limits to a freshly opened
// pool. These replace the hardcoded Postgres-only limits the server used
// before per-DSN caching existed; they are now applied uniformly since a
// cached pool (SQLite included) lives long enough for them to matter.
func configurePool(db *sql.DB) {
	db.SetMaxOpenConns(settingInt(defs.DBPoolMaxOpenSetting, 10))
	db.SetMaxIdleConns(settingInt(defs.DBPoolMaxIdleSetting, 2))
	db.SetConnMaxLifetime(time.Duration(settingInt(defs.DBPoolMaxLifetimeSetting, 300)) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(settingInt(defs.DBPoolMaxIdleTimeSetting, 60)) * time.Second)
}


// Touch refreshes the last-used time for the cached pool for name, if one
// exists, without opening or returning it. Callers that keep using a
// *sql.DB they obtained from an earlier Get -- most notably a long-lived
// REST transaction, whose requests are dispatched by transaction id and
// never call Get again for the DSN -- must call this periodically (e.g. on
// each request against that transaction, and on each /keepalive) so
// sweepIdle does not mistake an actively used pool for an idle one and
// close it out from under the transaction.
func Touch(name string) {
	mu.Lock()
	defer mu.Unlock()

	if e, ok := entries[name]; ok {
		e.lastUsed = time.Now()
	}
}

// Evict closes and removes the cached pool for name, if one exists, and
// clears any bad-DSN backoff recorded for it. It is safe to call when there
// is no cached entry. Callers should invoke this whenever a DSN definition
// is written or deleted (see internal/dsns/dsn_sqldb.go), so a stale pool
// never keeps serving a DSN name that now points somewhere else.
func Evict(name string) {
	mu.Lock()
	e, ok := entries[name]
	delete(entries, name)
	delete(badUntil, name)
	mu.Unlock()

	if ok {
		ui.Log(ui.DBLogger, "db.pool.evict", ui.A{"dsn": name})
		e.db.Close()
	}
}

// CloseAll closes every cached pool. Intended for graceful server shutdown.
func CloseAll() {
	mu.Lock()
	all := entries
	entries = map[string]*entry{}
	mu.Unlock()

	for name, e := range all {
		ui.Log(ui.DBLogger, "db.pool.evict", ui.A{"dsn": name})
		e.db.Close()
	}
}

// PoolStats reports a cached pool's connection statistics together with the
// last time it was handed out by Get, for admin observability.
type PoolStats struct {
	sql.DBStats
	LastUsed time.Time
}

// Stats returns a snapshot of connection statistics for every currently
// cached pool, keyed by DSN name, for admin observability (see
// /admin/resources and /admin/caches).
func Stats() map[string]PoolStats {
	mu.Lock()
	defer mu.Unlock()

	result := make(map[string]PoolStats, len(entries))
	for name, e := range entries {
		result[name] = PoolStats{DBStats: e.db.Stats(), LastUsed: e.lastUsed}
	}

	return result
}

func startSweeperOnce() {
	sweeperStarted.Do(func() {
		go sweepLoop()
	})
}

func sweepLoop() {
	for {
		time.Sleep(sweepInterval)
		sweepIdle()
	}
}

// sweepIdle closes and evicts any cached pool that has not been used within
// DBPoolIdleEvictSetting seconds, so a DSN that is rarely touched does not
// hold a pool (and its physical connections) open forever.
func sweepIdle() {
	ttl := time.Duration(settingInt(defs.DBPoolIdleEvictSetting, 600)) * time.Second
	cutoff := time.Now().Add(-ttl)

	mu.Lock()

	stale := make(map[string]*entry)

	for name, e := range entries {
		if e.lastUsed.Before(cutoff) {
			stale[name] = e

			delete(entries, name)
		}
	}

	mu.Unlock()

	for name, e := range stale {
		ui.Log(ui.DBLogger, "db.pool.idle.evict", ui.A{"dsn": name})
		e.db.Close()
	}
}

// poolingEnabled reports whether per-DSN pool caching is active. Unset
// defaults to true; only an explicit false disables it.
func poolingEnabled() bool {
	if settings.Get(defs.DBPoolEnabledSetting) == "" {
		return true
	}

	return settings.GetBool(defs.DBPoolEnabledSetting)
}

// settingInt reads an integer setting, falling back to def when the setting
// is absent or not a positive integer.
func settingInt(key string, def int) int {
	if v := settings.GetInt(key); v > 0 {
		return v
	}

	return def
}

func retryBackoff() time.Duration {
	return time.Duration(settingInt(defs.DBPoolRetrySetting, 10)) * time.Second
}

func pingTimeout() time.Duration {
	return time.Duration(settingInt(defs.DBPoolPingTimeoutSetting, 5)) * time.Second
}
