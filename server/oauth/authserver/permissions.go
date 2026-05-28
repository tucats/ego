package authserver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

const (
	// requiredDirMode is the only mode allowed on the oauth directory:
	// owner read/write/execute only.  Group and world bits must be clear.
	requiredDirMode os.FileMode = 0700

	// requiredFileMode is the only mode allowed on secret files (PEM key,
	// client registry): owner read/write only.  Group and world bits must
	// be clear.
	requiredFileMode os.FileMode = 0600
)

// ensureOAuthDir guarantees that the OAuth2 AS working directory exists and
// has the required permissions (0700 — owner only). If the directory is absent
// it is created. If its permissions are wrong the function attempts to correct
// them with chmod. If the chmod fails an error is returned and the caller
// should abort server startup.
//
// The directory path is {EGO_PATH}/lib/oauth unless overridden by explicit
// settings for the key file or client file (in which case those explicit paths
// carry their own directory, and callers need only validate those files).
func ensureOAuthDir() (string, error) {
	egoPath := settings.Get(defs.EgoPathSetting)
	dir := filepath.Join(egoPath, defaultOAuthSubDir)

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		// Create the directory with the required permissions.
		if mkErr := os.MkdirAll(dir, requiredDirMode); mkErr != nil {
			return dir, fmt.Errorf("creating OAuth2 directory %s: %w", dir, mkErr)
		}

		ui.Log(ui.ServerLogger, "oauth.as.dir.created", ui.A{"path": dir})

		return dir, nil
	}

	if err != nil {
		return dir, fmt.Errorf("accessing OAuth2 directory %s: %w", dir, err)
	}

	if !info.IsDir() {
		return dir, fmt.Errorf("OAuth2 path %s exists but is not a directory", dir)
	}

	// Verify the directory mode and correct it if necessary.
	if err := ensureMode(dir, info.Mode().Perm(), requiredDirMode, true); err != nil {
		return dir, err
	}

	return dir, nil
}

// ensureFilePermissions checks that the file at path has exactly the required
// permissions (0600). If the permissions are wrong the function attempts a
// chmod. Returns an error if the chmod fails or if the path cannot be stat'd.
//
// A missing file is not an error — the caller is responsible for detecting
// absence and creating the file; permissions on a nonexistent file are moot.
func ensureFilePermissions(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil // not present yet — will be created with correct perms
	}

	if err != nil {
		return fmt.Errorf("accessing OAuth2 file %s: %w", path, err)
	}

	return ensureMode(path, info.Mode().Perm(), requiredFileMode, false)
}

// ensureMode checks whether perm matches required and, if not, attempts a
// chmod. If isDir is true the error messages say "directory"; otherwise "file".
// Returns an error only when chmod itself fails.
func ensureMode(path string, perm, required os.FileMode, isDir bool) error {
	kind := "file"
	if isDir {
		kind = "directory"
	}

	// Mask to the lower 9 permission bits so we don't compare setuid / sticky bits.
	if perm&0777 == required {
		return nil
	}

	ui.Log(ui.ServerLogger, "oauth.as.permissions.fixing", ui.A{
		"path": path,
		"kind": kind,
		"have": fmt.Sprintf("%04o", perm&0777),
		"want": fmt.Sprintf("%04o", required),
	})

	if chmodErr := os.Chmod(path, required); chmodErr != nil {
		return fmt.Errorf(
			"OAuth2 %s %s has insecure permissions %04o and chmod failed: %w",
			kind, path, perm&0777, chmodErr,
		)
	}

	ui.Log(ui.ServerLogger, "oauth.as.permissions.fixed", ui.A{
		"path": path,
		"kind": kind,
		"mode": fmt.Sprintf("%04o", required),
	})

	return nil
}
