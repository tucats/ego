package tasks

import (
	"os"
	"regexp"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/errors"
)

// activeFieldPattern matches a JSON "active" key and its quoted string
// value (the task file format always quotes it, e.g. "active": "true"),
// capturing everything up to and including the colon and whitespace so the
// replacement can drop in a new value without disturbing indentation.
var activeFieldPattern = regexp.MustCompile(`("active"\s*:\s*)"[^"]*"`)

// deactivateFile rewrites a task file's "active" field to "false" in
// place, preserving every comment and all other formatting -- a full JSON
// re-marshal would silently drop any comments the author wrote (see
// docs/internals/TASKS.md). The file's existing permissions are preserved.
//
// If the field can't be found in the raw text (a task file with no
// explicit "active" field, which already defaults to inactive on load),
// the file is left untouched: there's nothing to correct on disk, since
// the next load will see the same "never active" result either way.
func deactivateFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	original, err := os.ReadFile(path)
	if err != nil {
		return errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	if !activeFieldPattern.Match(original) {
		ui.Log(tasksLogger, "tasks.deactivate.field.missing", ui.A{"path": path})

		return nil
	}

	patched := activeFieldPattern.ReplaceAll(original, []byte(`${1}"false"`))

	if err := os.WriteFile(path, patched, info.Mode().Perm()); err != nil {
		return errors.New(errors.ErrTasksFileAccess).Context(err.Error())
	}

	return nil
}
