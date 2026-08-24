package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/errors"
)

// stateFileName is the sidecar file, inside the tasks directory, that
// records each task's last-run time and outcome so recurring schedules
// survive a server restart without ever rewriting -- or risking comments
// in -- the user-authored task files. It starts with "." so LoadAll's
// directory scan (which skips hidden files) never mistakes it for a task
// definition.
const stateFileName = ".state.json"

// persistedState is the on-disk shape of one task's execution history.
// Running and LoadedAt are deliberately not part of it: "in progress"
// never survives a restart, and LoadedAt is meant to be re-anchored to
// each new process's own startup (see State.LoadedAt), not persisted.
type persistedState struct {
	LastRun    time.Time `json:"lastRun"`
	LastStatus int       `json:"lastStatus"`
	Success    bool      `json:"success"`
	RunCount   int       `json:"runCount"`
}

// StateFile returns the path to the sidecar state file.
func StateFile() string {
	return filepath.Join(Directory(), stateFileName)
}

// LoadState reads the sidecar state file, if present, and applies its
// contents to the in-memory state of every currently-registered task.
// Tasks with no entry in the file -- including every task on a brand new
// install -- simply keep their zero-value state, meaning "never run",
// which makes them immediately due (subject to After). A missing or
// corrupt file is not an error: it just means every task starts out
// looking like it has never run. Call this after LoadAll has populated
// the registry.
//
// This updates fields on the existing *State in place rather than
// replacing it, specifically so it does not clobber LoadedAt -- LoadAll's
// register call already stamped that with this process's own start time,
// and persistedState has nothing to say about it (see State.LoadedAt).
func LoadState() error {
	path := StateFile()

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	var persisted map[string]persistedState

	if err := json.Unmarshal(b, &persisted); err != nil {
		ui.Log(tasksLogger, "tasks.state.load.error", ui.A{"path": path, "error": err.Error()})

		return nil
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
	}

	return nil
}

// SaveState writes the current execution history for every registered task
// to the sidecar state file. Called after every task run; at the scale
// this feature targets (a handful of tasks, not thousands) rewriting the
// whole small file each time is simpler than an incremental update and
// cheap enough not to matter.
func SaveState() error {
	registryLock.RLock()

	persisted := make(map[string]persistedState, len(states))

	for id, state := range states {
		persisted[id] = persistedState{
			LastRun:    state.LastRun,
			LastStatus: state.LastStatus,
			Success:    state.Success,
			RunCount:   state.RunCount,
		}
	}

	registryLock.RUnlock()

	b, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	if err := os.WriteFile(StateFile(), b, requiredFileMode); err != nil {
		return errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	return nil
}

// recordRun updates a task's in-memory state after a run completes and
// persists the full state file so the result survives a restart. RunCount
// is incremented unconditionally -- it counts every attempt, successful or
// not, since Task.Count is a lifetime cap on how many times the task runs
// at all, not on how many times it succeeds. Running is deliberately left
// true until the state file write has been attempted: the task isn't
// really "done" until its result is durably recorded, and callers that
// poll runningCount() to know when a run has fully finished (including its
// state write) depend on that ordering.
func recordRun(id string, status int, success bool, when time.Time) {
	registryLock.Lock()

	state, found := states[id]
	if !found {
		state = &State{LoadedAt: time.Now()}
		states[id] = state
	}

	state.LastRun = when
	state.LastStatus = status
	state.Success = success
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
