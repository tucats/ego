package dbpool

import (
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"

	_ "modernc.org/sqlite"
)

func tempSqliteDSN(t *testing.T) string {
	t.Helper()

	return filepath.Join(t.TempDir(), "test.db")
}

func TestGetCachesHandlePerName(t *testing.T) {
	name := "pool-test-cache"
	path := tempSqliteDSN(t)
	t.Cleanup(func() { Evict(name) })

	db1, pooled1, err := Get(name, defs.SqliteProvider, path)
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}

	if !pooled1 {
		t.Fatalf("expected a pooled handle")
	}

	db2, pooled2, err := Get(name, defs.SqliteProvider, path)
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}

	if !pooled2 {
		t.Fatalf("expected a pooled handle")
	}

	if db1 != db2 {
		t.Fatalf("expected the second Get to return the same cached *sql.DB")
	}
}

// TestGetConcurrentCallersShareOneHandle guards the singleflight coalescing:
// many concurrent first-time callers for the same uncached DSN name must all
// end up with the identical *sql.DB, not each open (and leak) their own.
func TestGetConcurrentCallersShareOneHandle(t *testing.T) {
	name := "pool-test-concurrent"
	path := tempSqliteDSN(t)
	t.Cleanup(func() { Evict(name) })

	const n = 20

	results := make([]*sql.DB, n)
	errs := make([]error, n)

	var wg sync.WaitGroup

	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()

			db, _, err := Get(name, defs.SqliteProvider, path)
			results[i] = db
			errs[i] = err
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Get[%d]: %v", i, err)
		}
	}

	for i := 1; i < n; i++ {
		if results[i] != results[0] {
			t.Fatalf("expected every concurrent Get to return the same handle")
		}
	}
}

// TestEvictClosesAndRemovesHandle guards the invalidation hook that
// internal/dsns calls on DSN write/delete: eviction must close the shared
// handle (so it doesn't leak) and remove it from the cache (so the next Get
// creates a fresh one instead of reusing the closed one).
func TestEvictClosesAndRemovesHandle(t *testing.T) {
	name := "pool-test-evict"
	path := tempSqliteDSN(t)

	db1, _, err := Get(name, defs.SqliteProvider, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	Evict(name)

	if err := db1.Ping(); err == nil {
		t.Fatalf("expected the evicted handle to be closed")
	}

	db2, _, err := Get(name, defs.SqliteProvider, path)
	if err != nil {
		t.Fatalf("Get after evict: %v", err)
	}

	t.Cleanup(func() { Evict(name) })

	if db1 == db2 {
		t.Fatalf("expected a fresh handle after eviction, got the closed one back")
	}
}

// TestGetBadDSNBacksOffThenRetries guards the graceful-bad-DSN requirement:
// the first Get against an unreachable database should fail with the real
// connect error, and a second Get arriving within the retry backoff window
// should fail fast with ErrDSNPoolUnavailable instead of paying another full
// connect attempt.
func TestGetBadDSNBacksOffThenRetries(t *testing.T) {
	name := "pool-test-baddsn"
	badPath := filepath.Join(t.TempDir(), "no-such-directory", "test.db")

	original := settings.Get(defs.DBPoolRetrySetting)
	settings.Set(defs.DBPoolRetrySetting, "60")

	t.Cleanup(func() { settings.Set(defs.DBPoolRetrySetting, original) })

	// Evict clears both a cached handle and any bad-DSN backoff state, so a
	// repeat run of this test (e.g. "go test -count=N") starts from a clean
	// slate instead of inheriting the previous run's still-active backoff
	// window for this same name.
	Evict(name)
	t.Cleanup(func() { Evict(name) })

	_, _, err := Get(name, defs.SqliteProvider, badPath)
	if err == nil {
		t.Fatalf("expected the first Get against an unreachable DSN to fail")
	}

	if errors.Equal(err, errors.ErrDSNPoolUnavailable) {
		t.Fatalf("expected the first failure to be the underlying connect error, not the backoff error")
	}

	_, _, err = Get(name, defs.SqliteProvider, badPath)
	if !errors.Equal(err, errors.ErrDSNPoolUnavailable) {
		t.Fatalf("expected a Get within the backoff window to return ErrDSNPoolUnavailable, got %v", err)
	}
}

// TestGetPoolingDisabledBypassesCache guards the escape hatch: with pooling
// disabled, Get must behave like a plain sql.Open passthrough -- a fresh,
// unshared handle on every call, exactly like the server's original
// per-request behavior.
func TestGetPoolingDisabledBypassesCache(t *testing.T) {
	name := "pool-test-disabled"
	path := tempSqliteDSN(t)

	original := settings.Get(defs.DBPoolEnabledSetting)
	settings.Set(defs.DBPoolEnabledSetting, "false")

	t.Cleanup(func() { settings.Set(defs.DBPoolEnabledSetting, original) })

	db1, pooled1, err := Get(name, defs.SqliteProvider, path)
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}

	if pooled1 {
		t.Fatalf("expected a non-pooled handle when pooling is disabled")
	}

	defer db1.Close()

	db2, pooled2, err := Get(name, defs.SqliteProvider, path)
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}

	if pooled2 {
		t.Fatalf("expected a non-pooled handle when pooling is disabled")
	}

	defer db2.Close()

	if db1 == db2 {
		t.Fatalf("expected distinct handles when pooling is disabled")
	}
}

func TestStatsReportsCachedEntries(t *testing.T) {
	name := "pool-test-stats"
	path := tempSqliteDSN(t)
	t.Cleanup(func() { Evict(name) })

	if _, _, err := Get(name, defs.SqliteProvider, path); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if _, ok := Stats()[name]; !ok {
		t.Fatalf("expected Stats() to report an entry for %q", name)
	}
}

func TestCloseAllClosesEveryCachedHandle(t *testing.T) {
	name := "pool-test-closeall"
	path := tempSqliteDSN(t)

	db, _, err := Get(name, defs.SqliteProvider, path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	CloseAll()

	if err := db.Ping(); err == nil {
		t.Fatalf("expected the handle to be closed after CloseAll")
	}
}
