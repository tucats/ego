package tasks

import (
	"path/filepath"

	"github.com/tucats/ego/internal/cli/ui"
)

// ReloadTaskID is the reserved task id that triggers a directory rescan
// (POST /admin/tasks/@reload) instead of running a specific task. No real
// task may use this id -- validateTask rejects it at load time, the same
// way other "@name" pseudo-identifiers are reserved elsewhere (@sql,
// @permissions, @metadata, ...).
const ReloadTaskID = "@reload"

// ReloadResult summarizes what one directory rescan changed.
type ReloadResult struct {
	Total   int
	New     int
	Updated int
	Removed int
}

// Reload re-scans the tasks directory and merges what it finds into the
// existing registry, without restarting the server:
//
//   - A file whose id is new is added.
//   - A file whose id is already registered replaces that task's
//     definition (method, endpoint, body, repeat, active, ...) in place,
//     but keeps its execution history (last run, last status, success) --
//     see upsert.
//   - A previously-registered task whose file is no longer present in the
//     directory is removed.
//   - A file that fails permission enforcement, parsing, or validation is
//     skipped and logged, exactly like LoadAll -- it does not stop the
//     rest of the directory from reloading. Critically, its *file still
//     being present* means the task it used to define (if any) is NOT
//     removed by the "no longer present" rule above: a bad edit leaves
//     the task running with its last-known-good definition rather than
//     deleting it out from under the scheduler.
//   - Two files claiming the same id in the same reload pass: the first
//     one processed (alphabetically by filename) wins, matching LoadAll.
//
// This is what lets an admin add, edit, or reactivate (hand-editing
// "active" back to true) a task without stopping the server.
func Reload() (ReloadResult, error) {
	names, dir, err := listTaskFiles()
	if err != nil {
		return ReloadResult{}, err
	}

	present := make(map[string]bool, len(names))
	for _, name := range names {
		present[filepath.Join(dir, name)] = true
	}

	var result ReloadResult

	seen := make(map[string]string, len(names))

	for _, name := range names {
		path := filepath.Join(dir, name)

		task, err := parseTaskFile(path)
		if err != nil {
			ui.Log(tasksLogger, "tasks.load.skipped", ui.A{"path": path, "error": err.Error()})

			continue
		}

		if existingPath, duplicate := seen[task.ID]; duplicate {
			ui.Log(tasksLogger, "tasks.load.duplicate", ui.A{"id": task.ID, "path": path, "existing": existingPath})

			continue
		}

		seen[task.ID] = path
		result.Total++

		if upsert(task) {
			result.New++

			ui.Log(tasksLogger, "tasks.loaded", ui.A{"id": task.ID, "description": task.Description, "path": path})
		} else {
			result.Updated++

			ui.Log(tasksLogger, "tasks.reload.updated", ui.A{"id": task.ID, "description": task.Description, "path": path})
		}
	}

	for _, id := range removeMissing(present) {
		result.Removed++

		ui.Log(tasksLogger, "tasks.reload.removed", ui.A{"id": id})
	}

	return result, nil
}
