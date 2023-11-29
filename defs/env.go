package defs

// This section contains environment variable names used by Ego.
const (
	// The environment variable that defines where the runtime files are
	// to be found.
	EgoPathEnv = "EGO_PATH"

	// The environment variable that contains the path to which the log file
	// is written.
	EgoLogEnv = "EGO_LOG"

	// The environment variable that contains the name(s) of the loggers that
	// are to be enabled by default at startup (before command line processing).
	EgoDefaultLogging = "EGO_DEFAULT_LOGGING"

	// The environment variable that contains the name to use for writing log
	// file messages. If not specified, defaults to writing to stdout.
	EgoDefaultLogFileName = "EGO_LOG_FILE"

	// The environment variable that contains the port number to start listening
	// on when in server mode, or the default port to use to connect to a server.
	EgoPortEnv = "EGO_PORT"

	// The environment variable that contains the default file name for the HTTPS
	// CERT file.
	EgoCertFileEnv = "EGO_CERT_FILE"

	// The environment variable that contains the default file name for the HTTPS
	// KEY file.
	EgoKeyFileEnv = "EGO_KEY_FILE"

	// The environment variable that contains "TRUE" if the client is allowed to
	// connect to a server using an insecure connection.
	EgoInsecureClientEnv = "EGO_INSECURE_CLIENT"
)
