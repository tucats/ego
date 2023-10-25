package app

// If we are generating, create the zip archive data from the lib directory
// found at the root level of the workspace.
//go:generate zipgo ../../lib --output unzip.go --package app --data --digest lib.checksum

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/defs"
)

// LibraryAction is the action routine to suppress library initialization.
func LibraryAction(c *cli.Context) error {
	settings.SetDefault(defs.SuppressLibraryInitSetting, "true")

	return nil
}

// LibraryInit initializes the library by unzipping the embedded zip file
// into the active default library path. If the lib directory already
// exists, then no action is taken.
func LibraryInit() error {
	// If initialization is suppressed, we're done.
	if settings.GetBool(defs.SuppressLibraryInitSetting) {
		return nil
	}

	// If the library directory exists, we're done.
	path := settings.Get(defs.EgoLibPathSetting)

	if _, err := os.Stat(path); err == nil {
		return nil
	}

	// Unzip the embedded zip file into the library directory.
	// The replace option is set to false, so we won't replace
	// existing files found in the output directory.
	return InstallLibrary(settings.Get(defs.EgoLibPathSetting), false)
}

// InstallLibrary extracts the zip data to the file system. The path specifies the
// directory to extract the files to. If replace is true, existing files are
// replaced in the output directory.
func InstallLibrary(path string, replace bool) error {
	// Decode the zip data.
	data, err := base64.StdEncoding.DecodeString(zipdata)
	if err != nil {
		return err
	}

	// Open the zip archive.
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

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

	// Create the file in the file system.
	path = filepath.Join(path, f.Name)
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