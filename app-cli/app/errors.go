package app

import "fmt"

// List of application errors.
const (
	ErrorPrefixString = "application error"

	HTTPError               = "received HTTP %d"
	InvalidCredentialsError = "invalid credentials"
	InvalidLoggerName       = "invalid logger name: %s"
	InvalidOutputFormatErr  = "invalid output format specified: %s"
	LogonEndpointError      = "logon endpoint not found"
	NoCredentialsError      = "no credentials provided"
	NoLogonServerError      = "no --logon-server specified"
	UnknownOptionError      = "unknown command line option: %s"
)

// AppError is the wrapper around general application errors.
type AppError struct {
	err error
}

// NewAppError creates a new AppError instance, using the error message
// string provided, and any optional parameters are used as format
// values for the message string.
func NewAppError(msg string, args ...interface{}) AppError {
	a := AppError{
		err: fmt.Errorf(msg, args...),
	}

	return a
}

// Error formats an error into a printable string.
func (e AppError) Error() string {
	msg := fmt.Sprintf("%s, %s", ErrorPrefixString, e.err.Error())

	return msg
}
