package tasks

import (
	"fmt"
	"os"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/errors"
)

const (
	// requiredDirMode is the only mode allowed on the tasks directory:
	// owner read/write/execute only. Group and world bits must be clear.
	requiredDirMode os.FileMode = 0700

	// requiredFileMode is the only mode allowed on a task file: owner
	// read/write only. Group and world bits must be clear, since a task
	// file can contain credentials or other sensitive request data.
	requiredFileMode os.FileMode = 0600
)

// ensureDirPermissions guarantees that the tasks directory exists and has
// the required permissions (0700). If the directory is absent it is
// created. If its permissions are wrong, it attempts to correct them with
// chmod. Unlike ensureFilePermissions, a directory that can't be corrected
// is fatal to the whole task subsystem: a world-readable directory can leak
// the existence and names of task files even when their contents are
// protected.
func ensureDirPermissions(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if mkErr := os.MkdirAll(dir, requiredDirMode); mkErr != nil {
			return errors.New(errors.ErrTasksDirCreate).Context(fmt.Sprintf("%s: %v", dir, mkErr))
		}

		ui.Log(tasksLogger, "tasks.dir.created", ui.A{"path": dir})

		return nil
	}

	if err != nil {
		return errors.New(errors.ErrTasksDirAccess).Context(fmt.Sprintf("%s: %v", dir, err))
	}

	if !info.IsDir() {
		return errors.New(errors.ErrTasksDirNotDir).Context(dir)
	}

	return ensureMode(dir, info.Mode().Perm(), requiredDirMode, true)
}

// ensureFilePermissions corrects a task file's permissions to 0600 if
// necessary. A file that cannot be corrected is not fatal to the whole
// subsystem -- the caller skips just that one file.
func ensureFilePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return errors.New(errors.ErrTasksFileAccess).Context(fmt.Sprintf("%s: %v", path, err))
	}

	return ensureMode(path, info.Mode().Perm(), requiredFileMode, false)
}

// ensureMode checks whether perm matches required and, if not, attempts a
// chmod. Returns an error only when the chmod itself fails.
func ensureMode(path string, perm, required os.FileMode, isDir bool) error {
	kind := "file"
	if isDir {
		kind = "directory"
	}

	// Mask to the lower 9 permission bits so we don't compare setuid/sticky bits.
	if perm&0777 == required {
		return nil
	}

	ui.Log(tasksLogger, "tasks.permissions.fixing", ui.A{
		"path": path,
		"kind": kind,
		"have": fmt.Sprintf("%04o", perm&0777),
		"want": fmt.Sprintf("%04o", required),
	})

	if chmodErr := os.Chmod(path, required); chmodErr != nil {
		return errors.New(errors.ErrTasksPermissionsInsecure).Context(
			fmt.Sprintf("%s %s (mode %04o): %v", kind, path, perm&0777, chmodErr),
		)
	}

	ui.Log(tasksLogger, "tasks.permissions.fixed", ui.A{
		"path": path,
		"kind": kind,
		"mode": fmt.Sprintf("%04o", required),
	})

	return nil
}
