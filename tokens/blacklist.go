package tokens

import (
	"sync"
	"time"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/resources"
)

// The connection string to the credentials database used to hold blacklisted token information.
// If this is an empty string, the blacklisting is not enabled. This is also the case when the
// credentials database is being managed in-memory.
var connectionString string

// The handle to the resource object in the credentials database used for token blacklisting.
var handle *resources.ResHandle

// The mutex used to synchronize creation of resources for the blacklist resource.
var mutex sync.Mutex

// SetDatabasePath sets the path to the database file used for token blacklisting. IF the
// database path is not provided, blacklisting is disabled. If a path is given, establish
// a resource handle for the blacklist object in the credentials database.
func SetDatabasePath(path string) error {
	var err error

	mutex.Lock()
	defer mutex.Unlock()

	connectionString = path

	// If no database in use (using the in-memory authenticator), do nothing.
	if path == "" {
		return nil
	}

	// We need to establish our own resource object in this database.

	// Use the resources manager to open the database connection.
	handle, err = resources.Open(BackListItem{}, "blacklist", connectionString)
	if err != nil {
		return errors.New(err)
	}

	// Create the underlying database table definition if it does not yet exist.
	if err = handle.CreateIf(); err != nil {
		ui.Log(ui.ServerLogger, "server.db.error", ui.A{
			"error": err})

		return errors.New(err)
	}

	return nil
}

func Blacklist(id string) error {
	mutex.Lock()
	defer mutex.Unlock()

	if handle == nil {
		return nil
	}

	item := &BackListItem{
		ID:      id,
		Active:  true,
		Created: time.Now().Format(time.RFC822Z),
	}

	err := handle.Insert(item)
	if err != nil {
		return errors.New(err)
	}

	return nil
}

// IsBlacklisted checks to see if the given token ID is blacklisted. If blacklisting is
// not enabled, this always returns false. Otherwise, it checks the blacklist resource
// for an active entry for the given token ID. If found, the ID is considered blacklisted.
func IsBlacklisted(t Token) (bool, error) {
	mutex.Lock()
	defer mutex.Unlock()

	if handle == nil {
		return false, nil
	}

	var item *BackListItem

	items, err := handle.Read(handle.Equals("id", t.TokenID.String()))
	if err != nil {
		if errors.Equal(err, errors.ErrNotFound) {
			return false, nil
		}

		return false, errors.New(err)
	}

	for _, i := range items {
		item = i.(*BackListItem)
		if item.Active {
			// We need to update the information on the blacklist resource.
			item.Last = time.Now().Format(time.RFC822Z)
			item.Created = t.Created.Format(time.RFC822Z)
			item.User = t.Name

			err = handle.Update(item, handle.Equals("id", t.TokenID.String()))

			return true, err
		}
	}

	return false, nil
}
