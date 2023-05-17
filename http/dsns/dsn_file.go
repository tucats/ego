package dsns

import (
	"encoding/json"
	"io/ioutil"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

type fileService struct {
	path  string
	dirty bool
	data  map[string]defs.DSN
	auth  map[string]DSNAction
}

func NewFileService(userDatabaseFile string) (dsnService, error) {
	if userDatabaseFile == "memory" {
		userDatabaseFile = ""
	}

	svc := &fileService{
		path: userDatabaseFile,
		data: map[string]defs.DSN{},
		auth: map[string]DSNAction{},
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

			ui.Log(ui.AuthLogger, "Using file-system credential store with %d items", len(svc.data))
		}
	}

	if svc.data == nil || len(svc.data) == 0 {
		svc.data = map[string]defs.DSN{}
		svc.dirty = true

		ui.Log(ui.AuthLogger, "Creating new empty DSN table in memory")
	}

	return svc, nil
}

func (f *fileService) ListDSNS(user string) (map[string]defs.DSN, error) {
	return f.data, nil
}

func (f *fileService) ReadDSN(user, name string, doNotLog bool) (defs.DSN, error) {
	var err error

	dsn, ok := f.data[name]
	if !ok {
		err = errors.ErrNoSuchUser.Context(name)
	}

	return dsn, err
}

func (f *fileService) WriteDSN(user string, dsn defs.DSN) error {
	_, found := f.data[dsn.Name]
	f.data[dsn.Name] = dsn
	f.dirty = true

	if found {
		ui.Log(ui.AuthLogger, "Updated dsn %s", dsn.Name)
	} else {
		ui.Log(ui.AuthLogger, "Created dsn %s", dsn.Name)
	}

	return nil
}

func (f *fileService) DeleteDSN(user, name string) error {
	u, err := f.ReadDSN(user, name, false)
	if err == nil {
		delete(f.data, u.Name)
		f.dirty = true

		ui.Log(ui.AuthLogger, "Deleted dsn %s", u.Name)
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
	err = ioutil.WriteFile(dsnDatabaseFile, b, 0600)
	if err == nil {
		f.dirty = false

		ui.Log(ui.AuthLogger, "Rewrote file-system credential store")
	} else {
		err = errors.NewError(err)
	}

	return err
}

// AuthDSN determines if the given username is allowed to access the
// named DSN. This will involve lookups to the auth map to determine
// if the DSN is restricted, and if so, is this user on the list?
func (f *fileService) AuthDSN(user, name string, action DSNAction) bool {
	key := user + "|" + name

	if value, found := f.auth[key]; found {
		return (value & action) != DSNNoAccess
	}

	return false
}

// GrantDSN sets the allowed actions for an item. The grant flag indicates if the
// value is granted versus revoked.
func (f *fileService) GrantDSN(user, name string, action DSNAction, grant bool) error {
	key := user + "|" + name

	if value, found := f.auth[key]; found {
		if grant {
			value = value | action
		} else {
			value = value &^ action
		}

		f.auth[key] = value
	} else {
		if grant {
			f.auth[key] = action
		} else {
			f.auth[key] = DSNNoAccess
		}
	}

	return nil
}
