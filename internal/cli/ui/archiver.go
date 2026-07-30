package ui

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var archiveLogFileName string

// Tuning parameters for the cross-process archive lock. A cluster's nodes can
// share a single archive file and directory, and all nodes typically roll
// their logs over at the same wall-clock boundary, so writes to the shared
// archive must be serialized across processes, not just goroutines.
const (
	archiveLockRetryInterval = 50 * time.Millisecond
	archiveLockMaxWait       = 10 * time.Second
	archiveLockStaleAfter    = 30 * time.Second
)

// SetArchive is used to set the archive string value. This can be done from the App
// object during initialization using a configuration item, or by the command line
// processing of the --archive-log global option.
func SetArchive(name string) {
	archiveLogFileName = name
}

// ArchiveLogFileName returns the current archive log filename, or an empty string
// if no archive has been configured.
func ArchiveLogFileName() string {
	return archiveLogFileName
}

// Given a log file name, add it to the archive log file. If the archive log file
// does not exist, it is created. Access to the archive is serialized across
// processes with a lock file, since multiple nodes of a cluster can share the
// same archive file and directory.
func addToLogArchive(fileName string) error {
	unlock, err := acquireArchiveLock(archiveLogFileName)
	if err != nil {
		return err
	}
	defer unlock()

	newArchiveName := archiveLogFileName + ".new"

	zipReader, err := zip.OpenReader(archiveLogFileName)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		zipReader = nil
	}

	targetFile, err := os.Create(newArchiveName)
	if err != nil {
		return err
	}

	targetZipWriter := zip.NewWriter(targetFile)

	err = copyExistingArchive(zipReader, targetZipWriter)
	if err != nil {
		return err
	}

	// Now copy the file named by the fileName parameter into the output archive.
	if file, err := os.Open(fileName); err == nil {
		// Get the fileinfo for the file
		fileInfo, err := file.Stat()
		if err != nil {
			return err
		}

		header, _ := zip.FileInfoHeader(fileInfo)
		header.Name = filepath.Base(fileName)

		if targetItem, err := targetZipWriter.CreateHeader(header); err != nil {
			return err
		} else {
			if _, err = io.Copy(targetItem, file); err != nil {
				return err
			}
		}
	}

	// The create of the new archive went well, so delete the old  archive
	// if it existed, and rename the temp name to the new archive file.
	if zipReader != nil {
		_ = zipReader.Close()
		_ = os.Remove(archiveLogFileName)
	}

	_ = targetZipWriter.Close()
	_ = targetFile.Close()

	return os.Rename(newArchiveName, archiveLogFileName)
}

// acquireArchiveLock obtains an exclusive, cross-process lock on the given
// archive file using an atomically-created lock file (relies on O_EXCL, so it
// works the same way on every platform this project supports). It returns a
// function that releases the lock; the caller must invoke it (typically via
// defer) once done with the archive.
//
// A lock file older than archiveLockStaleAfter is assumed to be left over from
// a process that died while holding it, and is removed so progress can
// continue.
func acquireArchiveLock(archiveName string) (func(), error) {
	lockName := archiveName + ".lock"
	deadline := time.Now().Add(archiveLockMaxWait)

	for {
		lockFile, err := os.OpenFile(lockName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = lockFile.WriteString(strconv.Itoa(os.Getpid()))
			lockFile.Close()

			return func() { _ = os.Remove(lockName) }, nil
		}

		if !os.IsExist(err) {
			return nil, err
		}

		if info, statErr := os.Stat(lockName); statErr == nil {
			if time.Since(info.ModTime()) > archiveLockStaleAfter {
				_ = os.Remove(lockName)

				continue
			}
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for archive lock %s", lockName)
		}

		time.Sleep(archiveLockRetryInterval)
	}
}

func copyExistingArchive(zipReader *zip.ReadCloser, targetZipWriter *zip.Writer) error {
	if zipReader != nil {
		for _, zipItem := range zipReader.File {
			if zipItemReader, err := zipItem.Open(); err != nil {
				return err
			} else {
				if header, err := zip.FileInfoHeader(zipItem.FileInfo()); err != nil {
					return err
				} else {
					header.Name = zipItem.Name
					if targetItem, err := targetZipWriter.CreateHeader(header); err != nil {
						return err
					} else {
						if _, err = io.Copy(targetItem, zipItemReader); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}
