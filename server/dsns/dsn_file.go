package dsns

import (
	"encoding/json"
	"os"
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

// NewFileService creates a new file-based DSN service. If the userDatabaseFile
// is "memory" then an in-memory service is created. Otherwise, the file is read
// and the contents are used to initialize the service.
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

		if b, err := os.ReadFile(userDatabaseFile); err != nil {
			return nil, errors.New(err)
		} else {
			if key := settings.Get(defs.LogonUserdataKeySetting); key != "" {
				if r, err := util.Decrypt(string(b), key); err != nil {
					return svc, errors.New(err)
				} else {
					b = []byte(r)
				}
			}

			if err = json.Unmarshal(b, &svcObject); err != nil {
				return svc, errors.New(err)
			}

			svc.Data = svcObject.Data
			svc.Auth = svcObject.Auth

			ui.Log(ui.AuthLogger, "auth.file.size", ui.A{
				"size": len(svc.Data)})
		}
	}

	if len(svc.Data) == 0 {
		svc.Data = map[string]defs.DSN{}
		svc.dirty = true

		ui.Log(ui.AuthLogger, "auth.dsn.memory", nil)
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
		err = errors.ErrNoSuchDSN.Context(name)
	}

	return dsn, err
}

func (f *fileService) WriteDSN(user string, dsn defs.DSN) error {
	_, found := f.Data[dsn.Name]
	f.Data[dsn.Name] = dsn
	f.dirty = true

	if found {
		ui.Log(ui.AuthLogger, "auth.dsn.update", ui.A{
			"name": dsn.Name})
	} else {
		ui.Log(ui.AuthLogger, "auth.dsn.create", ui.A{
			"name": dsn.Name})
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

		ui.Log(ui.AuthLogger, "auth.dsn.delete", ui.A{
			"name": u.Name})
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
		return errors.New(err)
	}

	if key := settings.Get(defs.LogonUserdataKeySetting); key != "" {
		r, err := util.Encrypt(string(b), key)
		if err != nil {
			return err
		}

		b = []byte(r)
	}

	// Write to the database file.
	err = os.WriteFile(dsnDatabaseFile, b, 0600)
	if err == nil {
		f.dirty = false

		ui.Log(ui.AuthLogger, "auth.dsn.flush", nil)
	} else {
		err = errors.New(err)
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
