package tables

import (
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables/database"
	"github.com/tucats/ego/util"
)

type Transaction struct {
	id      string
	db      *database.Database
	expires time.Time
}

var transactions = make(map[string]*Transaction)
var transactionsLock sync.Mutex
var transactionsCleanupStarted bool
var MaxTransactions = 100

// GetDatabase opens a new database connection or returns an existing one if a transaction
// was specified in the request (meaning an existing transaction is in progress).
func GetDatabase(session *server.Session, dsnName string, action dsns.DSNAction) (*database.Database, error) {
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
	return database.Open(session, dsnName, dsns.DSNWriteAction)
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
		"uuid":    id,
	})

	return t.db
}

// BeginHandler begins a new transaction for the given DSN. It opens the database connection,
// and starts a new transaction. The transaction id is returned in the response. The transaction
// is stored in the internal active transactions map. If an expiration time was specified as a
// parameter, the transaction will expire after the specified duration.
func BeginHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		err     error
		expires time.Time = time.Now().Add(time.Minute * 5) // Default expiration time is 5 minutes.
	)

	// Get the expiration time from the parameter if present.
	if expiresList := session.Parameters[defs.ExpiresParameterName]; len(expiresList) > 0 {
		duration, err := time.ParseDuration(expiresList[0])
		if err != nil {
			return util.ErrorResponse(w, session.ID, i18n.T("error.tx.expiration"), http.StatusBadRequest)
		}

		expires = time.Now().Add(duration)
	}

	// If there are too many active transactions, return an error.
	transactionsLock.Lock()
	defer transactionsLock.Unlock()

	if len(transactions) >= MaxTransactions {
		return util.ErrorResponse(w, session.ID, i18n.T("error.tx.max"), http.StatusTooManyRequests)
	}

	// Access the database and table.
	dsnName := data.String(session.URLParts["dsn"])

	db, err := database.Open(session, dsnName, dsns.DSNReadAction+dsns.DSNWriteAction)
	if err == nil && db != nil {
		db.Begin()
	}

	t := &Transaction{
		id:      uuid.New().String(),
		db:      db,
		expires: expires,
	}

	// If we haven't started the cleanup timer, do so now. Basically, wake up once a minute and
	// look for expired transactions to rollback and delete.
	if !transactionsCleanupStarted {
		go func() {
			for {
				time.Sleep(time.Second * 60) // Clean up every minute
				cleanupExpiredTransactions()
			}
		}()

		transactionsCleanupStarted = true
	}

	// Add the transaction to the map.
	transactions[t.id] = t
	ui.Log(ui.TableLogger, "table.tx.rest.begin", ui.A{
		"session": session.ID,
		"id":      t.id,
		"expires": expires.Format(time.RFC3339),
	})

	response := defs.TransactionResponse{
		ID:      t.id,
		Expires: expires.Format(time.RFC3339),
	}

	w.Header().Add(defs.ContentTypeHeader, defs.TransactionResponseMediaType)
	b := util.WriteJSON(w, response, &session.ResponseLength)
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
func RollbackHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	// Get the transaction ID parameter from the request.
	parameters := session.Parameters[defs.TransactionIDParameterName]
	if len(parameters) != 1 {
		return util.ErrorResponse(w, session.ID, errors.ErrMissingTransactionID.Error(), http.StatusBadRequest)
	}

	// Get the transaction ID from the request parameters.
	id := data.String(parameters[0])

	// Get the transaction from the map.
	transactionsLock.Lock()
	defer transactionsLock.Unlock()

	tx, ok := transactions[id]
	if !ok {
		return util.ErrorResponse(w, session.ID, errors.ErrTransactionNotFound.Context(id).Error(), http.StatusNotFound)
	}

	tx.db.Rollback()
	ui.Log(ui.TableLogger, "table.tx.rest.rollback", ui.A{
		"session": session.ID,
		"id":      id,
	})

	delete(transactions, id)

	return http.StatusOK
}

// CommitHandler lets caller commit a transaction. Look it up by the parameter "id".
// If the transaction exists, commit it and remove it from the map. If the commit
// fails for any reason, it is still deleted and an error returned to the caller.
func CommitHandler(session *server.Session, w http.ResponseWriter, r *http.Request) int {
	var (
		err error
	)

	// Get the transaction ID parameter from the request.
	parameters := session.Parameters[defs.TransactionIDParameterName]
	if len(parameters) != 1 {
		return util.ErrorResponse(w, session.ID, errors.ErrMissingTransactionID.Error(), http.StatusBadRequest)
	}

	// Get the transaction ID from the request parameters.
	id := data.String(parameters[0])

	// Get the transaction from the map.
	transactionsLock.Lock()
	defer transactionsLock.Unlock()

	tx, ok := transactions[id]
	if !ok {
		return util.ErrorResponse(w, session.ID, errors.ErrTransactionNotFound.Context(id).Error(), http.StatusNotFound)
	}

	err = tx.db.Commit()
	if err != nil {
		ui.Log(ui.TableLogger, "table.tx.rest.commit.error", ui.A{
			"session": session.ID,
			"id":      id,
			"error":   err.Error(),
		})

		return util.ErrorResponse(w, session.ID, i18n.T("error.db.commit"), http.StatusInternalServerError)
	}

	ui.Log(ui.TableLogger, "table.tx.rest.commit", ui.A{
		"session": session.ID,
		"id":      id,
	})

	delete(transactions, id)

	return http.StatusOK
}
