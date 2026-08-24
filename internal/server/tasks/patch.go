package tasks

import (
	"os"
	"strconv"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

// applyTaskPatch returns a copy of task with every field named in patch
// overwritten by the corresponding value, leaving every other field of
// task (and all of task's execution State, which lives separately -- see
// defs.go) untouched. It does not validate the result or touch the task
// file; see patchTask for that.
func applyTaskPatch(task *Task, patch defs.TaskPatchRequest) Task {
	updated := *task

	if patch.Active != nil {
		updated.Active = *patch.Active
	}

	if patch.Interval != nil {
		updated.Interval = *patch.Interval
	}

	if patch.Count != nil {
		updated.Count = *patch.Count
	}

	if patch.After != nil {
		updated.After = *patch.After
	}

	return updated
}

// taskPatchFileFields converts patch into the ordered list of
// jsonFieldPatch entries patchJSONFields needs to rewrite a task file --
// only the fields patch actually sets are included, in the fixed order
// active/interval/count/after so a patch that inserts more than one
// missing field at once produces a deterministic file layout.
//
// Task.Active is stored on disk as a JSON string ("true"/"false" -- see
// the `json:"active,string"` tag on Task.Active), not a bare boolean, so
// its Value here is the stringified form; patchJSONFields marshals
// whatever Value it's given as-is, so a Go string becomes a quoted JSON
// string.
func taskPatchFileFields(patch defs.TaskPatchRequest) []jsonFieldPatch {
	fields := make([]jsonFieldPatch, 0, 4)

	if patch.Active != nil {
		fields = append(fields, jsonFieldPatch{Key: "active", Value: strconv.FormatBool(*patch.Active)})
	}

	if patch.Interval != nil {
		fields = append(fields, jsonFieldPatch{Key: "interval", Value: *patch.Interval})
	}

	if patch.Count != nil {
		fields = append(fields, jsonFieldPatch{Key: "count", Value: *patch.Count})
	}

	if patch.After != nil {
		fields = append(fields, jsonFieldPatch{Key: "after", Value: *patch.After})
	}

	return fields
}

// patchTask validates patch against task's other fields (the same rules
// load-time validation applies -- see validateTask -- so a PATCH can never
// leave a task file in a state that would fail to load), rewrites the
// fields it changes into the task's own file (preserving every comment and
// all other formatting, via patchJSONFields/patchTaskFile), and
// re-registers the result with the running registry the same way Reload
// updates one already-loaded task: the file is re-parsed from disk and
// swapped in via upsert, so the task's State (LastRun/Success/RunCount/...)
// survives the edit untouched, and any run already in flight keeps running
// against the definition it started with.
//
// task itself is never mutated; on success, the newly-registered *Task is
// returned.
func patchTask(task *Task, patch defs.TaskPatchRequest) (*Task, error) {
	candidate := applyTaskPatch(task, patch)

	if err := validateTask(&candidate); err != nil {
		return nil, err
	}

	fileFields := taskPatchFileFields(patch)
	if len(fileFields) == 0 {
		return task, nil
	}

	if err := patchTaskFile(task.Path, fileFields); err != nil {
		return nil, err
	}

	reparsed, err := parseTaskFile(task.Path)
	if err != nil {
		// The file this function itself just wrote no longer parses or
		// validates. Since the same candidate was already validated above
		// before any write happened, this should only be reachable if
		// something else modified the file concurrently between the write
		// and this re-read -- log it distinctly from an ordinary
		// load-time skip (tasks.load.skipped), since here it means a
		// just-accepted PATCH's result could not be confirmed.
		ui.Log(tasksLogger, "tasks.patch.reparse.error", ui.A{"id": task.ID, "path": task.Path, "error": err.Error()})

		return nil, err
	}

	upsert(reparsed)

	return reparsed, nil
}

// patchTaskFile rewrites the given fields of the task file at path in
// place, preserving comments and all other formatting (see
// patchJSONFields), while keeping the file's existing permissions.
func patchTaskFile(path string, fields []jsonFieldPatch) error {
	info, err := os.Stat(path)
	if err != nil {
		return errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	original, err := os.ReadFile(path)
	if err != nil {
		return errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	patched, err := patchJSONFields(original, fields)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, patched, info.Mode().Perm()); err != nil {
		return errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	return nil
}
