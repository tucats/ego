package app

// List of application errors.
const (
	HTTPError               = "received HTTP %d"
	InvalidCredentialsError = "invalid credentials"
	InvalidLoggerName       = "invalid logger name: %s"
	InvalidOutputFormatErr  = "invalid output format specified: %s"
	LogonEndpointError      = "logon endpoint not found"
	NoCredentialsError      = "no credentials provided"
	NoLogonServerError      = "no --logon-server specified"
	UnknownOptionError      = "unknown command line option: %s"
)
