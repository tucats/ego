package auth

import (
	"reflect"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/resources"
)

type databaseService struct {
	constr     string
	userHandle *resources.ResHandle
}

// NewDatabaseService creates a new user service that uses a database to store user information.
// The connStr parameter is the connection string to the database, and the defaultUser and
// defaultPassword are used to create a default user if one does not already exist.
func NewDatabaseService(connStr, defaultUser, defaultPassword string) (userIOService, error) {
	var (
		err error
		svc = &databaseService{}
	)

	// Use the resources manager to open the database connection.
	svc.userHandle, err = resources.Open(defs.User{}, "credentials", connStr)
	if err != nil {
		return nil, errors.New(err)
	}

	svc.constr = connStr

	// Create the underlying database table definition if it does not yet exist.
	if err = svc.userHandle.CreateIf(); err != nil {
		ui.Log(ui.ServerLogger, "server.db.error", ui.A{
			"error": err})

		return nil, errors.New(err)
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
			ui.Log(ui.AuthLogger, "auth.user.create", ui.A{
				"user": user.Name})
		}
	}

	if err == nil {
		ui.Log(ui.AuthLogger, "auth.db", ui.A{
			"constr": svc.constr})
	} else {
		ui.Log(ui.ServerLogger, "server.db.error", ui.A{
			"error": err})
	}

	return svc, err
}

// ListUsers returns a map of all users in the database. The password value
// is always masked with asterisks.
func (pg *databaseService) ListUsers() map[string]defs.User {
	r := map[string]defs.User{}

	rowSet, err := pg.userHandle.Begin().Read()
	if err != nil {
		ui.Log(ui.ServerLogger, "server.db.error", ui.A{
			"error": err})

		return r
	}

	for _, row := range rowSet {
		user := row.(*defs.User)
		user.Password = "********"

		r[user.Name] = *user
	}

	return r
}

// ReadUser returns a user definition from the database. If the doNotLog
// parameter is true, the operation is not logged to the AUTH audit log.
func (pg *databaseService) ReadUser(name string, doNotLog bool) (defs.User, error) {
	var (
		err   error
		user  *defs.User
		found bool
	)

	// Is it in the short-term cache?
	if item, found := caches.Find(caches.AuthCache, name); found {
		user, ok := item.(defs.User)
		if !ok {
			return defs.User{}, errors.ErrInvalidCacheItem.Context(reflect.TypeOf(item).String())
		}

		return user, nil
	}

	rowSet, err := pg.userHandle.Begin().Read(pg.userHandle.Equals("name", name))
	if err != nil {
		ui.Log(ui.ServerLogger, "server.db.error", ui.A{
			"error": err})

		return defs.User{}, errors.New(err)
	}

	for _, row := range rowSet {
		user = row.(*defs.User)
		found = true
	}

	if !found {
		if !doNotLog {
			ui.Log(ui.AuthLogger, "auth.user.not.found", ui.A{
				"user": name})
		}

		return defs.User{}, errors.ErrNoSuchUser.Context(name)
	}

	// Add the item to the short-term cache, and return it to the caller.
	caches.Add(caches.AuthCache, name, *user)

	return *user, err
}

// WriteUser adds or updates a user definition in the database. If the user
// already exists, it is updated. If the user does not exist, it is added.
func (pg *databaseService) WriteUser(user defs.User) error {
	var update string

	caches.Delete(caches.AuthCache, user.Name)

	_, err := pg.ReadUser(user.Name, false)
	if err == nil {
		update = "auth.user.update"
		err = pg.userHandle.Begin().Update(user, pg.userHandle.Equals("name", user.Name))
	} else {
		update = "auth.user.create"
		err = pg.userHandle.Begin().Insert(user)
	}

	if err != nil {
		ui.Log(ui.ServerLogger, "server.db.error", ui.A{
			"error": err})

		err = errors.New(err)
	} else {
		ui.Log(ui.AuthLogger, update, ui.A{
			"user": user.Name})
	}

	return err
}

// DeleteUser removes a user definition from the database.
func (pg *databaseService) DeleteUser(name string) error {
	var err error

	// Make sure the item no longer exists in the short-term cache.
	caches.Delete(caches.AuthCache, name)

	count, err := pg.userHandle.Begin().Delete(pg.userHandle.Equals("name", name))
	if err != nil {
		ui.Log(ui.ServerLogger, "server.db.error", ui.A{
			"error": err})

		err = errors.New(err)
	} else {
		if count > 0 {
			ui.Log(ui.AuthLogger, "auth.user.delete", ui.A{
				"user": name})
		} else {
			ui.Log(ui.AuthLogger, "auth.user.not.found", ui.A{
				"user": name})
		}
	}

	return err
}

// Required interface, but does no work for the Database service. For
// the database service, the data is always written to the database
// immediately.
func (pg *databaseService) Flush() error {
	var err error

	return err
}
