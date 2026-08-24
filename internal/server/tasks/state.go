package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/dsns"
	"github.com/tucats/ego/internal/errors"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// stateFileName is the sidecar file, inside the tasks directory, that
// records each task's last-run time and outcome so recurring schedules
// survive a server restart without ever rewriting -- or risking comments
// in -- the user-authored task files. It starts with "." so LoadAll's
// directory scan (which skips hidden files) never mistakes it for a task
// definition. Only used when the shared system database is not available
// -- see stateStore and InitializeStateStore.
const stateFileName = ".state.json"

// persistedState is the persisted shape of one task's execution history.
// Running and LoadedAt are deliberately not part of it: "in progress"
// never survives a restart, and LoadedAt is meant to be re-anchored to
// each new process's own startup (see State.LoadedAt), not persisted.
type persistedState struct {
	LastRun    time.Time `json:"lastRun"`
	LastStatus int       `json:"lastStatus"`
	Success    bool      `json:"success"`
	RunCount   int       `json:"runCount"`
	FailedTest string    `json:"failedTest,omitempty"`
}

// StateFile returns the path to the sidecar state file. Meaningless when
// the active store is database-backed, but still used by tests and by
// permission-hardening code that runs regardless of backend.
func StateFile() string {
	return filepath.Join(Directory(), stateFileName)
}

// stateStore is the persistence backend for task run-state history: either
// the JSON sidecar file (fileStateStore, below) or a table in the shared
// system database (databaseStateStore, state_sqldb.go) when one is
// available. See InitializeStateStore for how the backend is selected.
type stateStore interface {
	// load returns the persisted state for every task known to the store,
	// keyed by task id. A store with nothing persisted yet (a brand new
	// install, or a missing sidecar file) returns an empty, non-nil map
	// and a nil error.
	load() (map[string]persistedState, error)

	// save writes the given full snapshot of every task's persisted state.
	save(persisted map[string]persistedState) error

	// close releases any resources (an open database connection) held by
	// this store. A no-op for the file store.
	close() error
}

// activeStore is the task-state persistence backend in use by this process.
// It defaults to the file store so that code which never calls
// InitializeStateStore -- every existing test, and any use of this package
// outside a running server -- keeps working exactly as before. A real
// server startup calls InitializeStateStore once, after dsns.Initialize, to
// possibly upgrade it to the database store.
var activeStore stateStore = fileStateStore{}

// InitializeStateStore selects the persistence backend for task run-state:
// a table in the shared system database when one is available -- the same
// database used for the DSN catalog and user credentials when
// --users/ego.server.userdata points at a database URL, see
// dsns.DSNDatabaseURL -- or the JSON sidecar file otherwise. Call this
// once, after dsns.Initialize, and before the first LoadState/SaveState
// call.
//
// A database-open failure is not fatal: it is logged and this falls back
// to the file store, the same way other optional startup features degrade
// (see dsns.EnsureSystemDSN).
func InitializeStateStore() {
	_ = activeStore.close()

	if isDatabaseBacked(dsns.DSNDatabaseURL) {
		store, err := newDatabaseStateStore(dsns.DSNDatabaseURL)
		if err == nil {
			activeStore = store

			ui.Log(tasksLogger, "tasks.state.store.database", nil)

			return
		}

		ui.Log(tasksLogger, "tasks.state.store.database.error", ui.A{"error": err.Error()})
	}

	activeStore = fileStateStore{}
}

// isDatabaseBacked reports whether the given DSN connection string -- the
// same one used for the DSN catalog and user credentials, see
// dsns.DSNDatabaseURL -- points at a database rather than a plain file
// path, "memory", or the empty string.
func isDatabaseBacked(connStr string) bool {
	scheme, err := egostrings.FindScheme(connStr)
	if err != nil {
		return false
	}

	switch scheme {
	case defs.PostgresProvider, defs.SqliteProvider, defs.DeprecatedSqliteProvider:
		return true
	default:
		return false
	}
}

// fileStateStore is the original JSON sidecar file backend.
type fileStateStore struct{}

func (fileStateStore) load() (map[string]persistedState, error) {
	path := StateFile()

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]persistedState{}, nil
		}

		return nil, errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	var persisted map[string]persistedState

	if err := json.Unmarshal(b, &persisted); err != nil {
		ui.Log(tasksLogger, "tasks.state.load.error", ui.A{"path": path, "error": err.Error()})

		return map[string]persistedState{}, nil
	}

	return persisted, nil
}

func (fileStateStore) save(persisted map[string]persistedState) error {
	b, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	if err := os.WriteFile(StateFile(), b, requiredFileMode); err != nil {
		return errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	return nil
}

func (fileStateStore) close() error {
	return nil
}

// LoadState reads the active store's persisted state, if any, and applies
// its contents to the in-memory state of every currently-registered task.
// Tasks with no entry in the store -- including every task on a brand new
// install -- simply keep their zero-value state, meaning "never run",
// which makes them immediately due (subject to After). A missing or
// corrupt file (file store) or empty table (database store) is not an
// error: it just means every task starts out looking like it has never
// run. Call this after LoadAll has populated the registry.
//
// This updates fields on the existing *State in place rather than
// replacing it, specifically so it does not clobber LoadedAt -- LoadAll's
// register call already stamped that with this process's own start time,
// and persistedState has nothing to say about it (see State.LoadedAt).
func LoadState() error {
	persisted, err := activeStore.load()
	if err != nil {
		return err
	}

	registryLock.Lock()
	defer registryLock.Unlock()

	for id, entry := range persisted {
		if _, found := registry[id]; !found {
			// Stale entry for a task that was removed or renamed; drop it
			// silently -- the next SaveState call won't write it back.
			continue
		}

		state, found := states[id]
		if !found {
			state = &State{LoadedAt: time.Now()}
			states[id] = state
		}

		state.LastRun = entry.LastRun
		state.LastStatus = entry.LastStatus
		state.Success = entry.Success
		state.RunCount = entry.RunCount
		state.FailedTest = entry.FailedTest
	}

	return nil
}

// SaveState writes the current execution history for every registered task
// to the active store. Called after every task run; at the scale this
// feature targets (a handful of tasks, not thousands) rewriting the whole
// small snapshot each time is simpler than an incremental update and cheap
// enough not to matter, for either backend.
func SaveState() error {
	registryLock.RLock()

	persisted := make(map[string]persistedState, len(states))

	for id, state := range states {
		persisted[id] = persistedState{
			LastRun:    state.LastRun,
			LastStatus: state.LastStatus,
			Success:    state.Success,
			RunCount:   state.RunCount,
			FailedTest: state.FailedTest,
		}
	}

	registryLock.RUnlock()

	return activeStore.save(persisted)
}

// recordRun updates a task's in-memory state after a run completes and
// persists the full state to the active store so the result survives a
// restart. RunCount is incremented unconditionally -- it counts every
// attempt, successful or not, since Task.Count is a lifetime cap on how
// many times the task runs at all, not on how many times it succeeds.
// failedTest is recorded verbatim (including empty, clearing any previous
// value) so it always reflects the *last* run, the same as
// LastStatus/Success. Running is deliberately left true until the state
// write has been attempted: the task isn't really "done" until its result
// is durably recorded, and callers that poll runningCount() to know when a
// run has fully finished (including its state write) depend on that
// ordering.
func recordRun(id string, status int, success bool, failedTest string, when time.Time) {
	registryLock.Lock()

	state, found := states[id]
	if !found {
		state = &State{LoadedAt: time.Now()}
		states[id] = state
	}

	state.LastRun = when
	state.LastStatus = status
	state.Success = success
	state.FailedTest = failedTest
	state.RunCount++

	registryLock.Unlock()

	if err := SaveState(); err != nil {
		ui.Log(tasksLogger, "tasks.state.save.error", ui.A{"error": err.Error()})
	}

	registryLock.Lock()
	state.Running = false
	registryLock.Unlock()
}

// Status returns a copy of the current execution state for a task id. The
// copy is safe to read without further locking.
func Status(id string) (State, bool) {
	registryLock.RLock()
	defer registryLock.RUnlock()

	state, found := states[id]
	if !found {
		return State{}, false
	}

	return *state, true
}
