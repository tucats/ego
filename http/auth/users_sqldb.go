package auth

import (
	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/resources"
)

type databaseService struct {
	constr     string
	userHandle *resources.ResHandle
}

func NewDatabaseService(connStr, defaultUser, defaultPassword string) (userIOService, error) {
	var (
		err error
		svc = &databaseService{}
	)

	svc.userHandle, err = resources.Open(defs.User{}, "credentials", connStr)
	if err != nil {
		return nil, errors.NewError(err)
	}

	if err = svc.userHandle.CreateIf(); err != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", err)

		return nil, errors.NewError(err)
	}

	// Does the default user already exist? If not, create it.
	_, err = svc.ReadUser(defaultUser, true)
	if err != nil {
		user := defs.User{
			Name:        defaultUser,
			Password:    HashString(defaultPassword),
			ID:          uuid.New(),
			Permissions: []string{"root", "logon"},
		}

		err = svc.userHandle.Insert(user)

		if err == nil {
			ui.Log(ui.AuthLogger, "Default database credential %s created", user.Name)
		}
	}

	if err == nil {
		ui.Log(ui.AuthLogger, "Database credential store %s", svc.constr)
	} else {
		ui.Log(ui.ServerLogger, "Database error: %v", err)
	}

	return svc, err
}

func (pg *databaseService) ListUsers() map[string]defs.User {
	r := map[string]defs.User{}

	rowSet, err := pg.userHandle.Read()
	if err != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", err)

		return r
	}

	for _, row := range rowSet {
		user := row.(*defs.User)
		user.Password = "********"

		r[user.Name] = *user
	}

	return r
}

func (pg *databaseService) ReadUser(name string, doNotLog bool) (defs.User, error) {
	var (
		err   error
		user  *defs.User
		found bool
	)

	rowSet, err := pg.userHandle.Read(pg.userHandle.Equals("name", name))
	if err != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", err)

		return defs.User{}, errors.NewError(err)
	}

	for _, row := range rowSet {
		user = row.(*defs.User)
		found = true
	}

	if !found {
		if !doNotLog {
			ui.Log(ui.AuthLogger, "No database record for %s", name)
		}

		return defs.User{}, errors.ErrNoSuchUser.Context(name)
	}

	return *user, err
}

func (pg *databaseService) WriteUser(user defs.User) error {
	action := ""

	_, err := pg.ReadUser(user.Name, false)
	if err == nil {
		action = "updated in"
		err = pg.userHandle.Update(user, pg.userHandle.Equals("name", user.Name))
	} else {
		action = "added to"
		err = pg.userHandle.Insert(user)
	}

	if err != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", err)

		err = errors.NewError(err)
	} else {
		ui.Log(ui.AuthLogger, "User %s %s database", user.Name, action)
	}

	return err
}

func (pg *databaseService) DeleteUser(name string) error {
	var err error

	count, err := pg.userHandle.Delete(pg.userHandle.Equals("name", name))
	if err != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", err)

		err = errors.NewError(err)
	} else {
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
