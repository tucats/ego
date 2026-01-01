package resources

import (
	"errors"

	"github.com/tucats/ego/app-cli/ui"
	egoErrors "github.com/tucats/ego/errors"
)

// Create will create the table for the associated resource type. An error
// is produced if the resource handle is nil or the database table cannot
// be created.
func (r *ResHandle) Create() error {
	var err error

	if r == nil {
		return egoErrors.ErrNoResourceHandle
	}

	if r.Err != nil {
		return r.Err
	}

	if r.Database == nil {
		return errors.New("database not open")
	}

	// If there isn't a default primary key, set one now.
	r.SetDefaultPrimaryKey()

	sql := r.createTableSQL()

	ui.Log(ui.ResourceLogger, "resource.create", ui.A{
		"sql": sql})

	_, err = r.Database.Exec(sql)

	return err
}

// CreateIf will create the underlying database table for the given resource
// handle if the table does not already exist.
func (r *ResHandle) CreateIf() error {
	var err error

	if r == nil {
		return egoErrors.ErrNoResourceHandle
	}

	if r.Err != nil {
		return r.Err
	}

	if r.Database == nil {
		return errors.New("database not open")
	}

	sql := r.doesTableExistSQL()

	ui.Log(ui.ResourceLogger, "resource.createif", ui.A{
		"sql": sql})

	rows, err := r.Database.Query(sql)
	if rows != nil {
		defer rows.Close()
	}

	if err != nil {
		return r.Create()
	}

	return err
}

// This resets the state of a resource handle to a known state before
// beginning a chain of operations. For example, this resets the error
// state such that any subsequent operations (like applying filters) will
// result in a new error state that can be detected by the caller.
func (r *ResHandle) Begin() *ResHandle {
	if r == nil {
		return nil
	}

	r.Err = nil

	return r
}
