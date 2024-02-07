package ui

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

var archiveLogFileName string

// SetArchive is used to set the archive string value. This can be done from the App
// object during initialization using a configuration item, or by the command line
// processing of the --archive-log global option.
func SetArchive(name string) {
	archiveLogFileName = name
}

// Given a log file name, add it to the archive log file. If the archive log file
// does not exist, it is created.
func addToLogArchive(fileName string) error {
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
