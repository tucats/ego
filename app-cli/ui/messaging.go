// Package ui contains basic tools for interacting with a user. This includes generating
// informational and debugging messages. It also includes functions for controlling
// whether those messages are displayed or not.
package ui

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tucats/ego/i18n"
)

// Formatted output types for data more complex than individual messages, such
// as the format for tabular data output. Choices are "text", "json", "indented",
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
var OutputFormat = TextFormat

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
	AuthLogger
	ByteCodeLogger
	CLILogger
	CompilerLogger
	DBLogger
	DebugLogger
	InfoLogger
	RestLogger
	ServerLogger
	SQLLogger
	SymbolLogger
	TableLogger
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
	{"AUTH", false},
	{"BYTECODE", false},
	{"CLI", false},
	{"COMPILER", false},
	{"DB", false},
	{"DEBUG", false},
	{"INFO", false},
	{"REST", false},
	{"SERVER", false},
	{"SQL", false},
	{"SYMBOLS", false},
	{"TABLES", false},
	{"TRACE", false},
	{"USER", false},
}

// LogTimeStampFormat stores the format string used to produce log messages,
// using the Go standard format string. You can override the default by
// creating a profile item called "ego.log.format".
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

// Return a comma-separated list of the active loggers.
func ActiveLoggers() string {
	result := strings.Builder{}

	for _, logger := range loggers {
		if logger.active {
			if result.Len() > 0 {
				result.WriteRune(',')
			}

			result.WriteString(logger.name)
		}
	}

	return result.String()
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
// If there are no arguments, the format string is output without
// further processing (that is, safe even if it contains formatting
// operators, as long as there are no arguments).
//
// Note that the format string is tested to see if it is probably
// a localization string. If so, it is localized before output.
// If it was localized, and there is a single argument that is a
// proper map[string]interface{} object, then that is used for the
// formatting.
func Say(format string, args ...interface{}) {
	var s string

	alreadyFormatted := false

	// If it might be a message ID, translate it. If there is not
	// translation available, then the format is unchanged.
	if strings.Index(format, ".") > 0 {
		if len(args) > 0 {
			if m, ok := args[0].(map[string]interface{}); ok {
				format = i18n.T(format, m)
				alreadyFormatted = true
			}
			format = i18n.T(format)
		} else {
			format = i18n.T(format)
		}
	}

	if !QuietMode {
		if alreadyFormatted || len(args) == 0 {
			s = i18n.T(format)
		} else {
			s = fmt.Sprintf(format, args...)
		}

		fmt.Println(s)
	}
}
