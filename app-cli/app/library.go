package app

// If we are building, the "go:generate" operation causes the Go compiler phase
// to run the specified command. This command compiles and builds the zipgo tool
// found in the tools directory of the workspace. This command create the zip a
// rchive data from the lib directory found at the root level of the workspace.
//
// Note that if the lib directory contains https certificates, they will be
// omitted from the zip file.

//go:generate go run ../../tools/zipgo/ ../../lib --output unzip.go --package app --data --digest  --omit https-server.crt,https-server.key

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

// LibraryAction is the action routine to suppress library initialization. This
// action is called when the --no-lib-init global option is used when Ego is
// invoked. Note that this is a hidden option not visible in the "help" output.
//
// When the option is presetn, the sets the "ego.runtime.suppress_library_init"
// config option to true, which prevents Ego from generation a new /lib directory
// if it isn't found during initialization.
//
// Note that this is set as the default value for the setting, which means it
// overrides the persisten setting in the configuration file, if present, but is
// not persisted in the configuration file.
func LibraryAction(c *cli.Context) error {
	settings.SetDefault(defs.SuppressLibraryInitSetting, "true")

	return nil
}

// LibraryInit initializes the library by unzipping the embedded zip file
// into the active default library path. If the lib directory already
// exists, then no action is taken.
func LibraryInit() error {
	var err error

	// If initialization is suppressed, we're done.
	if settings.GetBool(defs.SuppressLibraryInitSetting) {
		return nil
	}

	// If the library directory exists, we're done.
	path := settings.Get(defs.EgoLibPathSetting)
	if path == "" {
		path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	if _, err = os.Stat(path); err == nil {
		ui.Log(ui.AppLogger, "Runtime library found at %s", path)

		return nil
	}

	ui.Log(ui.AppLogger, "Attempt to access library failed, %v", err)

	// Unzip the embedded zip file into the library directory.
	// The replace option is set to false, so we won't replace
	// existing files found in the output directory.
	return InstallLibrary(path, false)
}

// InstallLibrary extracts the zip data to the file system. The path specifies the
// directory to extract the files to. If replace is true, existing files are
// replaced in the output directory.
func InstallLibrary(path string, replace bool) error {
	// Open the zip archive.
	r, err := zip.NewReader(bytes.NewReader(zipdata), int64(len(zipdata)))
	if err != nil {
		return err
	}

	ui.Log(ui.AppLogger, "Extracting library to %s", path)

	// Extract the files in the archive.
	for _, f := range r.File {
		if err := extractFile(f, path, replace); err != nil {
			return err
		}
	}

	return nil
}

// extractFile extracts a single file from the zip archive.
func extractFile(f *zip.File, path string, replace bool) error {
	// Open the file in the archive.
	rc, err := f.Open()
	if err != nil {
		return err
	}

	defer rc.Close()

	// Create the file in the file system. Make the path from the archive
	// be relative, and then strip off any directory prefix components.
	name := strings.TrimPrefix(f.Name, "/")
	for strings.HasPrefix(name, "../") {
		name = name[3:]
	}

	// If the path already ends in the "lib" name, strip it off
	path = strings.TrimSuffix(strings.TrimSuffix(path, "/"), defs.LibPathName)

	path = filepath.Join(path, name)
	ui.Log(ui.AppLogger, "Extracting %s to %s", name, path)

	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		// If the file exists and we are not replacing, do nothing.
		if _, err := os.Stat(path); !replace && err == nil {
			return nil
		}

		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()

		// Copy the file contents.
		if _, err := io.Copy(f, rc); err != nil {
			return err
		}
	}

	return nil
}
