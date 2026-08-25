package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/util"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

const dictionaryName = "dictionary.json"

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
//
// Before scanning, it preloads the global save/substitution dictionary
// (save.go) with SESSIONID, this server instance's UUID -- available to
// any task's endpoint/parameters/body as {{SESSIONID}} without needing a
// "save" step of its own to obtain it.
func LoadAll() error {
	setSaved("SESSIONID", defs.InstanceID)

	loadDictionary()

	names, dir, err := listTaskFiles()
	if err != nil {
		return err
	}

	ui.Log(tasksLogger, "tasks.init.start", ui.A{"path": dir, "count": len(names)})

	for _, name := range names {
		loadOne(filepath.Join(dir, name))
	}

	ui.Log(tasksLogger, "tasks.init.complete", ui.A{"loaded": len(Tasks()), "found": len(names)})

	return nil
}

// listTaskFiles enforces the tasks directory's permissions, then returns
// the sorted, deterministic list of task filenames within it (hidden files
// -- the sidecar state file -- and anything not ending in .json are
// excluded). This also specificcally excludes "dictionary.json" which
// holds data to be preloaded into the substitution dictionary.
//
// This function is shared by LoadAll (startup) and Reload (POST
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

		// Always skip over the reserved dictionary file, which doesn't contain a task
		// but instead is a preload for the dictionary.
		if name == dictionaryName {
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

	if task.Interval != "" {
		if _, err := util.ParseDuration(task.Interval); err != nil {
			return errors.New(errors.ErrTasksInvalidField).Context("interval: " + task.Interval)
		}
	} else if task.Count != 0 && task.Count != 1 {
		// No interval means one-shot: it only ever gets one run, so any
		// Count other than the implied 1 (or the equivalent explicit 1)
		// can never be satisfied. Rather than silently ignoring a
		// confusing value, reject it as an ambiguous task definition.
		return errors.New(errors.ErrTasksInvalidField).Context("count: requires interval when count is not 1")
	}

	if task.After != "" {
		if _, err := util.ParseDuration(task.After); err != nil {
			return errors.New(errors.ErrTasksInvalidField).Context("after: " + task.After)
		}
	}

	if task.Timeout != "" {
		if _, err := util.ParseDuration(task.Timeout); err != nil {
			return errors.New(errors.ErrTasksInvalidField).Context("timeout: " + task.Timeout)
		}
	}

	for i, check := range task.Tests {
		prefix := "tests[" + strconv.Itoa(i) + "]"

		if check.Name == "" {
			return errors.New(errors.ErrTasksMissingField).Context(prefix + ".name")
		}

		if check.Query == "" {
			return errors.New(errors.ErrTasksMissingField).Context(prefix + ".query")
		}

		if !validCheckOperators[check.Operator] {
			return errors.New(errors.ErrTasksInvalidField).Context(prefix + ".op: " + check.Operator)
		}

		if check.Operator == "len" {
			if _, err := strconv.Atoi(check.Value); err != nil {
				return errors.New(errors.ErrTasksInvalidField).Context(prefix + ".value: " + check.Value)
			}
		}
	}

	return nil
}

// loadDictionary loads the "dictionary.json" file in the lib directory
// path, and preloads its contents into the global substitution dictionary
// used by tasks. If it fails, it logs it but does not stop the rest
// of the task manager.
func loadDictionary() {
	name := filepath.Join(Directory(), dictionaryName)

	b, err := ui.ReadJSONFile(name)
	if err != nil {
		ui.Log(ui.TaskLogger, "tasks.dictionary.err", ui.A{
			"name":  name,
			"error": err.Error(),
		})

		return
	}

	items := map[string]string{}

	err = json.Unmarshal(b, &items)
	if err != nil {
		ui.Log(ui.TaskLogger, "tasks.dictionary.err", ui.A{
			"name":  name,
			"error": err.Error(),
		})

		return
	}

	ui.Log(ui.TaskLogger, "tasks.dictionary.load", ui.A{
		"name": name,
	})

	// Load the items from the JSON into the dictionary
	for key, value := range items {
		// Handle some special cases
		switch strings.ToLower(value) {
		// The current server instance UUID
		case "$server":
			value = defs.InstanceID

		// Make a random unique string of characters
		case "$hash":
			value = egostrings.Gibberish(uuid.New())

		// Make a UUID string value.
		case "$uuid":
			value = uuid.New().String()
		}

		ui.Log(ui.TaskLogger, "tasks.dictionary.define", ui.A{
			"key":   key,
			"value": value,
		})

		setSaved(key, value)
	}
}
