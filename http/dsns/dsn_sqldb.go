package dsns

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/google/uuid"
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
		provider char varying(32),
		database char varying(255),
		host char varying(255),
		port integer,
		username char varying(128),
		pass char varying(1024),
		secured boolean,
		native boolean)`

	insertQueryString = `
		insert into dsns(name, id, provider, database, host, port, username, pass, secured, native) 
        	values($1,$2,$3,$4,$5,$6,$7,$8,$9, $10) `

	readDSNQueryString = `
		select name, id, provider, database, host, port, username, pass, secured, native 
    		from dsns where name = $1
`

	updateDSNQueryString = `
		update dsns
			set database=$1
			set host=$2
			set port=$3
			set username=$4
			set pass=$5
			set secured=$6
			set native=$7
			set provider=$8
			where name=$9`

	listDSNQueryString = `
		select name, id, provider, database, host, port, username, secured, native
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

func (pg *databaseService) ListDSNS(user string) map[string]defs.DSN {
	r := map[string]defs.DSN{}

	rowSet, dberr := pg.db.Query(listDSNQueryString)
	if rowSet != nil {
		defer rowSet.Close()
	}

	if dberr != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", dberr)

		return r
	}

	for rowSet.Next() {
		var (
			name, id, provider, database, host, username string
			port                                         int
			secured, native                              bool
		)

		dberr = rowSet.Scan(&name, &id, &provider, &database, &host, &port, &username, &secured, &native)
		if dberr != nil {
			ui.Log(ui.ServerLogger, "Database error: %v", dberr)

			return r
		}

		dsname := defs.DSN{
			Name:     name,
			ID:       id,
			Provider: provider,
			Database: database,
			Host:     host,
			Port:     port,
			Username: username,
			Password: "********",
			Secured:  secured,
			Native:   native,
		}

		r[name] = dsname
	}

	return r
}

func (pg *databaseService) ReadDSN(user, name string, doNotLog bool) (defs.DSN, error) {
	var err error

	var dsname defs.DSN

	rowSet, dberr := pg.db.Query(readDSNQueryString, name)

	if rowSet != nil {
		defer rowSet.Close()
	}

	if dberr != nil {
		ui.Log(ui.ServerLogger, "Database error: %v", dberr)

		return dsname, errors.NewError(dberr)
	}

	found := false

	for rowSet.Next() {
		var (
			name, id, provider, database, host, username, password string
			port                                                   int
			secured, native                                        bool
		)

		dberr = rowSet.Scan(&name, &id, &provider, &database, &host, &port, &username, &password, &secured, &native)
		if dberr != nil {
			ui.Log(ui.ServerLogger, "Database error: %v", dberr)

			return dsname, errors.NewError(dberr)
		}

		dsname = defs.DSN{
			Name:     name,
			ID:       id,
			Provider: provider,
			Database: database,
			Host:     host,
			Port:     port,
			Username: username,
			Password: password,
			Secured:  secured,
			Native:   native,
		}

		found = true
	}

	if !found {
		if !doNotLog {
			ui.Log(ui.AuthLogger, "No dsn record for %s", name)
		}

		err = errors.ErrNoSuchUser.Context(name)
	}

	return dsname, err
}

func (pg *databaseService) WriteDSN(user string, dsname defs.DSN) error {
	var (
		err error
		tx  *sql.Tx
	)

	action := "updated in"

	_, dberr := pg.ReadDSN(user, dsname.Name, false)
	if dberr == nil {
		tx, _ = pg.db.Begin()

		ui.Log(ui.SQLLogger, "[0] Query: %s", util.SessionLog(0, updateDSNQueryString))
		ui.Log(ui.SQLLogger, "[0] Parms: $1='%s', $2='%s', $3=%d, $4='%s', $5='%s', $6=%v, $7=%v, $8='%s', $9='%s'",
			dsname.Database,
			dsname.Host,
			dsname.Port,
			dsname.Username,
			dsname.Password,
			dsname.Secured,
			dsname.Native,
			dsname.Provider,
			dsname.Name)

		rslt, e3 := pg.db.Exec(updateDSNQueryString,
			dsname.Database,
			dsname.Host,
			dsname.Port,
			dsname.Username,
			dsname.Password,
			dsname.Secured,
			dsname.Native,
			dsname.Provider,
			dsname.Name)
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
		dsname.ID = uuid.NewString()

		ui.Log(ui.SQLLogger, "[0] Query: %s", util.SessionLog(0, insertQueryString))
		ui.Log(ui.SQLLogger, "[0] Parms: $1='%s', $2='%s', $3='%s', $4='%s', $5='%s', $6=%d, $7='%s', $8='%s', $9=%v, $10=%v",
			dsname.Name,
			dsname.ID,
			dsname.Provider,
			dsname.Database,
			dsname.Host,
			dsname.Port,
			dsname.Username,
			dsname.Password,
			dsname.Secured,
			dsname.Native,
		)

		_, e3 := pg.db.Exec(insertQueryString,
			dsname.Name,
			dsname.ID,
			dsname.Provider,
			dsname.Database,
			dsname.Host,
			dsname.Port,
			dsname.Username,
			dsname.Password,
			dsname.Secured,
			dsname.Native,
		)

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

func (pg *databaseService) DeleteDSN(user, name string) error {
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

// AuthDSN determines if the given username is allowed to access the
// named DSN. This will involve lookups to the DSNAUTH table to determine
// if the DSN is restricted, and if so, is this user on the list?
//
// @tomcole for now, just return true.
func (pg *databaseService) AuthDSN(user, name string, action DSNAction) bool {
	return true
}
