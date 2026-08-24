package tasks

import (
	"time"

	"github.com/tucats/ego/internal/resources"
)

// taskStateTable is the name of the table holding one row per task's
// persisted run-state, when the database store is active.
const taskStateTable = "task_state"

// taskStateRow is the SQL-table shape of one task's persisted run-state,
// keyed by task ID. LastRun is stored as an RFC3339 string rather than a
// time.Time because internal/resources.describe has no case for time.Time
// (it would silently produce an untyped column) -- every other SQL-backed
// struct in this codebase follows the same string-timestamp convention,
// e.g. defs.User.LastTokenAt and auth's StartLogEntry.Time.
type taskStateRow struct {
	ID         string
	LastRun    string
	LastStatus int
	Success    bool
	RunCount   int
	FailedTest string
}

// databaseStateStore persists task run-state as rows in the shared system
// database -- the same database used for the DSN catalog and user
// credentials when --users/ego.server.userdata points at a database URL --
// instead of the JSON sidecar file.
type databaseStateStore struct {
	handle *resources.ResHandle
}

// newDatabaseStateStore opens (creating if necessary) the task_state table
// in the database identified by connStr.
func newDatabaseStateStore(connStr string) (*databaseStateStore, error) {
	handle, err := resources.Open(taskStateRow{}, taskStateTable, connStr)
	if err != nil {
		return nil, err
	}

	handle.SetPrimaryKey("ID")

	if err := handle.CreateIf(); err != nil {
		handle.Close()

		return nil, err
	}

	return &databaseStateStore{handle: handle}, nil
}

func (s *databaseStateStore) load() (map[string]persistedState, error) {
	rows, err := s.handle.Begin().Read()
	if err != nil {
		return nil, err
	}

	result := make(map[string]persistedState, len(rows))

	for _, row := range rows {
		r, ok := row.(*taskStateRow)
		if !ok {
			continue
		}

		lastRun, _ := time.Parse(time.RFC3339, r.LastRun)

		result[r.ID] = persistedState{
			LastRun:    lastRun,
			LastStatus: r.LastStatus,
			Success:    r.Success,
			RunCount:   r.RunCount,
			FailedTest: r.FailedTest,
		}
	}

	return result, nil
}

// save upserts every entry in the given snapshot. A single Read up front
// determines which task IDs already have a row, so each entry costs one
// Insert or UpdateOne rather than a Read-then-write round trip per task.
func (s *databaseStateStore) save(persisted map[string]persistedState) error {
	rows, err := s.handle.Begin().Read()
	if err != nil {
		return err
	}

	existing := make(map[string]bool, len(rows))

	for _, row := range rows {
		if r, ok := row.(*taskStateRow); ok {
			existing[r.ID] = true
		}
	}

	for id, state := range persisted {
		row := taskStateRow{
			ID:         id,
			LastRun:    state.LastRun.UTC().Format(time.RFC3339),
			LastStatus: state.LastStatus,
			Success:    state.Success,
			RunCount:   state.RunCount,
			FailedTest: state.FailedTest,
		}

		if existing[id] {
			err = s.handle.Begin().UpdateOne(row)
		} else {
			err = s.handle.Begin().Insert(row)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (s *databaseStateStore) close() error {
	return s.handle.Close()
}
