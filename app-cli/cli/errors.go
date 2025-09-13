package cli

import (
	"fmt"
	"os"
)

// Exit codes passed to the operating system.
const (
	// The application exited without error.
	ExitSuccess = 0

	// The application incurred an error during execution.
	ExitGeneralError = 1

	// The application had an error in command line specification or environment.
	ExitUsageError = 2
)

// ExitError is a wrapped error code structure used to return a message
// and a desired operating system exit value.
type ExitError struct {
	ExitStatus int
	Message    string
}

// Error formats an ExitError into a text string for display.
func (e ExitError) Error() string {
	return e.Message
}

// NewExitError constructs an ExitError.
func NewExitError(msg string, code int) ExitError {
	return ExitError{ExitStatus: code, Message: msg}
}

// Exit exits the program, returning the given exit status code to the operating system.
func (e ExitError) Exit() {
	os.Exit(e.ExitStatus)
}

// CLI error message strings.
const (
	CLIErrorPrefix            = "during command line processing"
	InvalidBooleanValueError  = "option --%s invalid boolean value: %s"
	InvalidIntegerError       = "option --%s invalid integer value: %s"
	InvalidKeywordError       = "option --%s has no such keyword: %s"
	RequiredNotFoundError     = "required option %s not found"
	TooManyParametersError    = "too many parameters on command line"
	UnexpectedParametersError = "unexpected parameters or invalid subcommand"
	UnknownOptionError        = "unknown option: %s"
	WrongParameterCountError  = "incorrect number of parameters"
)

// Wrapper for CLI errors.
type Error struct {
	err error
}

// NewCLIError generates a new CLIError object using the message string and optional
// values that are formatted using the message string.
func NewCLIError(msg string, args ...any) Error {
	e := Error{
		err: fmt.Errorf(msg, args...),
	}

	return e
}

// Error returns a string representation of the CLIError.
func (ce Error) Error() string {
	return fmt.Sprintf("%s, %s", CLIErrorPrefix, ce.err.Error())
}
