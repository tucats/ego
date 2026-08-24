package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/util"
)

// validMethods is the set of HTTP methods a task may specify.
var validMethods = map[string]bool{
	"GET":    true,
	"POST":   true,
	"PUT":    true,
	"PATCH":  true,
	"DELETE": true,
}

// Directory returns the lib/tasks directory path, honoring the same
// ego.runtime.path.lib override used elsewhere in the codebase to relocate
// the lib/ tree.
func Directory() string {
	root := settings.Get(defs.EgoLibPathSetting)
	if root == "" {
		root = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	return filepath.Join(root, "tasks")
}

// LoadAll scans the tasks directory, enforces the directory's and each
// file's permissions, parses every task definition, and populates the
// in-memory registry. A file that fails permission enforcement or fails to
// parse or validate is skipped and logged; it does not stop the rest of the
// directory from loading. Returns an error only for a directory-level
// problem (missing and can't be created, or permissions that can't be
// corrected), since that affects every task, not just one file.
func LoadAll() error {
	names, dir, err := listTaskFiles()
	if err != nil {
		return err
	}

	for _, name := range names {
		loadOne(filepath.Join(dir, name))
	}

	return nil
}

// listTaskFiles enforces the tasks directory's permissions, then returns
// the sorted, deterministic list of task filenames within it (hidden files
// -- the sidecar state file -- and anything not ending in .json are
// excluded). Shared by LoadAll (startup) and Reload (POST
// /admin/tasks/@reload), so both scan the directory identically.
func listTaskFiles() (names []string, dir string, err error) {
	dir = Directory()

	if err := ensureDirPermissions(dir); err != nil {
		return nil, dir, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, dir, errors.New(errors.ErrTasksDirAccess).Context(err.Error())
	}

	names = make([]string, 0, len(entries))

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}

		names = append(names, name)
	}

	// Sorted, deterministic order: when two files declare the same task id,
	// the first one processed (alphabetically by filename) wins.
	sort.Strings(names)

	return names, dir, nil
}

// parseTaskFile enforces one file's permissions, reads and parses it
// (stripping comment lines), validates the result, and stamps its source
// path onto the returned Task. Shared by LoadAll and Reload.
func parseTaskFile(path string) (*Task, error) {
	if err := ensureFilePermissions(path); err != nil {
		return nil, err
	}

	b, err := ui.ReadJSONFile(path)
	if err != nil {
		return nil, err
	}

	var task Task

	if err := json.Unmarshal(b, &task); err != nil {
		return nil, err
	}

	if err := validateTask(&task); err != nil {
		return nil, err
	}

	task.Path = path

	return &task, nil
}

func loadOne(path string) {
	task, err := parseTaskFile(path)
	if err != nil {
		ui.Log(tasksLogger, "tasks.load.skipped", ui.A{"path": path, "error": err.Error()})

		return
	}

	if existing, duplicate := register(task); duplicate {
		ui.Log(tasksLogger, "tasks.load.duplicate", ui.A{"id": task.ID, "path": path, "existing": existing.Path})

		return
	}

	ui.Log(tasksLogger, "tasks.loaded", ui.A{"id": task.ID, "description": task.Description, "path": path})
}

// validateTask checks the required fields and normalizes the method name.
// It intentionally does not check that task.User names a real user -- that
// is deferred to first dispatch (see docs/internals/TASKS.md), since the
// auth subsystem's user database is a separate, independently-initialized
// service and a load-time check here would only be able to catch the
// common case anyway (users can be added or removed after the server
// starts).
func validateTask(task *Task) error {
	switch {
	case task.ID == "":
		return errors.New(errors.ErrTasksMissingField).Context("id")
	case task.ID == ReloadTaskID:
		return errors.New(errors.ErrTasksInvalidField).Context("id: " + ReloadTaskID + " is reserved")
	case task.User == "":
		return errors.New(errors.ErrTasksMissingField).Context("user")
	case task.Method == "":
		return errors.New(errors.ErrTasksMissingField).Context("method")
	case task.Endpoint == "":
		return errors.New(errors.ErrTasksMissingField).Context("endpoint")
	}

	task.Method = strings.ToUpper(task.Method)
	if !validMethods[task.Method] {
		return errors.New(errors.ErrTasksInvalidField).Context("method: " + task.Method)
	}

	if task.Repeat != "" && task.Repeat != "once" {
		if _, err := util.ParseDuration(task.Repeat); err != nil {
			return errors.New(errors.ErrTasksInvalidField).Context("repeat: " + task.Repeat)
		}
	}

	if task.Timeout != "" {
		if _, err := util.ParseDuration(task.Timeout); err != nil {
			return errors.New(errors.ErrTasksInvalidField).Context("timeout: " + task.Timeout)
		}
	}

	return nil
}
