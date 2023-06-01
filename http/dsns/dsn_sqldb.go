package dsns

import (
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/resources"
)

type databaseService struct {
	constr     string
	dsnHandle  *resources.ResHandle
	authHandle *resources.ResHandle
}

func NewDatabaseService(connStr string) (dsnService, error) {
	svc := &databaseService{}

	// Is the URL formed correctly?
	url, err := url.Parse(connStr)
	if err != nil {
		return nil, errors.NewError(err)
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

	// If there was a password specified in the URL, blank it out now before we log it.
	if pstr, found := url.User.Password(); found {
		svc.constr = strings.ReplaceAll(connStr, ":"+pstr+"@", ":"+strings.Repeat("*", len(pstr))+"@")
	} else {
		svc.constr = connStr
	}

	if dberr := svc.initializeDatabase(); dberr != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", dberr)

		return nil, errors.NewError(dberr)
	}

	ui.Log(ui.AuthLogger, "Database data source name store %s", svc.constr)

	return svc, nil
}

func (pg *databaseService) ListDSNS(user string) (map[string]defs.DSN, error) {
	r := map[string]defs.DSN{}

	iArray, err := pg.dsnHandle.Read()
	if err != nil {
		return r, err
	}

	for _, item := range iArray {
		dsn := item.(*defs.DSN)
		dsn.Password = "*********"
		r[dsn.Name] = *dsn
	}

	return r, nil
}

func (pg *databaseService) ReadDSN(user, name string, doNotLog bool) (defs.DSN, error) {
	var err error

	var dsname defs.DSN

	item, err := pg.dsnHandle.Read(&resources.Filter{Name: "name", Value: "'" + name + "'", Operator: "="})
	if err != nil {
		return dsname, err
	}

	found := len(item) > 0

	if found {
		dsname = *item[0].(*defs.DSN)
	} else {
		if !doNotLog {
			ui.Log(ui.AuthLogger, "No dsn record for %s", name)
		}

		err = errors.ErrNoSuchDSN.Context(name)
	}

	return dsname, err
}

func (pg *databaseService) WriteDSN(user string, dsname defs.DSN) error {
	var (
		err error
	)

	action := "updated in"

	items, err := pg.dsnHandle.Read(&resources.Filter{Name: "name", Value: "'" + dsname.Name + "'", Operator: "="})

	if err != nil {
		return err
	}

	if len(items) == 0 {
		action = "added to"
		dsname.ID = uuid.NewString()

		err = pg.dsnHandle.Insert(dsname)
	} else {
		err = pg.dsnHandle.Update(dsname, resources.Filter{Name: "name", Value: "'" + dsname.Name + "'", Operator: "="})
	}

	if err != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", err)

		err = errors.NewError(err)
	} else {
		ui.Log(ui.AuthLogger, "User %s %s database", dsname.Name, action)
	}

	return err
}

func (pg *databaseService) DeleteDSN(user, name string) error {
	var err error

	count, err := pg.dsnHandle.Delete(&resources.Filter{Name: "name", Value: "'" + name + "'", Operator: "="})
	if err == nil {
		if count > 0 {
			ui.Log(ui.AuthLogger, "Deleted user %s from database", name)
		} else {
			ui.Log(ui.AuthLogger, "No user %s in database", name)
		}
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
		err = errors.NewError(err)
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
		&resources.Filter{
			Name:     "user",
			Value:    "'" + user + "'",
			Operator: "=",
		},
		&resources.Filter{
			Name:     "name",
			Value:    "'" + name + "'",
			Operator: "=",
		},
	)

	if err == nil && len(rows) > 0 {
		auth := rows[0].(DSNAuthorization)

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
		&resources.Filter{
			Name:     "user",
			Value:    "'" + user + "'",
			Operator: "=",
		},
		&resources.Filter{
			Name:     "name",
			Value:    "'" + name + "'",
			Operator: "=",
		},
	)

	if err != nil {
		return err
	}

	// If there is a row, scan it in to get the existing value. Otherwise, we default
	// to a zero-value for the authorization if there was no row already in the auth
	// table.
	existingAction := DSNNoAccess
	auth := DSNAuthorization{}
	exists := false

	if len(rows) > 0 {
		auth = rows[0].(DSNAuthorization)
		existingAction = auth.Action
		exists = true
	}

	// Based on the grant (vs revoke) flag, either set or clear
	// the bits associated with the new action mask.
	if grant {
		existingAction = existingAction | action
	} else {
		existingAction = existingAction &^ action
	}

	// If the DSN was not previously marked as restricted,
	// then update it now to be restricted so future access
	// will use the auth table for authorization checks.
	if !dsn.Restricted {
		if err = pg.WriteDSN(user, dsn); err != nil {
			return err
		}
	}

	// If this row already existed in the auth table, update the value with the new
	// action mask. If it did not exist before, insert it into the auth table.
	if exists {
		auth.Action = existingAction
		err = pg.authHandle.Update(auth,
			resources.Filter{
				Name:     "user",
				Value:    "'" + user + "'",
				Operator: "=",
			},
			resources.Filter{
				Name:     "name",
				Value:    "'" + name + "'",
				Operator: "=",
			},
		)
	} else {
		err = pg.authHandle.Insert(auth)
	}

	return err
}
