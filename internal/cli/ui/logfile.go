package ui

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/internal/errors"
)

var (
	logFile            *os.File
	baseLogFileName    string
	currentLogFileName string

	// LogRetainCount is the number of roll-over log versions to keep in the
	// logging directory.
	LogRetainCount = -1

	// rollOverTaskOnce guarantees the midnight log-rollover goroutine is started
	// at most once per process, however many times OpenLogFile is called
	// (GORTNS-4).
	rollOverTaskOnce sync.Once
)

// Define an io.Writer implementation that writes the buffer as a string to the log file.
type LogWriter struct{}

// Write is the io.Writer interface implementation.
func (l LogWriter) Write(buffer []byte) (int, error) {
	msg := string(buffer)

	tokens := strings.Split(msg, " ")
	if len(tokens) > 2 {
		msg = strings.Join(tokens[2:], " ")
	}

	Log(ServerLogger, "server.write.text", A{
		"text": strings.TrimSuffix(msg, "\n")})

	return len(msg), nil
}

// OpenLogFile opens a log file for writing. If the withTimeStamp flag is set,
// the log file name has a timestamp appended. Also, if there are too many
// old log files, they are purged.
func OpenLogFile(userLogFileName string, withTimeStamp bool) error {
	if LogRetainCount < 1 {
		LogRetainCount = 3
	}

	if err := openLogFile(userLogFileName, withTimeStamp); err != nil {
		return errors.New(err)
	}

	if withTimeStamp {
		PurgeLogs()

		// GORTNS-4: the rollover task is a "for { sleep until midnight; roll the
		// log }" loop with no exit, so exactly one of them must ever exist. It had
		// no guard at all: a second call to OpenLogFile with withTimeStamp set --
		// from a test, or from any future code that reopens the log -- would have
		// started a second identical task, and both would try to roll the same log
		// over at midnight.
		//
		// sync.Once is the Go tool for "do this at most once for the life of the
		// process, no matter how many callers ask or how many goroutines ask at the
		// same time". Its Do method runs the function the first time and is a
		// no-op on every later call, and it handles concurrent callers safely, so
		// no separate mutex is needed.
		rollOverTaskOnce.Do(func() {
			go rollOverTask()
		})
	}

	return nil
}

// Return the path of the current log file being written to.
func CurrentLogFile() string {
	if logFile == nil {
		return ""
	}

	return currentLogFileName
}

// Internal routine that actually opens a log file.
func openLogFile(path string, withTimeStamp bool) error {
	var (
		err      error
		fileName string
	)

	_ = SaveLastLog()

	if withTimeStamp {
		fileName = timeStampLogFileName(path)
	} else {
		fileName, err = filepath.Abs(path)
		if err != nil {
			return errors.New(err)
		}
	}

	logFile, err = os.Create(fileName)
	if err != nil {
		logFile = nil

		return errors.New(err)
	}

	baseLogFileName, _ = filepath.Abs(path)
	currentLogFileName, _ = filepath.Abs(fileName)

	WriteLog(InfoLogger, "logging.filename", A{
		"name": currentLogFileName})

	return nil
}

// Schedule roll-over operations for the log. We calculate when the next start-of-date + 24 hours
// is, and sleep until then. We then roll over the log file and sleep again.
func rollOverTask() {
	count := 0

	for {
		year, month, day := time.Now().Date()
		beginningOfDay := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		wakeTime := beginningOfDay.Add(24*time.Hour + time.Second)
		sleepUntil := time.Until(wakeTime)

		// Pause the message to the initial startup log is more readable.
		if count == 0 {
			time.Sleep(1 * time.Second)
		}

		count++

		WriteLog(InfoLogger, "logging.rollover.scheduled", A{
			"count": count,
			"time":  wakeTime.String()})

		time.Sleep(sleepUntil)
		RollOverLog()
	}
}

// Roll over the open log. Close the current log, and rename it to include a timestamp of when
// it was created. Then create a new log file.
func RollOverLog() {
	if err := SaveLastLog(); err != nil {
		WriteLog(InternalLogger, "logging.rollover.save", A{
			"error": err})

		return
	}

	if err := openLogFile(baseLogFileName, true); err != nil {
		WriteLog(InfoLogger, "logging.rollover.open", A{
			"error": err})

		return
	}

	PurgeLogs()
}

func timeStampLogFileName(path string) string {
	logStarted := time.Now()
	dateStamp := logStarted.Format("_2006-01-02-150405")
	newName, _ := filepath.Abs(strings.TrimSuffix(path, ".log") + dateStamp + ".log")

	return newName
}

// Save the current (last) log file to the archive name with the timestamp of when the log
// was initialized.
func SaveLastLog() error {
	if logFile != nil {
		WriteLog(InfoLogger, "logging.rollover", nil)

		sequenceMux.Lock()
		defer sequenceMux.Unlock()
		logFile.Close()

		logFile = nil
	}

	return nil
}

func PurgeLogs() int {
	count := 0
	keep := LogRetainCount
	searchPath := path.Dir(CurrentLogFile())
	names := []string{}

	logMessage := "logging.purging"

	if archiveLogFileName != "" {
		logMessage = "logging.archiving"
	}

	Log(InfoLogger, logMessage, A{
		"count": keep,
		"path":  searchPath})

	// Start by making a list of the log files in the directory.
	files, err := os.ReadDir(searchPath)
	if err != nil {
		Log(InfoLogger, "logging.list.error", A{
			"error": err.Error()})

		return count
	}

	logNamePattern := rolloverNamePattern(baseLogFileName)

	for _, file := range files {
		if logNamePattern.MatchString(file.Name()) && !file.IsDir() {
			names = append(names, file.Name())
		}
	}

	if len(names) <= 1 {
		return 0
	}

	sort.Strings(names)

	for n := 0; n < len(names)-keep; n++ {
		name := names[n]
		fileName := path.Join(searchPath, name)

		// If we're archiving, add this log file to the archive before we delete it.
		if archiveLogFileName != "" {
			if err := addToLogArchive(fileName); err != nil {
				Log(InfoLogger, "logging.archive.error", A{
					"filename": fileName,
					"error":    err})
			} else {
				Log(InfoLogger, "logging.archived", A{
					"filename": fileName})

				if err := os.Remove(fileName); err != nil {
					Log(InfoLogger, "logging.archive.delete.error", A{
						"filename": fileName,
						"error":    err})
				}

				count++
			}

			continue
		}

		// Now delete the file being purged.
		if err := os.Remove(fileName); err != nil {
			Log(InfoLogger, "logging.purge.error", A{
				"filename": fileName,
				"error":    err})
		} else {
			Log(InfoLogger, "logging.purged", A{
				"filename": fileName})

			count++
		}
	}

	return count
}

// rolloverNamePattern returns the regular expression that matches this
// instance's rolled-over log file names, derived from the base log filename
// stem. The match is scoped to this specific server instance (cluster name +
// port): a plain prefix match is not enough, since a standalone server's stem
// (e.g. "ego-server_") is itself a string prefix of a cluster member's
// qualified stem (e.g. "ego-server_gang_8501_"), which would cause the
// standalone instance to scoop up another instance's log files. Anchoring the
// match to the exact rollover timestamp format that timeStampLogFileName()
// appends avoids that.
// For example, base "ego-server_foo_8501.log" → pattern
// "ego-server_foo_8501_" + timestamp + ".log".
func rolloverNamePattern(base string) *regexp.Regexp {
	stem := strings.TrimSuffix(filepath.Base(base), ".log") + "_"

	return regexp.MustCompile("^" + regexp.QuoteMeta(stem) + `\d{4}-\d{2}-\d{2}-\d{6}\.log$`)
}

// olderLogFileNames returns the directory holding the log files and the base
// names of this instance's rolled-over log files that are still on disk,
// sorted oldest to newest. The currently active log file is excluded even
// though its own name matches the same timestamped pattern.
func olderLogFileNames() (string, []string) {
	dir := path.Dir(CurrentLogFile())

	files, err := os.ReadDir(dir)
	if err != nil {
		return dir, nil
	}

	pattern := rolloverNamePattern(baseLogFileName)
	active := filepath.Base(currentLogFileName)
	names := []string{}

	for _, file := range files {
		if file.IsDir() || file.Name() == active {
			continue
		}

		if pattern.MatchString(file.Name()) {
			names = append(names, file.Name())
		}
	}

	sort.Strings(names)

	return dir, names
}
