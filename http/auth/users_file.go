package auth

import (
	"encoding/json"
	"io/ioutil"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

type FileService struct {
	path  string
	dirty bool
	data  map[string]defs.User
}

func NewFileService(userDatabaseFile, defaultUser, defaultPassword string) (UserIOService, error) {
	svc := &FileService{
		path: userDatabaseFile,
		data: map[string]defs.User{},
	}

	if userDatabaseFile != "" {
		b, err := ioutil.ReadFile(userDatabaseFile)
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

			ui.Debug(ui.AuthLogger, "Using file-system credential store with %d items", len(svc.data))
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

		ui.Debug(ui.AuthLogger, "Using default credentials %s:%s", defaultUser, defaultPassword)
	}

	return svc, nil
}

func (f *FileService) ListUsers() map[string]defs.User {
	return f.data
}

func (f *FileService) ReadUser(name string, doNotLog bool) (defs.User, error) {
	var err error

	user, ok := f.data[name]
	if !ok {
		err = errors.ErrNoSuchUser.Context(name)
	}

	return user, err
}

func (f *FileService) WriteUser(user defs.User) error {
	_, found := f.data[user.Name]
	f.data[user.Name] = user
	f.dirty = true

	if found {
		ui.Debug(ui.AuthLogger, "Updated user %s", user.Name)
	} else {
		ui.Debug(ui.AuthLogger, "Created user %s", user.Name)
	}

	return nil
}

func (f *FileService) DeleteUser(name string) error {
	u, err := f.ReadUser(name, false)
	if err == nil {
		delete(f.data, u.Name)
		f.dirty = true

		ui.Debug(ui.AuthLogger, "Deleted user %s", u.Name)
	}

	return nil
}

func (f *FileService) Flush() error {
	if !f.dirty {
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
	err = ioutil.WriteFile(userDatabaseFile, b, 0600)
	if err == nil {
		f.dirty = false

		ui.Debug(ui.AuthLogger, "Rewrote file-system credential store")
	} else {
		err = errors.NewError(err)
	}

	return err
}
