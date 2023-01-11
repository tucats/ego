package db

import (
	"database/sql"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Begin implements the Begin() db function. This allocated a new structure that
// contains all the info needed to call the database, including the function pointers
// for the functions available to a specific handle.
func Begin(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	var tx *sql.Tx

	d, tx, err := client(s)
	if err == nil {
		this := getThisStruct(s)

		if tx == nil {
			var e2 error

			tx, e2 = d.Begin()
			if e2 == nil {
				this.SetAlways(transactionFieldName, tx)
			}
		} else {
			err = errors.ErrTransactionAlreadyActive
		}
	}

	return nil, err
}

// Rollback implements the Rollback() db function.
func Rollback(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	var tx *sql.Tx

	_, tx, err := client(s)
	if err == nil {
		this := getThisStruct(s)

		if tx != nil {
			err = tx.Rollback()
		} else {
			err = errors.ErrNoTransactionActive
		}

		this.SetAlways(transactionFieldName, nil)
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return nil, err
}

// Commit implements the Commit() db function.
func Commit(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	var tx *sql.Tx

	_, tx, err := client(s)
	if err == nil {
		this := getThisStruct(s)

		if tx != nil {
			err = tx.Commit()
		} else {
			err = errors.ErrNoTransactionActive
		}

		this.SetAlways(transactionFieldName, nil)
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return nil, err
}
