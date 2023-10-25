package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
)

// addFiles walks the file tree rooted at path and adds each file or directory
// to the zip archive.
func addFiles(w *zip.Writer, path, prefix string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return addDir(w, path, prefix)
	}

	return addFile(w, path, prefix)
}

// addDir adds the files in a directory to the zip archive.
func addDir(w *zip.Writer, path, prefix string) error {
	if log {
		fmt.Println(path + "/")
	}

	// Open the directory.
	dir, err := os.Open(path)
	if err != nil {
		return err
	}

	defer dir.Close()

	// Get list of files in the directory.
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	// Recursively add files in the subdirectories.
	for _, file := range files {
		if file.IsDir() {
			if err := addDir(w, filepath.Join(path, file.Name()), filepath.Join(prefix, file.Name())); err != nil {
				return err
			}
		} else {
			if err := addFile(w, filepath.Join(path, file.Name()), prefix); err != nil {
				return err
			}
		}
	}

	return nil
}

// addDir adds a single file to the zip archive. If the file is in the omit
// list, it is skipped.
func addFile(w *zip.Writer, path, prefix string) error {
	// Skip files that are in the omit list.
	if omit[filepath.Base(path)] {
		if log {
			fmt.Println(path, "(omitted)")
		}

		return nil
	}

	// Open the file.
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	// If we are building a digest value, capture that now.
	if digest != "" {
		if err := addFileToDigest(path, file); err != nil {
			return err
		}
	}

	if log {
		fmt.Println(path)
	}

	defer file.Close()

	zf, err := w.Create(path)
	if err != nil {
		return err
	}

	// Get file contents and write to the zip archive
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if _, err := zf.Write(data); err != nil {
		return err
	}

	return nil
}
