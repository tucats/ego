// Package ui contains basic tools for interacting with a user. This includes generating
// informational and debugging messages, including functions for controlling
// whether those messages are displayed or not.
package ui

import (
	"fmt"
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

	NoSuchLogger = -1
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
	InternalLogger
	OptimizerLogger
	RestLogger
	RouteLogger
	ServerLogger
	ServicesLogger
	SQLLogger
	StatsLogger
	SymbolLogger
	TableLogger
	TraceLogger
	TokenLogger
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
	{"INTERNAL", true},
	{"OPTIMIZER", false},
	{"REST", false},
	{"ROUTE", false},
	{"SERVER", false},
	{"SERVICES", false},
	{"SQL", false},
	{"STATS", false},
	{"SYMBOLS", false},
	{"TABLES", false},
	{"TRACE", false},
	{"TOKEN", false},
	{"USER", false},
}

// LogTimeStampFormat stores the format string used to produce log messages,
// using the Go standard format string. You can override the default by
// creating a profile item called "ego.log.format".
var LogTimeStampFormat string

// DefineLogger creates a new logger that can be used by the program.
// The logger name must be unique. The return value is the logger id
// passed to subsequent calls to ui.Log(loggerId, msg...) calls to
// generate log output. This must be done in the main program before
// the parser starts running, since it will use this list of logger names
// to validate logger setting options.
func DefineLogger(name string, active bool) int {
	name = strings.ToUpper(name)

	for _, logger := range loggers {
		if logger.name == name {
			panic("Duplicate logger name: " + name)
		}
	}

	loggers = append(loggers, logger{name: name, active: active})

	return len(loggers) - 1
}

// LoggerNames returns a string array with the names of all defined loggers.
func LoggerNames() []string {
	result := make([]string, len(loggers))

	for idx, logger := range loggers {
		result[idx] = logger.name
	}

	return result
}

// Get the name of a given logger class.
func LoggerByClass(class int) string {
	if class < 0 || class >= len(loggers) {
		WriteLog(InternalLogger, "ERROR: Invalid LoggerName() class %d", class)

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
func LoggerByName(loggerName string) int {
	for id, logger := range loggers {
		if strings.EqualFold(logger.name, loggerName) {
			return id
		}
	}

	WriteLog(InternalLogger, "ERROR: Invalid Logger() class name %s", loggerName)

	return NoSuchLogger
}

// Active enables or disables a logger.
func Active(class int, mode bool) bool {
	if class < 0 || class >= len(loggers) {
		WriteLog(InternalLogger, "ERROR: Invalid SetLogger() class %d", class)

		return false
	}

	loggers[class].active = mode

	return true
}

// Determine if a given logger is active. This is particularly useful
// when deciding if it's worth doing complex formatting operations.
func IsActive(class int) bool {
	if class < 0 || class >= len(loggers) {
		WriteLog(InternalLogger, "ERROR: Invalid LoggerIsActive() class %d", class)

		return false
	}

	return loggers[class].active
}

// Log displays a message if the selected log class is enabled. If the
// class is not active, no action is taken.  Use WriteLog if you want
// to write a message to a logging class regardless of whether it is
// active or not.
func Log(class int, format string, args ...interface{}) {
	if class < 0 || class >= len(loggers) {
		WriteLog(InternalLogger, "ERROR: Invalid Debug() class %d", class)

		return
	}

	if loggers[class].active {
		WriteLog(class, format, args...)
	}
}

// WriteLog displays a message to the log, regardless of whether the
// logger is enabled. If there is an active log file, the message is
// added to the log file, else it is written to stdout.
func WriteLog(class int, format string, args ...interface{}) {
	if class < 0 || class >= len(loggers) {
		WriteLog(InternalLogger, "ERROR: Invalid Log() class %d", class)

		return
	}

	s := formatLogMessage(class, format, args...)

	if logFile != nil {
		if _, err := logFile.Write([]byte(s + "\n")); err != nil {
			logFile = nil

			WriteLog(InternalLogger, "ERROR: Log() unable to write log entry; %v", err)

			return
		}
	} else {
		fmt.Println(s)
	}
}

// formatLogMessage displays a message to stdout.
func formatLogMessage(class int, format string, args ...interface{}) string {
	if class < 0 || class >= len(loggers) {
		WriteLog(InternalLogger, "ERROR: Invalid LogMessage() class %d", class)

		return ""
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

		if s != "" {
			fmt.Println(s)
		}
	}
}
