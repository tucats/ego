package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// addTree adds a file, or an entire directory tree, to the zip archive.
//
// The "root" argument is the path the user named on the command line. The
// "current" argument is the path we have recursed down to so far; on the
// first call the two are the same.
//
// The names stored inside the archive are always relative to root, and are
// always written with forward slashes. That matters for two reasons:
//
//   - The ZIP file format specification requires forward slashes in entry
//     names, regardless of which operating system created the file. Go's
//     filepath.Join uses the *host* separator, which is a backslash on
//     Windows, so an archive built on Windows with host separators would be
//     malformed and would not extract correctly anywhere.
//
//   - Storing names relative to root means the archive contains clean names
//     such as "services/count.ego". Previously the archive stored whatever
//     path the tool was invoked with, which for the "go:generate" directive
//     in internal/cli/app/library.go was literally "../../../lib/services/
//     count.ego". The code that extracted the archive then had to guess at
//     how to undo that, which was the source of several bugs.
func addTree(w *zip.Writer, root, current string) error {
	info, err := os.Stat(current)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return addDir(w, root, current)
	}

	return addFile(w, root, current)
}

// addDir adds every file in a directory, and recursively every file in its
// subdirectories, to the zip archive.
func addDir(w *zip.Writer, root, current string) error {
	if logging {
		fmt.Println(current + "/")
	}

	// os.ReadDir is used here rather than the older os.File.Readdir because
	// os.ReadDir guarantees the entries come back sorted by file name. The
	// older call returns entries in whatever order the file system happens
	// to hand them over, which differs between machines and between file
	// systems. Because both the compressed bytes and the change-detection
	// digest depend on the order the files are visited, an unsorted walk
	// meant two developers could build byte-for-byte different archives
	// from an identical source tree. Sorting makes the output reproducible.
	entries, err := os.ReadDir(current)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(current, entry.Name())

		if entry.IsDir() {
			err = addDir(w, root, path)
		} else {
			err = addFile(w, root, path)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// addFile adds a single file to the zip archive. If the file's name appears
// in the omit list, it is silently skipped instead.
func addFile(w *zip.Writer, root, path string) error {
	// Skip files that are in the omit list. Note that the list is matched
	// against the base name only, so "--omit README.md" omits every file
	// called README.md anywhere in the tree, not one specific one.
	if omit[filepath.Base(path)] {
		if logging {
			fmt.Println(path, "(omitted)")
		}

		return nil
	}

	// Read the whole file into memory exactly once. The previous version of
	// this code opened the file with os.Open to feed the digest, and then
	// separately called os.ReadFile to feed the compressor, so every file
	// was read from disk twice. It also registered its "defer file.Close()"
	// after an error return, which leaked the open file handle whenever the
	// digest step failed. Reading once into a byte slice avoids both
	// problems, and the files in this tree are small enough that holding one
	// of them in memory at a time costs nothing.
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Work out the name to store inside the archive: relative to the root
	// the user named, and using forward slashes. See addTree's comment.
	name, err := archiveName(root, path)
	if err != nil {
		return err
	}

	// If we are computing a digest to detect source changes, fold this file
	// into it now, using the archive-relative name so that the digest does
	// not change merely because the tool was invoked from a different
	// working directory.
	if digest {
		addFileToDigest(name, data)
	}

	if logging {
		fmt.Println(path, "->", name)
	}

	zf, err := w.Create(name)
	if err != nil {
		return err
	}

	rawSize += len(data)

	_, err = zf.Write(data)

	return err
}

// archiveName converts a host file system path into the name that will be
// stored inside the zip archive: relative to root, and slash-separated.
//
// When root names a single file rather than a directory, filepath.Rel would
// return ".", so in that case the file's base name is used instead.
func archiveName(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}

	if rel == "." {
		rel = filepath.Base(path)
	}

	// filepath.ToSlash rewrites the host separator (a backslash on Windows)
	// as the forward slash that the ZIP format requires.
	return filepath.ToSlash(rel), nil
}

// omitList renders the omit list as a stable, printable string. It is folded
// into the digest so that changing which files are excluded from the archive
// forces the archive to be rebuilt. Without this, editing the "--omit" option
// in the go:generate directive would leave a stale archive in place, because
// none of the *source files* would have changed.
func omitList() string {
	names := make([]string, 0, len(omit))
	for name := range omit {
		names = append(names, name)
	}

	// A Go map has no defined iteration order, so the names must be sorted
	// before they can be used as a stable digest input.
	sort.Strings(names)

	return strings.Join(names, ",")
}
