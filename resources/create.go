package resources

import (
	"errors"

	"github.com/tucats/ego/app-cli/ui"
)

// Create will create the table for the associated resource type. An error
// is produced if the resource handle is nil or the database table cannot
// be created.
func (r *ResHandle) Create() error {
	var err error

	if r.Database == nil {
		return errors.New("database not open")
	}

	// If there isn't a default primary key, set one now.
	r.SetDefaultPrimaryKey()

	sql := r.createTableSQL()

	ui.Log(ui.ResourceLogger, "[0] Create: %s", sql)

	_, err = r.Database.Exec(sql)

	return err
}

// CreateIf will create the underlying database table for the given resource
// handle if the table does not already exist.
func (r *ResHandle) CreateIf() error {
	var err error

	if r.Database == nil {
		return errors.New("database not open")
	}

	sql := r.doesTableExistSQL()

	ui.Log(ui.ResourceLogger, "[0] CreateIf: %s", sql)

	rows, err := r.Database.Query(sql)
	if rows != nil {
		defer rows.Close()
	}

	if err != nil {
		return r.Create()
	}

	return err
}
