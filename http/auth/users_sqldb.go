package auth

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

type DatabaseService struct {
	constr string
	driver string
	db     *sql.DB
}

const (
	probeTableExistsQueryString = `
	select * from credentials where 1=0`

	createTableQueryString = `
	create table credentials(
		name char varying(50) unique,
		id char(36),
		password char varying(128),
		permissions char varying(1024)) `

	insertQueryString = `
		insert into credentials(name, id, password, permissions) 
        	values($1,$2,$3,$4) `

	updateQueryString = `
		update credentials 
		set password=$2, 
			permissions=$3 
		where name=$1`

	readUserQueryString = `
		select name, id, password, permissions 
    		from credentials where name = $1
`

	listUsersQueryString = `
		select name, id, permissions 
			from credentials
`
	deleteUserQueryString = `
		delete from credentials where name=$1
`
)

func NewDatabaseService(connStr, defaultUser, defaultPassword string) (UserIOService, *errors.EgoError) {
	svc := &DatabaseService{}

	// Is the URL formed correctly?
	url, err := url.Parse(connStr)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	// Get the driver from the URL. If it's SQLITE3 we have to strip
	// off the scheme and leave just a filename path.
	svc.driver = url.Scheme
	if svc.driver == "sqlite3" {
		connStr = strings.TrimPrefix(connStr, "sqlite3://")
	}

	// Create a database connection
	svc.db, err = sql.Open(svc.driver, connStr)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	// If there was a password specified in the URL, blank it out now before we log it.
	if pstr, found := url.User.Password(); found {
		svc.constr = strings.ReplaceAll(connStr, tokenizer.ColonToken+pstr+"@", tokenizer.ColonToken+strings.Repeat("*", len(pstr))+"@")
	} else {
		svc.constr = connStr
	}

	if dberr := svc.initializeDatabase(); !errors.Nil(dberr) {
		ui.Debug(ui.ServerLogger, "Database error: %v", dberr)

		return nil, errors.New(dberr)
	}

	// Does the default user already exist? If not, create it.
	_, e2 := svc.ReadUser(defaultUser)
	if !errors.Nil(e2) {
		user := defs.User{
			Name:        defaultUser,
			Password:    HashString(defaultPassword),
			ID:          uuid.New(),
			Permissions: []string{"root", "logon"},
		}

		e2 = svc.WriteUser(user)

		if errors.Nil(e2) {
			ui.Debug(ui.AuthLogger, "Default database credential %s created", user.Name)
		}
	}

	if errors.Nil(e2) {
		ui.Debug(ui.AuthLogger, "Database credential store %s", svc.constr)
	} else {
		ui.Debug(ui.ServerLogger, "Database error: %v", e2)
	}

	return svc, e2
}

func (pg *DatabaseService) ListUsers() map[string]defs.User {
	r := map[string]defs.User{}

	rowSet, dberr := pg.db.Query(listUsersQueryString)
	if rowSet != nil {
		defer rowSet.Close()
	}

	if dberr != nil {
		ui.Debug(ui.ServerLogger, "Database error: %v", dberr)

		return r
	}

	for rowSet.Next() {
		var name, id, perms string

		dberr = rowSet.Scan(&name, &id, &perms)
		if dberr != nil {
			ui.Debug(ui.ServerLogger, "Database error: %v", dberr)

			return r
		}

		user := defs.User{
			Name:     name,
			ID:       uuid.MustParse(id),
			Password: "********",
		}

		if err := json.Unmarshal([]byte(perms), &user.Permissions); err != nil {
			user.Permissions = []string{}
		}

		r[name] = user
	}

	return r
}

func (pg *DatabaseService) ReadUser(name string) (defs.User, *errors.EgoError) {
	var err *errors.EgoError

	var user defs.User

	rowSet, dberr := pg.db.Query(readUserQueryString, name)

	if rowSet != nil {
		defer rowSet.Close()
	}

	if dberr != nil {
		ui.Debug(ui.ServerLogger, "Database error: %v", dberr)

		return user, errors.New(dberr)
	}

	found := false

	for rowSet.Next() {
		var name, id, password, perms string

		dberr = rowSet.Scan(&name, &id, &password, &perms)
		if dberr != nil {
			ui.Debug(ui.ServerLogger, "Database error: %v", dberr)

			return user, errors.New(dberr)
		}

		user.Name = name
		user.ID, _ = uuid.Parse(id)
		user.Password = password

		err := json.Unmarshal([]byte(perms), &user.Permissions)
		if err != nil {
			return user, errors.New(err)
		}

		found = true
	}

	if !found {
		ui.Debug(ui.AuthLogger, "No database record for %s", name)
		err = errors.New(errors.ErrNoSuchUser).Context(name)
	}

	return user, err
}

func (pg *DatabaseService) WriteUser(user defs.User) *errors.EgoError {
	var err *errors.EgoError

	b, _ := json.Marshal(user.Permissions)
	permString := string(b)

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	action := ""

	_, dberr := pg.ReadUser(user.Name)
	if errors.Nil(dberr) {
		action = "updated in"

		_, e3 := pg.db.Exec(updateQueryString, user.Name, user.Password, permString)
		if e3 != nil {
			dberr = errors.New(e3)
		}
	} else {
		action = "added to"
		_, e3 := pg.db.Exec(insertQueryString, user.Name, user.ID, user.Password, permString)
		dberr = errors.New(e3)
	}

	if !errors.Nil(dberr) {
		ui.Debug(ui.ServerLogger, "Database error: %v", dberr)

		err = errors.New(dberr)
	} else {
		ui.Debug(ui.AuthLogger, "User %s %s database", user.Name, action)
	}

	return err
}

func (pg *DatabaseService) DeleteUser(name string) *errors.EgoError {
	var err *errors.EgoError

	r, dberr := pg.db.Exec(deleteUserQueryString, name)
	if dberr != nil {
		ui.Debug(ui.ServerLogger, "Database error: %v", dberr)

		err = errors.New(dberr)
	} else {
		if count, _ := r.RowsAffected(); count > 0 {
			ui.Debug(ui.AuthLogger, "Deleted user %s from database", name)
		} else {
			ui.Debug(ui.AuthLogger, "No user %s in database", name)
		}
	}

	return err
}

// Required interface, but does no work for the Database service.
func (pg *DatabaseService) Flush() *errors.EgoError {
	var err *errors.EgoError

	return err
}

// Verify that the database is initialized.
func (pg *DatabaseService) initializeDatabase() *errors.EgoError {
	rows, dberr := pg.db.Query(probeTableExistsQueryString)
	if dberr != nil {
		_, dberr := pg.db.Exec(createTableQueryString)

		if dberr == nil {
			ui.Debug(ui.AuthLogger, "Created empty credentials table")
		} else {
			ui.Debug(ui.ServerLogger, "error creating table: %v", dberr)
		}
	}

	defer rows.Close()

	return errors.New(dberr)
}
