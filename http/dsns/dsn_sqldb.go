package dsns

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

type databaseService struct {
	constr string
	driver string
	db     *sql.DB
}

const (
	probeTableExistsQueryString = `
	select * from dsns where 1=0`

	createTableQueryString = `
	create table dsns(
		name char varying(50) unique,
		id char(36),
		password char varying(128),
		permissions char varying(1024)) `

	insertQueryString = `
		insert into dsns(name, id, password, permissions) 
        	values($1,$2,$3,$4) `

	readUserQueryString = `
		select name, id, password, permissions 
    		from dsns where name = $1
`

	listUsersQueryString = `
		select name, id, permissions 
			from dsns
`
	deleteUserQueryString = `
		delete from dsns where name=$1
`
)

func NewDatabaseService(connStr string) (dsnService, error) {
	svc := &databaseService{}

	// Is the URL formed correctly?
	url, err := url.Parse(connStr)
	if err != nil {
		return nil, errors.NewError(err)
	}

	// Get the driver from the URL. If it's SQLITE3 we have to strip
	// off the scheme and leave just a filename path.
	svc.driver = url.Scheme
	if svc.driver == "sqlite3" {
		connStr = strings.TrimPrefix(connStr, "sqlite3://")
	}

	// Create a database connection
	svc.db, err = sql.Open(svc.driver, connStr)
	if err != nil {
		return nil, errors.NewError(err)
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

func (pg *databaseService) ListDSNS() map[string]defs.DSN {
	r := map[string]defs.DSN{}

	rowSet, dberr := pg.db.Query(listUsersQueryString)
	if rowSet != nil {
		defer rowSet.Close()
	}

	if dberr != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", dberr)

		return r
	}

	for rowSet.Next() {
		var name, id, perms string

		// @tomcole update with database columns for DSNS
		dberr = rowSet.Scan(&name, &id, &perms)
		if dberr != nil {
			ui.Log(ui.ServerLogger, "Database error: %v", dberr)

			return r
		}

		dsname := defs.DSN{
			Name:     name,
			Password: "********",
		}

		r[name] = dsname
	}

	return r
}

func (pg *databaseService) ReadDSN(name string, doNotLog bool) (defs.DSN, error) {
	var err error

	var user defs.DSN

	rowSet, dberr := pg.db.Query(readUserQueryString, name)

	if rowSet != nil {
		defer rowSet.Close()
	}

	if dberr != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", dberr)

		return user, errors.NewError(dberr)
	}

	found := false

	for rowSet.Next() {
		var name, id, password, perms string

		dberr = rowSet.Scan(&name, &id, &password, &perms)
		if dberr != nil {
			ui.Log(ui.ServerLogger, "Database error: %v", dberr)

			return user, errors.NewError(dberr)
		}

		// @tomcole update with database columns for DSNS
		user.Name = name
		user.Password = password

		found = true
	}

	if !found {
		if !doNotLog {
			ui.Log(ui.AuthLogger, "No database record for %s", name)
		}

		err = errors.ErrNoSuchUser.Context(name)
	}

	return user, err
}

func (pg *databaseService) WriteDSN(dsname defs.DSN) error {
	var (
		err error
		tx  *sql.Tx
	)

	action := ""

	_, dberr := pg.ReadDSN(dsname.Name, false)
	if dberr == nil {
		tx, _ = pg.db.Begin()
		action = "updated in"

		// @tomcole update with database columns for DSNS
		query := fmt.Sprintf("update dsns\n  set password='%s',\n  where name='%s'",
			dsname.Password, dsname.Name)
		ui.Log(ui.SQLLogger, "[0] Query\n%s", util.SessionLog(0, query))

		rslt, e3 := pg.db.Exec(query)
		if e3 != nil {
			dberr = errors.NewError(e3)
		} else {
			if count, e4 := rslt.RowsAffected(); count != 1 {
				if e4 == nil {
					dberr = errors.NewError(errors.ErrWrongUserUpdatedCount).Context(count)
				} else {
					dberr = e4
				}
			}
		}
	} else {
		action = "added to"

		ui.Log(ui.SQLLogger, "[0] Query: %s", util.SessionLog(0, insertQueryString))
		ui.Log(ui.SQLLogger, "[0] Parms: $1='%s', $2='%s'",
			dsname.Name, dsname.Password)

		// @tomcole update with database columns for DSNS
		_, e3 := pg.db.Exec(insertQueryString, dsname.Name, dsname.Password)
		if e3 != nil {
			e3 = errors.NewError(e3)
		}

		dberr = e3
	}

	if dberr != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", dberr)

		_ = tx.Rollback()
		err = errors.NewError(dberr)
	} else {
		if tx != nil {
			err = tx.Commit()
		}

		ui.Log(ui.AuthLogger, "User %s %s database", dsname.Name, action)
	}

	return err
}

func (pg *databaseService) DeleteDSN(name string) error {
	var err error

	r, dberr := pg.db.Exec(deleteUserQueryString, name)
	if dberr != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", dberr)

		err = errors.NewError(dberr)
	} else {
		if count, _ := r.RowsAffected(); count > 0 {
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
	rows, dberr := pg.db.Query(probeTableExistsQueryString)

	defer func() {
		if rows != nil {
			rows.Close()
		}
	}()

	if dberr != nil {
		_, dberr = pg.db.Exec(createTableQueryString)
		if dberr == nil {
			ui.Log(ui.AuthLogger, "Created empty dsns table")
		} else {
			ui.Log(ui.ServerLogger, "error creating table: %v", dberr)
		}
	}

	if dberr != nil {
		dberr = errors.NewError(dberr)
	}

	return dberr
}
