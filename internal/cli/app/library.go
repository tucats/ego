package app

// Ego ships with a "lib" directory holding its runtime library, its sample
// services, and the web dashboard's assets. Rather than requiring that
// directory to be installed next to the executable, the whole tree is
// compressed into a single ZIP archive at build time and carried inside the
// executable itself. The first time Ego runs on a machine and finds no
// library directory, it unpacks that archive.
//
// Two directives below make that work.
//
// The "go:generate" directive is run by "go generate ./...", which both
// tools/build and tools/build.ps1 do before compiling. It builds and runs the
// zipgo tool from the tools directory, which compresses the lib directory at
// the root of the workspace into lib.zip, right here in this package's
// directory. Note the "--omit" option: if a developer has generated HTTPS
// certificates into their lib directory, those are private keys and must
// never be baked into a shipped executable, so they are excluded by name.
//
// The "go:embed" directive below tells the Go compiler to place the contents
// of that archive into the variable "zipdata" when this package is compiled.
//
// lib.zip is deliberately NOT checked into source control -- it is a build
// product, and .gitignore excludes "*.zip". A fresh clone therefore has no
// lib.zip, and a plain "go build" will fail with:
//
//	pattern lib.zip: no matching files found
//
// The fix is to run "go generate ./..." first, exactly as the build scripts
// do. ("go generate" itself works fine without the archive present, because
// it only scans source files for directives; it does not compile anything.)

//go:generate go run ../../../tools/zipgo/ ../../../lib --output lib.zip --digest --omit https-server.crt,https-server.key

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	// The blank import of "embed" is required by the go:embed directive
	// below. The directive needs the embed package to be linked in, but this
	// file never calls anything from it by name, and Go rejects an unused
	// import, so it is imported for its side effect only -- which is what the
	// underscore means.
	_ "embed"

	// Two different packages here are both called "errors": Go's own, and
	// Ego's. The import alias "goErrors" resolves the collision, following
	// the convention already used in app.go in this same package.
	//
	// Every error this file hands back to its caller is wrapped with
	// errors.New from Ego's package. That is not decoration: main.go's
	// reportError only prints errors that are of Ego's *errors.Error type,
	// and silently exits with a status of zero for anything else. A bare
	// error returned from here would therefore cause Ego to stop with no
	// message and no failing exit code at all.
	goErrors "errors"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

//go:embed lib.zip
var zipdata string

// Permissions used when creating the extracted library.
//
// Directories are created as 0755 -- read, write, and traverse for the owner,
// and read and traverse for everyone else. The "traverse" (execute) bit has
// to be set on a directory for anyone to be able to look inside it, which is
// why the group and other bits are 5 and not 4.
//
// Regular files are created as 0644 -- read and write for the owner, read
// only for everyone else. This is stated explicitly rather than relying on
// os.Create, which asks for 0666 and lets the process umask decide the rest.
const (
	directoryPerm = 0o755
	filePerm      = 0o644
)

// maxEntrySize is the largest single file this code will extract from the
// embedded archive: one gigabyte. The whole library is well under a megabyte,
// so this is not a limit anything legitimate can reach; it exists only so
// that a corrupt or hostile archive cannot ask for an absurd amount of data.
const maxEntrySize = 1 << 30

// LibraryAction is the action routine to suppress library initialization. This
// action is called when the --no-lib-init global option is used when Ego is
// invoked. Note that this is a hidden option not visible in the "help" output.
//
// When the option is present, the sets the "ego.runtime.suppress_library_init"
// config option to true, which prevents Ego from generation a new /lib directory
// if it isn't found during initialization.
//
// Note that this is set as the default value for the setting, which means it
// overrides the persistent setting in the configuration file, if present, but is
// not persisted in the configuration file.
func LibraryAction(c *cli.Context) error {
	settings.SetDefault(defs.SuppressLibraryInitSetting, "true")

	return nil
}

// LibraryInit makes sure the runtime library exists on disk, unpacking the
// copy embedded in this executable if it does not.
//
// The library is considered to need installing when its directory is missing
// altogether, or when it exists but is empty. The "empty" case matters
// because a directory can be left behind by an installation that was
// interrupted -- or simply created by hand by a user -- and if the only test
// were "does this directory exist", Ego would consider itself installed
// forever afterwards and never repair it.
func LibraryInit() error {
	// If initialization is suppressed, we're done.
	if settings.GetBool(defs.SuppressLibraryInitSetting) {
		return nil
	}

	path := libraryPath()

	needed, err := installNeeded(path)
	if err != nil {
		return err
	}

	if !needed {
		ui.Log(ui.AppLogger, "runtime.lib.path", ui.A{
			"path": path})

		return nil
	}

	return installLibraryAtomically(path)
}

// libraryPath returns the directory the runtime library should live in. This
// is the explicitly configured library path if there is one, and otherwise
// the "lib" subdirectory of the Ego installation path.
func libraryPath() string {
	if path := settings.Get(defs.EgoLibPathSetting); path != "" {
		return path
	}

	return filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
}

// installNeeded reports whether the library needs to be unpacked into the
// given path.
func installNeeded(path string) (bool, error) {
	info, err := os.Stat(path)

	// The path does not exist at all, so it certainly needs installing. Note
	// the use of errors.Is rather than a plain comparison: the error returned
	// by os.Stat wraps the underlying reason, and errors.Is looks through
	// those layers of wrapping to ask "is this, at bottom, a does-not-exist
	// error?"
	if goErrors.Is(err, fs.ErrNotExist) {
		return true, nil
	}

	// Any other failure -- most commonly a permission problem on a parent
	// directory -- is reported rather than swallowed. The previous version of
	// this code treated every Stat failure as "missing" and went on to
	// attempt an installation that could not possibly succeed.
	if err != nil {
		ui.Log(ui.AppLogger, "runtime.lib.error", ui.A{
			"error": err})

		return false, errors.New(err)
	}

	// Something is there, but if it is a file rather than a directory then
	// nothing sensible can be done with it, and silently unpacking around it
	// would only produce a more confusing failure later.
	if !info.IsDir() {
		return false, errors.New(fmt.Errorf("runtime library path is not a directory: %s", path))
	}

	empty, err := isEmptyDir(path)
	if err != nil {
		return false, errors.New(err)
	}

	if empty {
		ui.Log(ui.AppLogger, "runtime.lib.empty", ui.A{
			"path": path})
	}

	return empty, nil
}

// isEmptyDir reports whether a directory contains no entries at all.
//
// Rather than reading the whole directory listing, this asks for at most one
// entry. If the directory is empty there is nothing to return, and the read
// reports io.EOF -- which here means "empty", not "something went wrong".
func isEmptyDir(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, errors.New(err)
	}

	defer f.Close()

	_, err = f.Readdirnames(1)
	if goErrors.Is(err, io.EOF) {
		return true, nil
	}

	if err != nil {
		return false, errors.New(err)
	}

	return false, nil
}

// installLibraryAtomically unpacks the embedded library so that the target
// directory either ends up complete or is never created at all.
//
// It does this by unpacking into a temporary directory alongside the target
// and, only once every single file has been written successfully, renaming
// that directory into place. Renaming a directory within one file system is
// a single, indivisible operation as far as other programs are concerned:
// there is no moment at which another process can observe a half-populated
// library. If anything fails partway through, the temporary directory is
// deleted and the next run simply tries again.
//
// This is why the temporary directory is created next to the target rather
// than in the system temporary area: a rename can only be done within a
// single file system, and /tmp is frequently a different one.
func installLibraryAtomically(path string) error {
	parent := filepath.Dir(path)

	if err := os.MkdirAll(parent, directoryPerm); err != nil {
		return errors.New(err)
	}

	// The leading dot in the pattern keeps the half-built directory out of
	// ordinary directory listings while it exists. The trailing star is where
	// os.MkdirTemp splices in random characters, so that two Ego processes
	// starting at the same moment cannot collide.
	staging, err := os.MkdirTemp(parent, ".lib-install-*")
	if err != nil {
		return errors.New(err)
	}

	// This runs however this function exits, including on an error return.
	// After a successful rename the staging directory no longer exists, and
	// os.RemoveAll treats a path that is already gone as success, so there is
	// no need to guard this.
	defer os.RemoveAll(staging)

	ui.Log(ui.AppLogger, "runtime.lib.extract", ui.A{
		"path": path})

	// Unpack with replace set to true. Nothing can already exist inside a
	// directory that was created empty moments ago, so this simply avoids a
	// pointless existence check for every file.
	if err := InstallLibrary(staging, true); err != nil {
		return err
	}

	if err := os.Chmod(staging, directoryPerm); err != nil {
		return errors.New(err)
	}

	// If the target is an existing empty directory, remove it first. On Unix
	// a rename over an empty directory would succeed anyway, but on Windows
	// it fails, and doing this explicitly keeps the behavior the same on
	// every platform.
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		// A failure here is ignored deliberately: it means the directory is
		// no longer empty, which the rename below will detect and handle.
		_ = os.Remove(path)
	}

	if err := os.Rename(staging, path); err != nil {
		// The rename can legitimately fail if another Ego process started at
		// the same time and finished its own installation first. In that case
		// the library is present and correct, which is all this function was
		// asked to achieve, so report success and let the deferred cleanup
		// discard this process's redundant copy.
		if empty, checkErr := isEmptyDir(path); checkErr == nil && !empty {
			ui.Log(ui.AppLogger, "runtime.lib.race", ui.A{
				"path": path})

			return nil
		}

		return errors.New(err)
	}

	return nil
}

// InstallLibrary extracts the embedded library archive into the given
// directory. The files land directly in that directory, so passing
// "/opt/ego/lib" produces "/opt/ego/lib/services/count.ego".
//
// If replace is true, files already present are overwritten; if it is false,
// they are left alone.
//
// Callers that need the installation to be all-or-nothing should use
// installLibraryAtomically instead, which wraps this.
func InstallLibrary(path string, replace bool) error {
	// zipdata holds the embedded archive. It is a string rather than a byte
	// slice on purpose. A string's bytes are immutable, so the Go linker can
	// place them in the executable's read-only region, where the operating
	// system can share one copy between every running process and can never
	// be dirtied by a stray write. A []byte would have to go in a writable
	// region instead.
	//
	// For the same reason the string is handed to strings.NewReader rather
	// than being converted with []byte(zipdata): the conversion has to copy
	// every byte, precisely because the result is allowed to be modified.
	r, err := zip.NewReader(strings.NewReader(zipdata), int64(len(zipdata)))
	if err != nil {
		return errors.New(err)
	}

	for _, f := range r.File {
		if err := extractFile(f, path, replace); err != nil {
			return err
		}
	}

	return nil
}

// extractFile writes a single entry from the archive into the output
// directory.
func extractFile(f *zip.File, root string, replace bool) error {
	// Work out where this entry belongs, rejecting anything that would land
	// outside the output directory.
	path, err := safeJoin(root, f.Name)
	if err != nil {
		return err
	}

	ui.Log(ui.AppLogger, "runtime.lib.extract.item", ui.A{
		"name": f.Name,
		"path": path})

	if f.FileInfo().IsDir() {
		// Note the explicit nil test rather than the shorter
		// "return errors.New(os.MkdirAll(...))". errors.New returns a typed
		// pointer, and a nil pointer stored in Go's error interface is NOT
		// equal to nil -- the interface still records which type the missing
		// value would have had. Wrapping a successful (nil) result would
		// therefore hand the caller something that looks like a failure.
		if err := os.MkdirAll(path, directoryPerm); err != nil {
			return errors.New(err)
		}

		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), directoryPerm); err != nil {
		return errors.New(err)
	}

	// If the file exists and we are not replacing, do nothing.
	if !replace {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}

	rc, err := f.Open()
	if err != nil {
		return errors.New(err)
	}

	defer rc.Close()

	// os.OpenFile with these flags is what os.Create does, except that the
	// permissions are stated explicitly here. O_TRUNC discards any existing
	// contents, which matters in the replace case.
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, filePerm)
	if err != nil {
		return errors.New(err)
	}

	// Copy the contents, but never write more bytes than the archive's index
	// said this entry contains.
	//
	// A ZIP file records each entry's uncompressed size up front, so how much
	// data should appear here is known before any of it is read. Compressed
	// data that expands to far more than its stated size is how a "zip bomb"
	// works: a small archive that fills the disk when unpacked. Copying blind
	// with io.Copy would write whatever came out. io.CopyN stops at the
	// declared size instead.
	//
	// io.CopyN reports io.EOF when the source ends early, which for a
	// correctly formed archive cannot happen and so is treated as the error
	// it is.
	//
	// The declared size is checked against a ceiling first. A ZIP entry
	// records its size in 64 bits, which can describe a number far larger
	// than any file in this library will ever be -- and larger than the
	// signed 64-bit count io.CopyN accepts, so converting an absurd value
	// would wrap around to a negative number and copy nothing at all.
	if f.UncompressedSize64 > maxEntrySize {
		return errors.New(fmt.Errorf("library archive entry %q declares an implausible size of %d bytes",
			f.Name, f.UncompressedSize64))
	}

	if _, err := io.CopyN(out, rc, int64(f.UncompressedSize64)); err != nil { //nolint:gosec // bounded by maxEntrySize just above
		out.Close()

		return errors.New(err)
	}

	// As above, only wrap a genuine failure; wrapping a nil error would
	// produce a non-nil error interface holding a nil pointer.
	if err := out.Close(); err != nil {
		return errors.New(err)
	}

	return nil
}

// safeJoin converts an entry name from inside the archive into a path within
// the output directory, and refuses names that would escape it.
//
// Names inside a ZIP archive are just strings, and nothing in the format
// stops one from being an absolute path such as "/etc/passwd", or from
// climbing out of the output directory with "../../etc/passwd". A program
// that joins such a name onto its output directory without checking will
// happily overwrite files anywhere the user can write. That mistake is
// common enough to have its own name -- "zip slip".
//
// Ego builds this particular archive itself, so today's contents are known to
// be safe. The check is here anyway because InstallLibrary is exported, and
// because the cost of being wrong about "this input is trusted" is very high
// compared to the cost of one comparison per file.
func safeJoin(root, name string) (string, error) {
	// Entry names in a ZIP file always use forward slashes, whatever machine
	// wrote the archive. filepath.FromSlash converts them to whatever this
	// machine uses, which is a no-op everywhere except Windows.
	//
	// Doing this conversion is also what makes archives written by the old
	// version of the zipgo tool on Windows safe to reject: those contain
	// backslashes in their names, which are an ordinary filename character on
	// Unix and so cannot climb anywhere, and which the check below catches on
	// Windows.
	clean := filepath.FromSlash(name)

	if clean == "" || filepath.IsAbs(clean) || strings.HasPrefix(name, "/") {
		return "", errors.New(fmt.Errorf("unsafe path in library archive: %q", name))
	}

	path := filepath.Join(root, clean)

	// filepath.Join has already collapsed any ".." components, so asking for
	// the path of the result relative to root tells us where it truly ended
	// up. If that relative path itself starts with "..", the entry escaped.
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", errors.New(fmt.Errorf("unsafe path in library archive: %q", name))
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", errors.New(fmt.Errorf("unsafe path in library archive: %q", name))
	}

	return path, nil
}
