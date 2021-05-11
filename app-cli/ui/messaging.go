// Package ui contains basic tools for interacting with a user. This includes generating
// informational and debugging messages. It also includes functions for controlling
// whether those messages are displayed or not.
package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/errors"
)

// Formatted output types for data more complex than individual messages, such
// as the format for tabular data output. Choices are "text", "json", "indent",
// or default which means whatever was set by the command line or profile.
const (
	// DefaultTableFormat means use whatever the default is that may have been set
	// by the global option --output-type, etc.
	DefaultTableFormat = "default"

	// TextTableFormat indicates the output format should be human-readable text.
	TextFormat = "text"

	// JSONTableFormat indicates the output format should be machine-readable JSON.
	JSONFormat = "json"

	// JSONIndentedTableFormat indicates JSON output that is indented for readability.
	JSONIndentedFormat = "indented"

	JSONIndentPrefix = ""
	JSONIndentSpacer = "   "
)

// OutputFormat is the default output format if not overridden by a global option
// or explicit call from the user.
var OutputFormat = "text"

// QuietMode determines if optional messaging is performed.
var QuietMode = false

// The sequence number is generated and incremented for each message, in order. The
// associated mutext is used to prevent the sequence from being incremented by a
// separate thread or goroutine.
var sequence = 0
var sequenceMux sync.Mutex

// Classes of loggers go here. These are sequential integer values, and should match
// the order of the items in the loggers array below.
const (
	AppLogger = iota
	ByteCodeLogger
	CLILogger
	CompilerLogger
	DBLogger
	DebugLogger
	InfoLogger
	ServerLogger
	SymbolLogger
	TraceLogger
	UserLogger
)

type logger struct {
	name   string
	active bool
}

// The order of these items must match the numeric values of the logger classses above.
var loggers []logger = []logger{
	{"APP", false},
	{"BYTECODE", false},
	{"CLI", false},
	{"COMPILER", false},
	{"DB", false},
	{"DEBUG", false},
	{"INFO", false},
	{"SERVER", false},
	{"SYMBOLS", false},
	{"TRACE", false},
	{"USER", false},
}

var logFile *os.File
var baseLogFileName string
var currentLogFileName string

func OpenLogFile(userLogFileName string) *errors.EgoError {
	err := openLogFile(userLogFileName)
	if !errors.Nil(err) {
		return errors.New(err)
	}

	go rollOverTask()

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
func openLogFile(path string) *errors.EgoError {
	var err error

	_ = SaveLastLog()

	fileName := timeStampLogFileName(path)

	logFile, err = os.Create(fileName)
	if err != nil {
		logFile = nil

		return errors.New(err)
	}

	baseLogFileName, _ = filepath.Abs(path)
	currentLogFileName, _ = filepath.Abs(fileName)

	Log(InfoLogger, "New log file opened: %s", currentLogFileName)

	return nil
}

// Schedule roll-over operations for the log. We calculate when the next start-of-date + 24 hours
// is, and sleep until then. We then roll over the log file and sleep again.
func rollOverTask() {
	for {
		year, month, day := time.Now().Date()
		beginningOfDay := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		wakeTime := beginningOfDay.Add(24*time.Hour + time.Second)
		sleepUntil := time.Until(wakeTime)
		Log(InfoLogger, "Log rollover scheduled for %s", wakeTime.String())
		time.Sleep(sleepUntil)
		RollOverLog()
	}
}

// Roll over the open log. Close the current log, and rename it to include a timestamp of when
// it was created. Then create a new log file.
func RollOverLog() {
	err1 := SaveLastLog()
	if err1 != nil {
		panic("Unable to roll over log file; " + err1.Error())
	}

	err := openLogFile(baseLogFileName)
	if err != nil {
		panic("Unable to open new log file; " + err.Error())
	}
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
		Log(InfoLogger, "Log file being rolled over")

		sequenceMux.Lock()
		defer sequenceMux.Unlock()
		logFile.Close()

		logFile = nil
	}

	return nil
}

// This will contain the format string used to produce log messages, using the Go
// standard format string. You can override the default by creating a profile item
// called "ego.log.format".
var LogTimeStampFormat string

func LoggerNames() []string {
	result := make([]string, len(loggers))

	for idx, logger := range loggers {
		result[idx] = logger.name
	}

	return result
}

// Get the name of a given logger class.
func LoggerName(class int) string {
	if class < 0 || class >= len(loggers) {
		return ""
	}

	return loggers[class].name
}

// For a given logger name, find the class ID.
func Logger(loggerName string) int {
	for id, logger := range loggers {
		if strings.EqualFold(logger.name, loggerName) {
			return id
		}
	}

	return -1
}

// SetLogger enables or disables a logger.
func SetLogger(class int, mode bool) bool {
	if class < 0 || class >= len(loggers) {
		panic("invalid logger: " + strconv.Itoa(class))
	}

	loggers[class].active = mode

	return true
}

// Determine if a given logger is active. This is particularly useful
// when deciding if it's worth doing complex formatting operations.
func LoggerIsActive(class int) bool {
	if class < 0 || class >= len(loggers) {
		panic("invalid logger: " + strconv.Itoa(class))
	}

	return loggers[class].active
}

// Debug displays a message if debugging mode is enabled.
func Debug(class int, format string, args ...interface{}) {
	if class < 0 || class >= len(loggers) {
		panic("invalid logger: " + strconv.Itoa(class))
	}

	if loggers[class].active {
		Log(class, format, args...)
	}
}

// Log displays a message to stdout.
func Log(class int, format string, args ...interface{}) {
	if class < 0 || class >= len(loggers) {
		panic("invalid logger: " + strconv.Itoa(class))
	}

	s := LogMessage(class, format, args...)

	if logFile != nil {
		_, err := logFile.Write([]byte(s + "\n"))
		if err != nil {
			panic("Unable to write to log file; " + err.Error())
		}
	} else {
		fmt.Println(s)
	}
}

// LogMessage displays a message to stdout.
func LogMessage(class int, format string, args ...interface{}) string {
	if class < 0 || class >= len(loggers) {
		panic("invalid logger: " + strconv.Itoa(class))
	}

	className := loggers[class].name
	s := fmt.Sprintf(format, args...)

	sequenceMux.Lock()
	defer sequenceMux.Unlock()

	sequence = sequence + 1
	sequenceString := fmt.Sprintf("%d", sequence)

	if LogTimeStampFormat == "" {
		LogTimeStampFormat = "2006-01-02 15:04:05"
	}

	s = fmt.Sprintf("[%s] %-5s %-7s: %s", time.Now().Format(LogTimeStampFormat), sequenceString, className, s)

	return s
}

// Say displays a message to the user unless we are in "quiet" mode.
func Say(format string, args ...interface{}) {
	if !QuietMode {
		s := fmt.Sprintf(format, args...)
		fmt.Println(s)
	}
}
