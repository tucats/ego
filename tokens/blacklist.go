package tokens

import (
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/resources"
)

// connectionString holds the file-system path to the SQLite credentials
// database. An empty string means blacklisting is disabled: all blacklist
// operations become no-ops and always report "not blacklisted".
var connectionString string

// handle is the open database handle used for all blacklist reads and writes.
// It is nil until SetDatabasePath is called with a non-empty path.
// Every exported function in this file checks for nil before touching the
// database, so the blacklist is safely bypassed when no path is configured.
var handle *resources.ResHandle

// mutex serializes all blacklist database operations. The blacklist functions
// are called concurrently from multiple request-handling goroutines, so every
// exported function acquires this lock before reading or writing.
var mutex sync.Mutex

// useCache controls whether blacklist lookups are served from an in-memory
// cache before falling back to the database. Caching is enabled by default and
// dramatically reduces database load when many requests carry the same token.
var useCache bool = true

// SetDatabasePath configures the SQLite database that stores blacklisted token
// IDs. Call this once during server startup, before handling any requests.
//
// If path is an empty string, blacklisting is disabled for the lifetime of the
// process: Blacklist(), Delete(), IsBlacklisted(), Flush(), and List() all
// become no-ops that return nil / zero / empty slice.
//
// If path is non-empty, SetDatabasePath opens the database and creates the
// "blacklist" table if it does not yet exist. An error is returned if the
// database cannot be opened or the table cannot be created.
//
// Important: calling SetDatabasePath("") after a non-empty path has been set
// only resets the stored connection string; the existing database handle
// remains open. To truly disable blacklisting once it has been enabled, the
// server must be restarted.
func SetDatabasePath(path string) error {
	var err error

	mutex.Lock()
	defer mutex.Unlock()

	connectionString = path

	// If no database path was provided, silently disable blacklisting.
	if path == "" {
		return nil
	}

	// Open (or reopen) the credentials database. The resources package manages
	// the underlying SQLite connection.
	handle, err = resources.Open(BlackListItem{}, "blacklist", connectionString)
	if err != nil {
		return errors.New(err)
	}

	// Create the "blacklist" table if it does not already exist.
	if err = handle.CreateIf(); err != nil {
		ui.Log(ui.ServerLogger, "server.db.error", ui.A{
			"error": err})

		return errors.New(err)
	}

	return nil
}

// Close terminates resources associated with token blacklist testing. IF the
// process needs to resume blacklist testing, then a new call to SetDatabasePath()
// must be called to reconnect to the database that stores the blacklist.
func Close() {
	handle = nil
}

// Blacklist adds the given token ID to the revocation list. After this call,
// any future use of a token with that ID will be rejected by Validate() and
// Unwrap().
//
// If no database path has been configured (handle is nil), this function
// returns nil without doing anything — blacklisting is silently disabled.
//
// On success the in-memory caches (blacklist, auth, token) are purged so that
// subsequent requests re-validate against the updated database rather than
// serving stale cache entries.
func Blacklist(id string) error {
	mutex.Lock()
	defer mutex.Unlock()

	// Blacklisting is disabled — nothing to do.
	if handle == nil {
		return nil
	}

	item := &BlackListItem{
		ID:      id,
		Active:  true,
		Created: time.Now().Format(time.RFC822Z),
	}

	err := handle.Insert(item)
	if err != nil {
		return errors.New(err)
	}

	// Purge all related caches so that the newly-blacklisted token is rejected
	// on the very next request rather than being served from a cache.
	if useCache {
		caches.Purge(caches.BlacklistCache)
	}

	caches.Purge(caches.AuthCache)
	caches.Purge(caches.TokenCache)

	return nil
}

// Delete removes the given token ID from the blacklist, effectively un-revoking
// it. After this call the token can be used again (if it has not yet expired).
//
// Returns ErrNotFound when the ID is not present in the blacklist, including
// when no database has been configured (because the entry cannot exist).
//
// On success the cache entry for this ID is deleted so that subsequent
// IsBlacklisted calls re-query the database.
func Delete(id string) error {
	mutex.Lock()
	defer mutex.Unlock()

	if handle == nil {
		return errors.ErrNotFound.Clone().Context(id)
	}

	// Delete the row whose "id" column matches the supplied string.
	count, err := handle.Delete(handle.Equals("id", id))
	if err != nil {
		return errors.New(err)
	}

	// count == 0 means the ID was not in the database; report it as not found.
	if count == 0 {
		return errors.ErrNotFound.Clone().Context(id)
	}

	// Remove the stale cache entry so future IsBlacklisted calls go to the DB.
	if useCache {
		caches.Delete(caches.BlacklistCache, id)
	}

	return nil
}

// IsBlacklisted reports whether the token t has been revoked. It returns
// (true, nil) if the token's ID appears in the blacklist with Active == true,
// and (false, nil) if it does not. A non-nil error indicates a database problem.
//
// If no database has been configured (handle is nil), IsBlacklisted always
// returns (false, nil) — every token is treated as not revoked.
//
// Caching: to avoid a database round-trip on every request, results are stored
// in BlacklistCache. A cache hit (positive or negative) is returned immediately.
// On a cache miss the database is queried, and the result is written back to the
// cache for future callers. Cache entries are invalidated by Blacklist() and
// Delete().
//
// Side effect: when an active blacklist entry is found, IsBlacklisted updates
// the entry's Last, Created, and User fields to reflect the most recent use
// attempt, giving administrators an audit trail.
func IsBlacklisted(t Token) (bool, error) {
	mutex.Lock()
	defer mutex.Unlock()

	if handle == nil {
		return false, nil
	}

	var item *BlackListItem

	// Check the in-memory cache first to avoid a database round-trip.
	if useCache {
		v, found := caches.Find(caches.BlacklistCache, t.TokenID.String())
		if found {
			item = v.(*BlackListItem)

			return item.Active, nil
		}
	}

	// Cache miss — query the database for a matching ID.
	items, err := handle.Read(handle.Equals("id", t.TokenID.String()))
	if err != nil {
		if errors.Equal(err, errors.ErrNotFound) {
			// The token is not in the blacklist. Cache a "not active" sentinel so
			// the next lookup for this ID does not hit the database again.
			if useCache {
				caches.Add(caches.BlacklistCache, t.TokenID.String(), &BlackListItem{
					ID:     t.TokenID.String(),
					Active: false,
				})
			}

			return false, nil
		}

		return false, errors.New(err)
	}

	// Scan the results for any active entry.
	for _, i := range items {
		item = i.(*BlackListItem)
		if item.Active {
			// Update the blacklist record to show who tried to use it and when.
			item.Last = time.Now().Format(time.RFC822Z)
			item.Created = t.Created.Format(time.RFC822Z)
			item.User = t.Name

			err = handle.Update(item, handle.Equals("id", t.TokenID.String()))

			// Cache the updated item so the next call is served from memory.
			if useCache {
				caches.Add(caches.BlacklistCache, t.TokenID.String(), item)
			}

			return true, err
		}
	}

	// No active entry found — cache a negative result.
	if useCache {
		caches.Add(caches.BlacklistCache, t.TokenID.String(), &BlackListItem{
			ID:     t.TokenID.String(),
			Active: false,
		})
	}

	return false, nil
}

// Flush deletes every entry from the blacklist database table and clears the
// in-memory cache. It returns the number of rows deleted and any error.
//
// If no database has been configured, Flush returns (0, nil).
//
// Note: Flush removes all entries regardless of whether they are active,
// inactive, or expired. It is a complete wipe of the revocation list.
func Flush() (int, error) {
	mutex.Lock()
	defer mutex.Unlock()

	if handle == nil {
		return 0, nil
	}

	// Clear the cache before hitting the database so a concurrent request
	// cannot read a now-deleted entry from the cache.
	if useCache {
		caches.Purge(caches.BlacklistCache)
	}

	// Passing nil to Delete means "no filter" — delete all rows.
	count, err := handle.Delete(nil)

	return int(count), err
}

// List returns every entry in the blacklist as a slice of BlackListItem values.
// The slice is empty (not nil) when the blacklist is empty or when no database
// has been configured.
//
// An error is returned only if the database read fails.
func List() ([]BlackListItem, error) {
	mutex.Lock()
	defer mutex.Unlock()

	result := []BlackListItem{}

	if handle == nil {
		return result, nil
	}

	// Passing nil to Read means "no filter" — return all rows.
	items, err := handle.Read(nil)
	if err != nil {
		return nil, errors.New(err)
	}

	for _, i := range items {
		item := i.(*BlackListItem)

		result = append(result, *item)
	}

	return result, nil
}
