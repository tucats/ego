// Package ui contains basic tools for interacting with a user. This includes generating
// informational and debugging messages. It also includes functions for controlling
// whether those messages are displayed or not.
package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Formatted output types for data more complex than individual messages, such
// as the format for tabular data output. Choices are "text", "json", or default
// which means whatever was set by the command line or profile.
const (

	// DefaultTableFormat means use whatever the default is that may have been set
	// by the global option --output-type, etc.
	DefaultTableFormat = "default"

	// TextTableFormat indicates the output format should be human-readable text
	TextTableFormat = "text"

	// JSONTableFormat indicates the output format should be machine-readable JSON
	JSONTableFormat = "json"
)

// OutputFormat is the default output format if not overridden by a global option
// or explicit call from the user.
var OutputFormat = TextTableFormat

// DebugMode determines if "debug" style messages are output.
var DebugMode = false

// QuietMode determines if optional messaging is performed.
var QuietMode = false

// The sequence number is generated and incremented for each message, in order. The
// associated mutext is used to prevent the sequence from being incremented by a
// separate thread or goroutine.
var sequence = 0
var sequenceMux sync.Mutex

// Names of loggers go here.
const (
	DebugLogger    = "DEBUG"
	CLILogger      = "CLI"
	CompilerLogger = "COMPILER"
	SymbolLogger   = "SYMBOLS"
	ServerLogger   = "SERVER"
	AppLogger      = "APP"
	ByteCodeLogger = "BYTECODE"
	UserLogger     = "USER"
	DBLogger       = "DB"
)

// Loggers is a map of the names of logging modes that are enabled
var Loggers = map[string]bool{
	DebugLogger:    false,
	CLILogger:      false,
	CompilerLogger: false,
	SymbolLogger:   false,
	ServerLogger:   false,
	AppLogger:      false,
	ByteCodeLogger: false,
	UserLogger:     false,
	DBLogger:       false,
}

// SetLogger enables or disables a logger
func SetLogger(logger string, mode bool) bool {
	if _, ok := Loggers[logger]; !ok {
		return false
	}
	Loggers[logger] = mode

	// If any loggers are on, enable logging mode.
	DebugMode = false
	for _, v := range Loggers {
		if v {
			DebugMode = true
		}
	}
	return true
}

// Debug displays a message if debugging mode is enabled.
func Debug(logger string, format string, args ...interface{}) {
	if DebugMode {
		if active, found := Loggers[logger]; active && found {
			Log(logger, format, args...)
		}
	}
}

// Log displays a message to stdout
func Log(class string, format string, args ...interface{}) {
	s := LogMessage(class, format, args...)
	fmt.Println(s)
}

// LogMessage displays a message to stdout
func LogMessage(class string, format string, args ...interface{}) string {
	s := fmt.Sprintf(format, args...)

	sequenceMux.Lock()
	defer sequenceMux.Unlock()

	sequence = sequence + 1
	sequenceString := fmt.Sprintf("%d", sequence)
	tf := "20060102150405"
	s = fmt.Sprintf("[%s] %-5s %-7s: %s", time.Now().Format(tf), sequenceString, strings.ToUpper(class), s)
	return s
}

// Say displays a message to the user unless we are in "quiet" mode
func Say(format string, args ...interface{}) {
	if !QuietMode {
		s := fmt.Sprintf(format, args...)
		fmt.Println(s)
	}
}
