package auth

import (
	"encoding/json"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

type fileService struct {
	path  string
	dirty bool
	lock  sync.Mutex
	data  map[string]defs.User
}

// NewFileService creates a new user service that uses a file-based
// database to store user information. If the userDatabaseFile is
// an empty string then the database is not written to disk, and is only
// maintained in memory.
func NewFileService(userDatabaseFile, defaultUser, defaultPassword string) (userIOService, error) {
	if userDatabaseFile == "memory" {
		userDatabaseFile = ""
	}

	svc := &fileService{
		path: userDatabaseFile,
		data: map[string]defs.User{},
	}

	if userDatabaseFile != "" {
		b, err := os.ReadFile(userDatabaseFile)
		if err == nil {
			// If there is a user data key, decrypt the data before using it.
			// This is used to protect the user data from casual inspection.
			// If there is no key, the data is stored as plaintext JSON in the
			// file.
			if key := settings.Get(defs.LogonUserdataKeySetting); key != "" {
				r, err := util.Decrypt(string(b), key)
				if err != nil {
					return svc, err
				}

				b = []byte(r)
			}

			if len(b) > 0 {
				err = json.Unmarshal(b, &svc.data)
				if err != nil {
					return svc, errors.New(err)
				}
			}

			ui.Log(ui.AuthLogger, "Using file-system credential store with %d items", len(svc.data))
		}
	}

	// Construct the map of user definitions in memory if not already read from
	// the JSON file data.
	if len(svc.data) == 0 {
		svc.data = map[string]defs.User{
			defaultUser: {
				ID:          uuid.New(),
				Name:        defaultUser,
				Password:    HashString(defaultPassword),
				Permissions: []string{"root", "logon"},
			},
		}
		svc.dirty = true

		ui.Log(ui.AuthLogger, "Using default credentials %s:%s", defaultUser, strings.Repeat("*", len(defaultPassword)))
	}

	return svc, nil
}

// ListUsers returns a map of all users in the database.
func (f *fileService) ListUsers() map[string]defs.User {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.data
}

// ReadUser returns a user definition from the database. If the doNotLog
// parameter is true, the operation is not logged to the AUTH audit log.
func (f *fileService) ReadUser(name string, doNotLog bool) (defs.User, error) {
	var err error

	f.lock.Lock()
	defer f.lock.Unlock()

	user, ok := f.data[name]
	if !ok {
		err = errors.ErrNoSuchUser.Context(name)
	}

	return user, err
}

// WriteUser adds or updates a user definition in the database. If the user
// already exists, it is updated. If the user does not exist, it is added.
// The map is marked as dirty so it will be written to disk.
func (f *fileService) WriteUser(user defs.User) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	_, found := f.data[user.Name]
	f.data[user.Name] = user
	f.dirty = true

	if found {
		ui.Log(ui.AuthLogger, "Updated user %s", user.Name)
	} else {
		ui.Log(ui.AuthLogger, "Created user %s", user.Name)
	}

	return nil
}

// DeleteUser removes a user definition from the database. The map
// is marked as dirty so it will be written to disk.
func (f *fileService) DeleteUser(name string) error {
	u, err := f.ReadUser(name, false)
	if err == nil {
		f.lock.Lock()
		defer f.lock.Unlock()

		delete(f.data, u.Name)
		f.dirty = true

		ui.Log(ui.AuthLogger, "Deleted user %s", u.Name)
	}

	return nil
}

// Flush writes the file-based data to a json file. This operation is not
// done if there were no changes to the database, or there is not a database
// file name given.
func (f *fileService) Flush() error {
	f.lock.Lock()
	defer f.lock.Unlock()

	if !f.dirty || f.path == "" {
		return nil
	}

	// Convert the database to a json string
	b, err := json.MarshalIndent(f.data, "", "   ")
	if err != nil {
		return errors.New(err)
	}

	// If there is a user data encryption key, encrypt the data before
	// writing it. This is used to protect the user data from casual
	// inspection. If there is no key, the data is stored as plaintext
	// JSON in the file.
	if key := settings.Get(defs.LogonUserdataKeySetting); key != "" {
		r, err := util.Encrypt(string(b), key)
		if err != nil {
			return err
		}

		b = []byte(r)
	}

	// Write to the database file. The file is created with 0600 permissions
	// so that it is only readable by the owner.
	err = os.WriteFile(userDatabaseFile, b, 0600)
	if err == nil {
		f.dirty = false

		ui.Log(ui.AuthLogger, "Rewrote file-system credential store")
	} else {
		err = errors.New(err)
	}

	return err
}
