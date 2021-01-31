package cli

import "os"

// Exit codes passed to the operating system.
const (
	ExitSuccess      = 0
	ExitGeneralError = 1
	ExitUsageError   = 2
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

// NewExitError constructs an ExitError
func NewExitError(msg string, code int) ExitError {
	return ExitError{ExitStatus: code, Message: msg}
}

// Exit exits the program, returning the given exit status code to the operating system.
func (e ExitError) Exit() {
	os.Exit(e.ExitStatus)
}
