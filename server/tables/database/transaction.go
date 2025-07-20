package database

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

func (d *Database) Begin() error {
	if d.Transaction != nil {
		return errors.ErrTransactionAlreadyActive
	}

	ui.Log(ui.SQLLogger, "sql.begin", ui.A{
		"session":  d.Session.ID,
		"database": d.Name,
	})

	tx, err := d.Handle.Begin()
	if err != nil {
		return err
	}

	d.Transaction = tx

	return nil
}

func (d *Database) Commit() error {
	if d.Transaction == nil {
		return errors.ErrNoTransactionActive
	}

	ui.Log(ui.SQLLogger, "sql.commit", ui.A{
		"session":  d.Session.ID,
		"database": d.Name,
	})

	err := d.Transaction.Commit()
	if err != nil {
		return err
	}

	d.Transaction = nil

	return nil
}

func (d *Database) Rollback() error {
	if d.Transaction == nil {
		return errors.ErrNoTransactionActive
	}

	ui.Log(ui.SQLLogger, "sql.rollback", ui.A{
		"session":  d.Session.ID,
		"database": d.Name,
	})

	err := d.Transaction.Rollback()
	if err != nil {
		return err
	}

	d.Transaction = nil

	return nil
}
