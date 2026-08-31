package tables

import (
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/dberrors"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/util"
)

type Transaction struct {
	id      string
	db      *database.Database
	expires time.Time
	timeout time.Duration
}

var transactions = make(map[string]*Transaction)
var transactionsLock sync.Mutex
var transactionsCleanupStarted bool
var MaxTransactions = 100

// GetDatabase opens a new database connection or returns an existing one if a transaction
// was specified in the request (meaning an existing transaction is in progress).
func GetDatabase(session *router.Session, dsnName string, action dsns.DSNAction) (*database.Database, error) {
	// Is there a transaction id on this request? If so, grab the existing db for this
	// transaction and return it to the caller.
	if id := session.Parameters[defs.TransactionIDParameterName]; len(id) == 1 {
		txID := id[0]

		db := GetTransactionDB(session.ID, txID)
		if db != nil {
			return db, nil
		} else {
			return nil, errors.ErrTransactionNotFound.Context(txID)
		}
	}

	// No transaction id was given, so just do the regular database connection.
	return database.Open(session, dsnName, action)
}

// GetTransactionDB retrieves the database associated with a specific transaction id.
// If the transaction id is not found, or was expired, it is removed from the map and
// nil is returned.
func GetTransactionDB(session int, id string) *database.Database {
	transactionsLock.Lock()
	defer transactionsLock.Unlock()

	t, ok := transactions[id]
	if !ok || t.expires.Before(time.Now()) {
		delete(transactions, id)

		return nil
	}

	ui.Log(ui.DBLogger, "log.db.tx.using", ui.A{
		"session": session,
		"seq":     t.db.TransID,
		"uuid":    id,
	})

	return t.db
}

// BeginHandler begins a new transaction for the given DSN. It opens the database connection,
// and starts a new transaction. The transaction id is returned in the response. The transaction
// is stored in the internal active transactions map. If an expiration time was specified as a
// parameter, the transaction will expire after the specified duration.
func BeginHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		err     error
		expires time.Time = time.Now().Add(time.Minute * 5) // Default expiration time is 5 minutes.
	)

	// Get the expiration time from the parameter if present.
	if expiresList := session.Parameters[defs.ExpiresParameterName]; len(expiresList) > 0 {
		duration, err := time.ParseDuration(expiresList[0])
		if err != nil {
			return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.tx.expiration"), http.StatusBadRequest)
		}

		expires = time.Now().Add(duration)
	}

	// If there are too many active transactions, return an error.
	transactionsLock.Lock()
	defer transactionsLock.Unlock()

	if len(transactions) >= MaxTransactions {
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.tx.max"), http.StatusTooManyRequests)
	}

	// Access the database and table.
	dsnName := data.String(session.URLParts["dsn"])

	db, err := database.Open(session, dsnName, dsns.DSNReadAction+dsns.DSNWriteAction)
	if err != nil {
		// A DSN named in the URL that does not exist (or that this caller
		// cannot open) is a 404/403, not a server fault -- and, just as
		// important, falling through here would have gone on to build a
		// Transaction wrapping a nil db and then dereference it at
		// "t.db.TransUUID = t.id" a few lines down, panicking instead of
		// returning a clean error.
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), dberrors.PayloadStatus(err))
	}

	if db == nil {
		// database.Open should never return (nil, nil), but guard against
		// it so a violation of that contract is a clear 500 rather than the
		// same nil-dereference panic described above.
		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.db.nil.pointer"), http.StatusInternalServerError)
	}

	if err := db.Begin(); err != nil {
		db.Close()

		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	t := &Transaction{
		id:      uuid.New().String(),
		db:      db,
		expires: expires,             // When this transaction will expire
		timeout: time.Until(expires), // If the user hits keepalive, re-up for the same expiration duration window.
	}

	// If we haven't started the cleanup timer, do so now. Basically, wake up once a minute and
	// look for expired transactions to rollback and delete.
	//
	// NILPTR-6: the cleanup call is wrapped in util.SafeCall because this runs on
	// its own goroutine, where Go does not recover panics for us -- an unrecovered
	// panic in a background goroutine terminates the entire server process, not
	// just the goroutine. Since this task walks a map of transactions whose
	// database handles may already have been closed, a panic here is plausible;
	// logging it and retrying on the next minute is much better than exiting.
	if !transactionsCleanupStarted {
		go func() {
			for {
				time.Sleep(time.Second * 60) // Clean up every minute
				util.SafeCall("cleanup expired transactions", cleanupExpiredTransactions)
			}
		}()

		transactionsCleanupStarted = true
	}

	// Add the transaction to the map.
	t.db.TransUUID = t.id
	transactions[t.id] = t
	ui.Log(ui.TableLogger, "table.tx.rest.begin", ui.A{
		"session": session.ID,
		"id":      t.db.TransUUID,
		"seq":     t.db.TransID,
		"expires": expires.Format(time.RFC3339),
	})

	msg := i18n.T("log.sql.begin", ui.A{
		"id":       t.id,
		"seq":      t.db.TransID,
		"database": db.Name,
	})

	response := defs.TransactionResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		ID:         t.id,
		Expires:    expires.Format(time.RFC3339),
		Message:    msg,
	}

	w.Header().Add(defs.ContentTypeHeader, defs.TransactionResponseMediaType)
	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)
	ui.WriteLog(ui.RestLogger, "rest.response.payload", ui.A{
		"session": session.ID,
		"body":    string(b)})

	return http.StatusOK
}

// cleanupExpiredTransactions removes expired transactions from the map. It scans
// the map and removes any entries whose expiration timestamp is in the past.
func cleanupExpiredTransactions() {
	transactionsLock.Lock()
	defer transactionsLock.Unlock()

	for id, tx := range transactions {
		if time.Now().After(tx.expires) {
			ui.Log(ui.TableLogger, "table.tx.rest.cleanup", ui.A{
				"session": tx.id,
				"seq":     tx.db.TransID,
				"id":      tx.db.TransUUID,
				"expires": tx.expires.Format(time.RFC3339),
			})

			delete(transactions, id)

			if tx.db != nil {
				tx.db.Rollback()
				tx.db.Close()
			}
		}
	}
}

// RollbackHandler lets caller rollback a transaction. Look it up by the parameter "id".
// If the transaction exists, rollback it and remove it from the map.
func RollbackHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// Get the transaction ID parameter from the request.
	parameters := session.Parameters[defs.TransactionIDParameterName]
	if len(parameters) != 1 {
		return util.ErrorResponse(w, session.ID, errors.ErrMissingTransactionID.Localize(session.Language), http.StatusBadRequest)
	}

	// Get the transaction ID from the request parameters.
	id := data.String(parameters[0])

	// Get the transaction from the map.
	transactionsLock.Lock()
	defer transactionsLock.Unlock()

	tx, ok := transactions[id]
	if !ok {
		return util.ErrorResponse(w, session.ID, errors.ErrTransactionNotFound.Context(id).Localize(session.Language), http.StatusNotFound)
	}

	tx.db.Rollback()
	ui.Log(ui.TableLogger, "table.tx.rest.rollback", ui.A{
		"session": session.ID,
		"seq":     tx.db.TransID,
		"id":      id,
	})

	delete(transactions, id)

	// Rollback() has already cleared tx.db.Transaction, so this actually closes
	// the underlying connection (see Close's own "active/pending transaction"
	// guard) rather than no-oping. Without this, every successful rollback
	// through this REST endpoint leaked its connection pool forever -- only
	// cleanupExpiredTransactions closed one, and only for a transaction that
	// was never resolved at all (see that function's own Rollback+Close pair).
	tx.db.Close()

	return http.StatusOK
}

// CommitHandler lets caller commit a transaction. Look it up by the parameter "id".
// If the transaction exists, commit it and remove it from the map. If the commit
// fails for any reason, it is still deleted and an error returned to the caller.
func CommitHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		err error
	)

	// Get the transaction ID parameter from the request.
	parameters := session.Parameters[defs.TransactionIDParameterName]
	if len(parameters) != 1 {
		return util.ErrorResponse(w, session.ID, errors.ErrMissingTransactionID.Localize(session.Language), http.StatusBadRequest)
	}

	// Get the transaction ID from the request parameters.
	id := data.String(parameters[0])

	// Get the transaction from the map.
	transactionsLock.Lock()
	defer transactionsLock.Unlock()

	tx, ok := transactions[id]
	if !ok {
		return util.ErrorResponse(w, session.ID, errors.ErrTransactionNotFound.Context(id).Localize(session.Language), http.StatusNotFound)
	}

	// Use the transaction we found to do a commit
	err = tx.db.Commit()
	if err != nil {
		ui.Log(ui.TableLogger, "table.tx.rest.commit.error", ui.A{
			"session": session.ID,
			"id":      id,
			"seq":     tx.db.TransID,
			"error":   err.Error(),
		})

		// A failed Commit() leaves tx.db.Transaction non-nil (see Commit's own
		// doc comment -- it only clears Transaction on success), so plain
		// Close() would no-op on its "active/pending transaction" guard and
		// leak this connection forever, same as the missing Close() this whole
		// function's other branch fixes. CloseTX resolves the still-open
		// transaction itself (committing again, or rolling back if that also
		// fails -- see its own doc comment) before closing the handle, so the
		// connection is reclaimed either way. This also makes the code match
		// this function's own doc comment, which already promised a failed
		// commit "is still deleted" -- delete(transactions, id) was missing
		// from this path entirely before.
		delete(transactions, id)
		tx.db.CloseTX(session.ID)

		return util.ErrorResponse(w, session.ID, i18n.Text(session.Language, "error.db.commit"), http.StatusInternalServerError)
	}

	ui.Log(ui.TableLogger, "table.tx.rest.commit", ui.A{
		"session": session.ID,
		"id":      id,
		"seq":     tx.db.TransID,
	})

	// Commit() has already cleared tx.db.Transaction on this success path, so
	// this redundant assignment and the delete below are unchanged from
	// before; only the Close() call is new -- see RollbackHandler's identical
	// fix above for why it was needed.
	tx.db.Transaction = nil

	delete(transactions, id)
	tx.db.Close()

	return http.StatusOK
}

// KeepaliveHandler lets callers indicate they are still interested in a transaction.
// This is meant to block harvesting (and auto-rollback) of transactions, if the client
// side is doing a long-running operation. For example, the egoAdmin app opens a transaction
// when it edits a data table, keeping it alive for as long as the user has the window
// active. It uses this call periodically to prevent harvesting of the transaction as an
// abandoned transaction handle.
func KeepaliveHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// Get the transaction ID parameter from the request.
	parameters := session.Parameters[defs.TransactionIDParameterName]
	if len(parameters) != 1 {
		return util.ErrorResponse(w, session.ID, errors.ErrMissingTransactionID.Localize(session.Language), http.StatusBadRequest)
	}

	// Get the transaction ID from the request parameters.
	id := data.String(parameters[0])

	// Get the transaction from the map.
	transactionsLock.Lock()
	defer transactionsLock.Unlock()

	tx, ok := transactions[id]
	if !ok {
		return util.ErrorResponse(w, session.ID, errors.ErrTransactionNotFound.Context(id).Localize(session.Language), http.StatusNotFound)
	}

	// Calculate the next transaction timeout timestamp, and store the (now revitalized)
	// transaction value back in the map.
	tx.expires = time.Now().Add(tx.timeout)
	transactions[id] = tx

	ui.Log(ui.TableLogger, "table.tx.rest.keepalive", ui.A{
		"session": session.ID,
		"seq":     tx.db.TransID,
		"id":      id,
		"expires": tx.expires.Format(time.RFC3339),
	})

	response := defs.DSNKeepaliveResponse{
		ServerInfo: util.MakeServerInfo(session.ID),
		ID:         tx.id,
		Expires:    tx.expires.Format(time.RFC3339),
	}

	b := util.WriteJSON(w, session.Response(), http.StatusOK, response)
	ui.Log(ui.RestLogger, "rest.response.payload", ui.A{
		"body":    string(b),
		"session": session.ID,
	})

	return http.StatusOK
}
