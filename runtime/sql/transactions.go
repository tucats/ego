package sql

import (
	goSQL "database/sql"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// begin implements db.Client.Begin(). It starts a new database transaction and
// stores the resulting *goSQL.Tx in the transactionFieldName field of the Client
// struct. While a transaction is active, all Execute, Query, and QueryResult
// calls automatically run inside it.
//
// Returns ErrTransactionAlreadyActive if Begin() is called when a transaction
// is already in progress (nested transactions are not supported).
// Returns ErrArgumentCount if any arguments are passed (method takes none).
func begin(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		tx *goSQL.Tx
		e2 error
	)

	if args.Len() > 0 {
		return data.NewList(errors.ErrArgumentCount), errors.ErrArgumentCount
	}

	d, tx, err := client(s)
	if err == nil {
		this := getThis(s)

		if tx == nil {
			tx, e2 = d.Begin()
			if e2 == nil {
				this.SetAlways(transactionFieldName, tx)
			} else {
				err = errors.New(e2)
			}
		} else {
			err = errors.ErrTransactionAlreadyActive
		}
	}

	return data.NewList(err), err
}

// rollback implements db.Client.Rollback(). It aborts the current transaction,
// discarding all changes made since the matching Begin() call, and clears the
// transactionFieldName field so subsequent operations run outside a transaction.
//
// Returns ErrNoTransactionActive when called without a preceding Begin().
// Returns ErrArgumentCount if any arguments are passed (method takes none).
func rollback(s *symbols.SymbolTable, args data.List) (any, error) {
	var tx *goSQL.Tx

	if args.Len() > 0 {
		return data.NewList(errors.ErrArgumentCount), errors.ErrArgumentCount
	}

	_, tx, err := client(s)
	if err == nil {
		this := getThis(s)

		if tx != nil {
			err = tx.Rollback()
		} else {
			err = errors.ErrNoTransactionActive
		}

		this.SetAlways(transactionFieldName, nil)
	}

	if err != nil {
		err = errors.New(err)
	}

	return data.NewList(err), err
}

// commit implements db.Client.Commit(). It commits the current transaction,
// making all changes permanent, and clears the transactionFieldName field so
// subsequent operations run outside a transaction.
//
// Returns ErrNoTransactionActive when called without a preceding Begin().
// Returns ErrArgumentCount if any arguments are passed (method takes none).
func commit(s *symbols.SymbolTable, args data.List) (any, error) {
	var tx *goSQL.Tx

	if args.Len() > 0 {
		return data.NewList(errors.ErrArgumentCount), errors.ErrArgumentCount
	}

	_, tx, err := client(s)
	if err == nil {
		this := getThis(s)

		if tx != nil {
			err = tx.Commit()
		} else {
			err = errors.ErrNoTransactionActive
		}

		this.SetAlways(transactionFieldName, nil)
	}

	if err != nil {
		err = errors.New(err)
	}

	return data.NewList(err), err
}
