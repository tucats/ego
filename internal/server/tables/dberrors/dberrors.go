// Package dberrors is the one place that decides which HTTP status code a
// failed table operation reports.
//
// Before REST-1, every handler made that decision for itself, and there were
// three different policies in use for the same class of error: some returned
// 409 for everything, some split 400/409 by which stage failed, and some
// started from a default and upgraded it if the error message happened to
// contain a particular English substring. The same bad value could come back as
// 400 or 409 depending on whether the request was an insert or an update, and
// the same missing table could come back as 400, 404, or 500 depending on which
// handler was reached and which database was behind the DSN.
//
// The substring approach had a deeper problem than inconsistency: it tried to
// recover a fact -- "this was a missing table", "this was a uniqueness
// conflict" -- that the driver already knew and that had been flattened into a
// string on the way up. This package asks the driver instead, through the typed
// errors both supported drivers provide:
//
//   - PostgreSQL's lib/pq returns *pq.Error carrying a SQLSTATE code, the
//     five-character identifier the SQL standard defines for exactly this
//     purpose.
//   - SQLite's modernc.org/sqlite returns *sqlite.Error carrying a SQLite
//     result code.
//
// Ego's own errors are recognized by identity with errors.Equals, which works
// in every language -- the previous check compared against English message text
// and so silently failed on a server running in any other locale.
//
// See docs/API.md for the resulting contract, and docs/issues/REST-1.md for the
// full account.
package dberrors

import (
	"errors"
	"net/http"
	"strings"

	"github.com/lib/pq"
	egoerrors "github.com/tucats/ego/internal/errors"
	sqlite "modernc.org/sqlite"
)

// Class is what a failure turns out to have been, independent of which HTTP
// status any particular caller decides that deserves.
type Class int

const (
	// Unclassified means the error carried nothing this package recognizes.
	// The caller's own default applies, because only the caller knows whether
	// an unrecognized failure at its particular point is the client's fault or
	// the server's.
	Unclassified Class = iota

	// NotFound is a reference to a table or relation that does not exist.
	NotFound

	// Conflict is a violation of a uniqueness or foreign key constraint: the
	// request is well-formed, but it disagrees with data already stored.
	Conflict

	// InvalidValue is a value the database refused on its own terms -- a NULL
	// in a NOT NULL column, or a failed CHECK constraint. These are the
	// client's mistake even though the database is what noticed.
	InvalidValue

	// Permission is Ego's own refusal to perform the operation for this user.
	Permission
)

// SQLite result codes. These are declared here rather than imported from
// modernc.org/sqlite/lib so that reading this file does not require going and
// looking them up; they are part of SQLite's published ABI and do not change.
// The low byte is the primary result code and the rest is the "extended" code
// identifying which kind of constraint was violated.
const (
	sqliteError                = 1    // SQLITE_ERROR, the generic catch-all
	sqliteConstraint           = 19   // SQLITE_CONSTRAINT, base code
	sqliteConstraintCheck      = 275  // SQLITE_CONSTRAINT_CHECK
	sqliteConstraintForeignKey = 787  // SQLITE_CONSTRAINT_FOREIGNKEY
	sqliteConstraintNotNull    = 1299 // SQLITE_CONSTRAINT_NOTNULL
	sqliteConstraintPrimaryKey = 1555 // SQLITE_CONSTRAINT_PRIMARYKEY
	sqliteConstraintUnique     = 2067 // SQLITE_CONSTRAINT_UNIQUE
)

// PostgreSQL SQLSTATE codes, from the standard's class 23 (integrity constraint
// violation) and class 42 (syntax error or access rule violation).
const (
	pgNotNullViolation    = "23502"
	pgForeignKeyViolation = "23503"
	pgUniqueViolation     = "23505"
	pgCheckViolation      = "23514"
	pgUndefinedTable      = "42P01"
)

// Classify works out what a failed database or coercion operation actually was.
//
// It returns Unclassified for anything it does not recognize, rather than
// guessing. A caller turns that into a status code with PayloadStatus or
// ExecStatus below, each of which supplies the default appropriate to where it
// sits.
func Classify(err error) Class {
	if err == nil {
		return Unclassified
	}

	// Ego's own permission refusal. errors.Equals compares the error's
	// identity, not its text, so this is correct whatever language the server
	// is running in.
	if egoerrors.Equals(err, egoerrors.ErrNoPrivilegeForOperation) {
		return Permission
	}

	// A value that could not be converted to its column's type never reached
	// the database at all; the payload is at fault.
	if egoerrors.Equals(err, egoerrors.ErrAmbiguousTimeZone) ||
		egoerrors.Equals(err, egoerrors.ErrInvalidColumnName) {
		return InvalidValue
	}

	// errors.As walks the chain of wrapped errors looking for one of the given
	// type. Ego's *errors.Error implements Unwrap, so a driver error keeps its
	// type even after Ego has wrapped it with additional context.
	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		return classifyPostgres(pgErr)
	}

	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		return classifySQLite(sqliteErr)
	}

	return Unclassified
}

// classifyPostgres maps a SQLSTATE code to a Class. PostgreSQL distinguishes
// every case we care about by code, so no message text is consulted.
func classifyPostgres(err *pq.Error) Class {
	switch string(err.Code) {
	case pgUndefinedTable:
		return NotFound

	case pgUniqueViolation, pgForeignKeyViolation:
		return Conflict

	case pgNotNullViolation, pgCheckViolation:
		return InvalidValue

	default:
		return Unclassified
	}
}

// classifySQLite maps a SQLite result code to a Class.
//
// SQLite's constraint codes are as precise as PostgreSQL's, but it has no
// distinct code for "no such table" -- that arrives as the generic
// SQLITE_ERROR, the same code a syntax error uses. So this is the one place
// where message text still has to be consulted. It is a far narrower use than
// the checks it replaces: the error must already be a SQLite error carrying
// exactly the generic code before the text is looked at, rather than any error
// from anywhere being searched for a substring.
func classifySQLite(err *sqlite.Error) Class {
	switch err.Code() {
	case sqliteConstraintUnique, sqliteConstraintPrimaryKey, sqliteConstraintForeignKey:
		return Conflict

	case sqliteConstraintNotNull, sqliteConstraintCheck:
		return InvalidValue

	case sqliteError:
		// "SQL logic error: no such table: events (1)"
		if strings.Contains(err.Error(), "no such table") {
			return NotFound
		}

		return Unclassified

	default:
		// A constraint violation whose extended code we do not recognize is
		// still a constraint violation. The base result code is the low byte.
		if err.Code()&0xff == sqliteConstraint {
			return Conflict
		}

		return Unclassified
	}
}

// PayloadStatus is the status for a failure raised while building a query from
// the request body -- coercing values to their column types, resolving column
// names, checking permissions. Nothing has been sent to the database yet, so an
// unrecognized failure here is the client's payload being wrong: the default is
// 400.
func PayloadStatus(err error) int {
	return status(err, http.StatusBadRequest)
}

// ExecStatus is the status for a failure raised while executing a query. Ego
// built that SQL itself, so an unrecognized failure is not something the client
// can correct by sending a different payload: the default is 500. A recognized
// one -- a missing table, a uniqueness conflict, a rejected value -- still
// reports what it actually was.
func ExecStatus(err error) int {
	return status(err, http.StatusInternalServerError)
}

// status maps a classification to an HTTP status code, falling back to the
// caller's default when nothing was recognized. The mapping is documented for
// API clients in docs/API.md; keep the two in step.
func status(err error, unclassified int) int {
	switch Classify(err) {
	case NotFound:
		return http.StatusNotFound

	case Conflict:
		return http.StatusConflict

	case InvalidValue:
		return http.StatusBadRequest

	case Permission:
		return http.StatusForbidden

	default:
		return unclassified
	}
}
