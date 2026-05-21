package resources

import "database/sql"

// applyWriterPragmas configures a freshly opened SQLite database connection for
// concurrent multi-writer use. It must be called immediately after sql.Open so
// that the settings take effect before any queries run.
//
// Two pragmas are applied:
//
//  1. WAL journal mode — Write-Ahead Logging lets readers proceed without
//     blocking writers and vice versa. Without WAL, SQLite uses a rollback
//     journal that requires an exclusive lock for writes, making concurrent
//     access from multiple goroutines or processes very slow.
//
//  2. busy_timeout — When a write cannot acquire the lock immediately (another
//     writer is active), SQLite normally returns SQLITE_BUSY right away. Setting
//     a busy timeout of 5 000 ms tells the driver to retry internally for up to
//     five seconds before surfacing the error, which smooths over the brief lock
//     contention typical in a lightly loaded cluster.
func applyWriterPragmas(db *sql.DB) {
	// One writer at a time is all SQLite supports, but WAL lets concurrent
	// readers see a stable snapshot while a write is in progress.
	db.Exec("PRAGMA journal_mode=WAL;")

	// Retry automatically for up to 5 seconds instead of failing instantly
	// when another process holds the write lock.
	db.Exec("PRAGMA busy_timeout=5000;")
}
