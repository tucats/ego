// Package tasks implements scheduled server tasks: JSON definitions under
// lib/tasks that describe a call to an Ego server endpoint, optionally
// repeated on an interval. See docs/internals/TASKS.md for the design.
package tasks

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/tucats/ego/internal/cli/ui"
)

// tasksLogger is the log class used for all task-subsystem activity.
var tasksLogger = ui.DefineLogger("TASKS", false)

// Task describes one scheduled call to an Ego server endpoint, as loaded
// from a JSON file under lib/tasks/.
type Task struct {
	// Description is free text describing the task, used only in logging.
	Description string `json:"task"`

	// ID uniquely identifies the task. Must be unique across every file in
	// the tasks directory; a UUID is recommended but not required.
	ID string `json:"id"`

	// Active indicates whether the scheduler may run this task. A task with
	// Active false is still loaded and validated, but never dispatched.
	Active bool `json:"active,string"`

	// User is the identity the task runs as; the endpoint call carries this
	// user's real, live permissions.
	User string `json:"user"`

	// Method is the HTTP method used for the endpoint call (e.g. "post").
	Method string `json:"method"`

	// Endpoint is the server path to call (e.g. "/services/jiggle").
	Endpoint string `json:"endpoint"`

	// Parameters are URL query parameters to send with the request.
	Parameters map[string]string `json:"parameters,omitempty"`

	// Body is the raw JSON request body, kept as raw bytes so that {{name}}
	// substitution can be applied to the text before the request is sent.
	Body json.RawMessage `json:"body,omitempty"`

	// Status is the expected HTTP response status. If the actual status
	// differs, the Save block is not processed and the run is logged as a
	// failure.
	Status int `json:"status,omitempty"`

	// Save maps a name to a dot-notation JSON path queried against the
	// response body; the extracted value is stored in the global,
	// cross-task substitution dictionary under that name.
	Save map[string]string `json:"save,omitempty"`

	// Timeout is a Go duration string (with the Ego "d" days extension)
	// bounding the endpoint call. Empty means use the configured default.
	Timeout string `json:"timeout,omitempty"`

	// Repeat is "once" (run only at startup) or a duration string (with the
	// Ego "d" days extension) for recurring execution. The interval is
	// measured from when the task last finished running.
	Repeat string `json:"repeat,omitempty"`

	// Path is the absolute path of the file this task was loaded from. Not
	// part of the JSON payload -- needed so DELETE /admin/tasks/{id} can
	// patch the file's "active" field in place.
	Path string `json:"-"`
}

// State is the in-memory execution history for one task. It is never
// written into the task's own JSON file.
type State struct {
	LastRun    time.Time
	LastStatus int
	Success    bool
	Running    bool
}

var (
	registryLock sync.RWMutex
	registry     = map[string]*Task{}
	states       = map[string]*State{}
)

// register adds a task to the registry, keyed by its ID. If a task with the
// same ID is already registered, the new task is rejected: the existing
// task (and true) is returned so the caller can log which file "won".
func register(task *Task) (existing *Task, duplicate bool) {
	registryLock.Lock()
	defer registryLock.Unlock()

	if prior, found := registry[task.ID]; found {
		return prior, true
	}

	registry[task.ID] = task
	states[task.ID] = &State{}

	return nil, false
}

// Tasks returns every registered task, sorted by ID for deterministic
// order. Used internally by the scheduler; reporting (GET /admin/tasks)
// uses Snapshot instead, which reads each task's mutable fields under the
// same lock that protects them.
func Tasks() []*Task {
	registryLock.RLock()
	defer registryLock.RUnlock()

	result := make([]*Task, 0, len(registry))
	for _, task := range registry {
		result = append(result, task)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })

	return result
}

// Lookup returns the task registered under the given ID, if any.
func Lookup(id string) (*Task, bool) {
	registryLock.RLock()
	defer registryLock.RUnlock()

	task, found := registry[id]

	return task, found
}

// setActive clears (or sets) a task's Active flag under the registry lock.
// This is the only way Active is ever mutated after load, so every other
// reader of it (isDue, Snapshot) takes the same lock, and none of them
// race with a concurrent DELETE /admin/tasks/{id}.
func setActive(id string, active bool) {
	registryLock.Lock()
	defer registryLock.Unlock()

	if task, found := registry[id]; found {
		task.Active = active
	}
}

// TaskSummary is a point-in-time, lock-safe snapshot of one task's
// identity, definition fields, and current execution state -- everything
// GET /admin/tasks needs to report, without handing out the live *Task or
// *State pointers themselves.
type TaskSummary struct {
	Description string
	ID          string
	Active      bool
	State
}

// Snapshot returns a lock-safe summary of every registered task, sorted by
// ID.
func Snapshot() []TaskSummary {
	registryLock.RLock()
	defer registryLock.RUnlock()

	result := make([]TaskSummary, 0, len(registry))

	for id, task := range registry {
		summary := TaskSummary{
			Description: task.Description,
			ID:          id,
			Active:      task.Active,
		}

		if state, found := states[id]; found {
			summary.State = *state
		}

		result = append(result, summary)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })

	return result
}
