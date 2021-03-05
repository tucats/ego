package server

import (
	"encoding/json"
	"io/ioutil"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/persistence"
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

func NewFileService(userDatabaseFile, defaultUser, defaultPassword string) (UserIOService, *errors.EgoError) {
	svc := &FileService{
		path: userDatabaseFile,
		data: map[string]defs.User{},
	}

	if userDatabaseFile != "" {
		b, err := ioutil.ReadFile(userDatabaseFile)
		if errors.Nil(err) {
			if key := persistence.Get(defs.LogonUserdataKeySetting); key != "" {
				r, err := util.Decrypt(string(b), key)
				if !errors.Nil(err) {
					return svc, err
				}

				b = []byte(r)
			}

			if errors.Nil(err) {
				err = json.Unmarshal(b, &svc.data)
			}

			if !errors.Nil(err) {
				return svc, errors.New(err)
			}

			ui.Debug(ui.ServerLogger, "Using file-system credential store with %d items", len(svc.data))
		}
	}

	if svc.data == nil {
		svc.data = map[string]defs.User{
			defaultUser: {
				ID:          uuid.New(),
				Name:        defaultUser,
				Password:    HashString(defaultPassword),
				Permissions: []string{"root"},
			},
		}
		svc.dirty = true

		ui.Debug(ui.ServerLogger, "Using default credentials %s:%s", defaultUser, defaultPassword)
	}

	return svc, nil
}

func (f *FileService) ListUsers() map[string]defs.User {
	return f.data
}

func (f *FileService) ReadUser(name string) (defs.User, *errors.EgoError) {
	var err *errors.EgoError

	user, ok := f.data[name]
	if !ok {
		err = errors.New(errors.NoSuchUserError).Context(name)
	}

	return user, err
}

func (f *FileService) WriteUser(user defs.User) *errors.EgoError {
	_, found := f.data[user.Name]
	f.data[user.Name] = user
	f.dirty = true

	if found {
		ui.Debug(ui.ServerLogger, "Updated user %s", user.Name)
	} else {
		ui.Debug(ui.ServerLogger, "Created user %s", user.Name)
	}

	return nil
}

func (f *FileService) DeleteUser(name string) *errors.EgoError {
	u, err := f.ReadUser(name)
	if errors.Nil(err) {
		delete(f.data, u.Name)
		f.dirty = true

		ui.Debug(ui.ServerLogger, "Deleted user %s", u.Name)
	}

	return nil
}

func (f *FileService) Flush() *errors.EgoError {
	if !f.dirty {
		return nil
	}
	// Convert the database to a json string
	b, err := json.MarshalIndent(f.data, "", "   ")
	if !errors.Nil(err) {
		return errors.New(err)
	}

	if key := persistence.Get(defs.LogonUserdataKeySetting); key != "" {
		r, err := util.Encrypt(string(b), key)
		if !errors.Nil(err) {
			return err
		}

		b = []byte(r)
	}

	// Write to the database file.
	err = ioutil.WriteFile(userDatabaseFile, b, 0600)
	if err == nil {
		f.dirty = false

		ui.Debug(ui.ServerLogger, "Rewrote file-system credential store")
	}

	return errors.New(err)
}
