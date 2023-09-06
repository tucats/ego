package auth

import (
	"encoding/json"
	"os"
	"strings"

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
	data  map[string]defs.User
}

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
			if key := settings.Get(defs.LogonUserdataKeySetting); key != "" {
				r, err := util.Decrypt(string(b), key)
				if err != nil {
					return svc, err
				}

				b = []byte(r)
			}

			if err == nil {
				err = json.Unmarshal(b, &svc.data)
			}

			if err != nil {
				return svc, errors.NewError(err)
			}

			ui.Log(ui.AuthLogger, "Using file-system credential store with %d items", len(svc.data))
		}
	}

	if svc.data == nil || len(svc.data) == 0 {
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

func (f *fileService) ListUsers() map[string]defs.User {
	return f.data
}

func (f *fileService) ReadUser(name string, doNotLog bool) (defs.User, error) {
	var err error

	user, ok := f.data[name]
	if !ok {
		err = errors.ErrNoSuchUser.Context(name)
	}

	return user, err
}

func (f *fileService) WriteUser(user defs.User) error {
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

func (f *fileService) DeleteUser(name string) error {
	u, err := f.ReadUser(name, false)
	if err == nil {
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
	if !f.dirty || f.path == "" {
		return nil
	}

	// Convert the database to a json string
	b, err := json.MarshalIndent(f.data, "", "   ")
	if err != nil {
		return errors.NewError(err)
	}

	if key := settings.Get(defs.LogonUserdataKeySetting); key != "" {
		r, err := util.Encrypt(string(b), key)
		if err != nil {
			return err
		}

		b = []byte(r)
	}

	// Write to the database file.
	err = os.WriteFile(userDatabaseFile, b, 0600)
	if err == nil {
		f.dirty = false

		ui.Log(ui.AuthLogger, "Rewrote file-system credential store")
	} else {
		err = errors.NewError(err)
	}

	return err
}
