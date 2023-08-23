package dsns

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

type fileService struct {
	Path  string
	dirty bool
	Data  map[string]defs.DSN
	Auth  map[string]DSNAction
}

func NewFileService(userDatabaseFile string) (dsnService, error) {
	if userDatabaseFile == "memory" {
		userDatabaseFile = ""
	}

	svc := &fileService{
		Path: userDatabaseFile,
		Data: map[string]defs.DSN{},
		Auth: map[string]DSNAction{},
	}

	if userDatabaseFile != "" {
		svcObject := fileService{}

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
				err = json.Unmarshal(b, &svcObject)
			}

			if err != nil {
				return svc, errors.NewError(err)
			}

			svc.Data = svcObject.Data
			svc.Auth = svcObject.Auth

			ui.Log(ui.AuthLogger, "Using file-system credential store with %d items", len(svc.Data))
		}
	}

	if svc.Data == nil || len(svc.Data) == 0 {
		svc.Data = map[string]defs.DSN{}
		svc.dirty = true

		ui.Log(ui.AuthLogger, "Creating new empty DSN table in memory")
	}

	return svc, nil
}

func (f *fileService) ListDSNS(user string) (map[string]defs.DSN, error) {
	return f.Data, nil
}

func (f *fileService) ReadDSN(user, name string, doNotLog bool) (defs.DSN, error) {
	var err error

	dsn, ok := f.Data[name]
	if !ok {
		err = errors.ErrNoSuchUser.Context(name)
	}

	return dsn, err
}

func (f *fileService) WriteDSN(user string, dsn defs.DSN) error {
	_, found := f.Data[dsn.Name]
	f.Data[dsn.Name] = dsn
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
		key := user + "|" + name
		f.dirty = true

		delete(f.Data, u.Name)
		delete(f.Auth, key)

		ui.Log(ui.AuthLogger, "Deleted dsn %s", u.Name)
	}

	return nil
}

// Flush writes the file-based data to a json file. This operation is not
// done if there were no changes to the database, or there is not a database
// file name given.
func (f *fileService) Flush() error {
	if !f.dirty || f.Path == "" {
		return nil
	}

	// Convert the database to a json string
	b, err := json.MarshalIndent(f, "", "   ")
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

	if value, found := f.Auth[key]; found {
		return (value & action) != DSNNoAccess
	}

	return false
}

// GrantDSN sets the allowed actions for an item. The grant flag indicates if the
// value is granted versus revoked.
func (f *fileService) GrantDSN(user, name string, action DSNAction, grant bool) error {
	key := user + "|" + name

	if value, found := f.Auth[key]; found {
		if grant {
			value = value | action
		} else {
			value = value &^ action
		}

		f.Auth[key] = value
	} else {
		if grant {
			f.Auth[key] = action
		} else {
			f.Auth[key] = DSNNoAccess
		}
	}

	return nil
}

func (f *fileService) Permissions(user, name string) (map[string]DSNAction, error) {
	result := map[string]DSNAction{}

	d, err := f.ReadDSN(user, name, true)
	if err != nil {
		return result, err
	}

	if !d.Restricted {
		return result, nil
	}

	for key, auth := range f.Auth {
		parts := strings.Split(key, "|")
		if parts[1] == name {
			result[parts[0]] = auth
		}
	}

	return result, nil
}
