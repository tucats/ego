package dsns

import (
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/resources"
)

type databaseService struct {
	constr     string
	dsnHandle  *resources.ResHandle
	authHandle *resources.ResHandle
}

// Create a new service for supporting data source names, using a database
// as the persistent store.  The connection string to the database must be
// supplied. This will initialize a service and return it. If an error
// occurs accessing the database or creating the required tables, then
// a nill service pointer is returned along with an error value.
func NewDatabaseService(connStr string) (dsnService, error) {
	svc := &databaseService{}

	// Is the URL formed correctly?
	url, err := url.Parse(connStr)
	if err != nil {
		return nil, errors.New(err)
	}

	// Resource handle for a DSN.
	svc.dsnHandle, err = resources.Open(defs.DSN{}, "dsns", connStr)
	if err != nil {
		return svc, err
	}

	// Resource handle for a DSN authorization record
	svc.authHandle, err = resources.Open(DSNAuthorization{}, "dsns_auth", connStr)
	if err != nil {
		return svc, err
	}

	// Even though a DSN has an "id" field, it is indexed on the name of the DSN.
	svc.dsnHandle.SetPrimaryKey("name")

	// If there was a password specified in the URL, blank it out now before we log it.
	if pstr, found := url.User.Password(); found {
		svc.constr = strings.ReplaceAll(connStr, ":"+pstr+"@", ":"+strings.Repeat("*", len(pstr))+"@")
	} else {
		svc.constr = connStr
	}

	if dberr := svc.initializeDatabase(); dberr != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", dberr)

		return nil, errors.New(dberr)
	}

	ui.Log(ui.AuthLogger, "Database data source name store %s", svc.constr)

	return svc, nil
}

// ListDSNS lists the data source names stored in the database. The user
// parameter is not currently used.
func (pg *databaseService) ListDSNS(user string) (map[string]defs.DSN, error) {
	r := map[string]defs.DSN{}

	// Specify the sort info (ordered by the DSN name) and read the data.
	iArray, err := pg.dsnHandle.Sort("name").Read()
	if err != nil {
		return r, err
	}

	for _, item := range iArray {
		dsn := item.(*defs.DSN)
		dsn.Password = "*********"
		r[dsn.Name] = *dsn
	}

	// Clear the sort order.
	pg.dsnHandle.Sort()

	return r, nil
}

// ReadDSN reads a DSN definition from the database for the given user and data source name.
// The user parameter is not currently used. IF the DSN can be found by name, it is returned.
// If not, an empty DSN struct is returned along with a non-nil error code.
func (pg *databaseService) ReadDSN(user, name string, doNotLog bool) (defs.DSN, error) {
	var (
		err    error
		dsname defs.DSN
		item   interface{}
		found  bool
	)

	if item, found = caches.Find(caches.DSNCache, name); !found {
		item, err = pg.dsnHandle.ReadOne(name)
		if err != nil {
			if !doNotLog {
				ui.Log(ui.AuthLogger, "No dsn record for %s", name)
			}

			return dsname, err
		}
	}

	dsname = *item.(*defs.DSN)

	return dsname, err
}

func (pg *databaseService) WriteDSN(user string, dsname defs.DSN) error {
	var (
		err error
	)

	action := "updated in"

	caches.Delete(caches.DSNCache, dsname.Name)

	items, err := pg.dsnHandle.Read(pg.dsnHandle.Equals("name", dsname.Name))
	if err != nil {
		return err
	}

	if len(items) == 0 {
		action = "added to"
		dsname.ID = uuid.NewString()

		err = pg.dsnHandle.Insert(dsname)
	} else {
		err = pg.dsnHandle.UpdateOne(dsname)
	}

	if err != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", err)

		err = errors.New(err)
	} else {
		caches.Add(caches.DSNCache, dsname.Name, &dsname)
		ui.Log(ui.AuthLogger, "User %s %s database", dsname.Name, action)
	}

	return err
}

func (pg *databaseService) DeleteDSN(user, name string) error {
	var err error

	caches.Delete(caches.DSNCache, name)

	err = pg.dsnHandle.DeleteOne(name)
	if err == nil {
		// Delete any authentication objects for this DSN as well...
		_, _ = pg.authHandle.Delete(pg.authHandle.Equals("dsn", name))

		ui.Log(ui.AuthLogger, "Deleted DSN %s from database", name)
	}

	return err
}

// Required interface, but does no work for the Database service.
func (pg *databaseService) Flush() error {
	var err error

	return err
}

// Verify that the database is initialized.
func (pg *databaseService) initializeDatabase() error {
	err := pg.dsnHandle.CreateIf()
	if err == nil {
		err = pg.authHandle.CreateIf()
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// AuthDSN determines if the given username is allowed to access the
// named DSN. If the DSN is not marked as restricted, then this always
// returns true.  If restricted, an authorization record must exist in
// the dsnauths table, which has a bit-mask of allowed operations. The
// result is a bit-mapped AND of the requested and permitted actions.
func (pg *databaseService) AuthDSN(user, name string, action DSNAction) bool {
	dsn, err := pg.ReadDSN(user, name, true)
	if err != nil {
		return false
	}

	if !dsn.Restricted {
		return true
	}

	rows, err := pg.authHandle.Read(
		pg.authHandle.Equals("user", user),
		pg.authHandle.Equals("dsn", name),
	)

	if err == nil && len(rows) > 0 {
		auth := rows[0].(*DSNAuthorization)

		return (auth.Action & action) != 0
	}

	return false
}

// GrantDSN grants (or revokes) privileges from an existing DSN. If the DSN does not exist
// in the privileges table, it is added.
func (pg *databaseService) GrantDSN(user, name string, action DSNAction, grant bool) error {
	// Does this DSN even exist? If not, this is an error.
	dsn, err := pg.ReadDSN(user, name, true)
	if err != nil {
		return err
	}

	// Get the privilege info for this item.
	rows, err := pg.authHandle.Read(
		pg.authHandle.Equals("user", user),
		pg.authHandle.Equals("dsn", name),
	)

	if err != nil {
		return err
	}

	// If there is a row, scan it in to get the existing value. Otherwise, we default
	// to a zero-value for the authorization if there was no row already in the auth
	// table.
	existingAction := DSNNoAccess
	auth := &DSNAuthorization{}
	exists := false

	if len(rows) > 0 {
		auth = rows[0].(*DSNAuthorization)
		existingAction = auth.Action
		exists = true

		ui.Log(ui.AuthLogger, "[%d] DSN authorization record exists", 0)
	} else {
		ui.Log(ui.AuthLogger, "[%d] DSN authorization record does not already exist", 0)
		
		auth.DSN = name
		auth.User = user
		auth.Action = existingAction
	}

	// Based on the grant (vs revoke) flag, either set or clear
	// the bits associated with the new action mask.
	if grant {
		existingAction = existingAction | action
	} else {
		existingAction = existingAction &^ action
	}

	ui.Log(ui.AuthLogger, "[%d] DSN authorization action mask %d", 0, existingAction)

	// If the DSN was not previously marked as restricted,
	// then update it now to be restricted so future access
	// will use the auth table for authorization checks.
	if !dsn.Restricted {
		dsn.Restricted = true

		ui.Log(ui.AuthLogger, "[%d] DSN %s being marked as restricted", 0, dsn.Name)

		if err = pg.WriteDSN(user, dsn); err != nil {
			return err
		}
	}

	// If this row already existed in the auth table, update the value with the new
	// action mask. If it did not exist before, insert it into the auth table.
	auth.Action = existingAction
	if exists {
		err = pg.authHandle.Update(*auth,
			pg.authHandle.Equals("user", user),
			pg.authHandle.Equals("dsn", name))
	} else {
		err = pg.authHandle.Insert(*auth)
	}

	return err
}

// Permissions returns a map for each user that has access to the named DSN. The map
// indicates the user name and the integer bit mask of the allowed actions.
func (pg *databaseService) Permissions(user, name string) (map[string]DSNAction, error) {
	dsn, err := pg.ReadDSN(user, name, false)
	if err != nil {
		return nil, err
	}

	result := map[string]DSNAction{}
	if !dsn.Restricted {
		return result, nil
	}

	auths, err := pg.authHandle.Read(pg.authHandle.Equals("dsn", name))
	if err != nil {
		return nil, err
	}

	for _, authX := range auths {
		auth := authX.(*DSNAuthorization)
		result[auth.User] = auth.Action
	}

	return result, nil
}
