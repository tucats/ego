package app

import "fmt"

// List of application errors here
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

type AppError struct {
	err error
}

func NewAppError(msg string, args ...interface{}) AppError {
	a := AppError{
		err: fmt.Errorf(msg, args...),
	}

	return a
}

func (e AppError) Error() string {
	msg := fmt.Sprintf("%s, %s", ErrorPrefixString, e.err.Error())
	return msg
}
