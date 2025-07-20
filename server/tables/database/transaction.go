package database

import (
	"sync/atomic"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

var nextTransactionID uint64

func (d *Database) Begin() error {
	if d.Transaction != nil {
		return errors.ErrTransactionAlreadyActive
	}

	d.TransID = atomic.AddUint64(&nextTransactionID, 1)

	ui.Log(ui.SQLLogger, "sql.begin", ui.A{
		"session":  d.Session.ID,
		"database": d.Name,
		"id":       d.TransID,
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
		"session": d.Session.ID,
		"id":      d.TransID,
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
		"session": d.Session.ID,
		"id":      d.TransID,
	})

	err := d.Transaction.Rollback()
	if err != nil {
		return err
	}

	d.Transaction = nil

	return nil
}
