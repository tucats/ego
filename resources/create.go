package resources

import (
	"errors"

	"github.com/tucats/ego/app-cli/ui"
)

func (r *ResHandle) Create() error {
	var err error

	if r.Database == nil {
		return errors.New("database not open")
	}

	sql := r.createTableSQL()

	ui.Log(ui.DBLogger, "[0] Resource create: %s", sql)

	_, err = r.Database.Exec(sql)

	return err
}

func (r *ResHandle) CreateIf() error {
	var err error

	if r.Database == nil {
		return errors.New("database not open")
	}

	sql := r.doesTableExistSQL()

	ui.Log(ui.DBLogger, "[0] Resource createIf: %s", sql)

	rows, err := r.Database.Query(sql)
	if rows != nil {
		defer rows.Close()
	}

	if err != nil {
		return r.Create()
	}

	return err
}
