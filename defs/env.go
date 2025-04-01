package defs

// This section contains environment variable names used by Ego.
const (
	// The environment variable that defines where the runtime files are
	// to be found.
	EgoPathEnv = "EGO_PATH"

	// The environment variable that contains the path to which the log file
	// is written.
	EgoLogEnv = "EGO_LOG"

	// This environment variable sets the default localization language to use.
	// If not specified, the native OS language is used.
	EgoLangEnv = "EGO_LANG"

	// If set, this environment variable points to a JSON localization file
	// that is merged with the internally-compiled localization dictionaries.
	EgoLocalzationFileEnv = "EGO_LOCALIZATION_FILE"

	// The environment variable that contains the name of the archive zip file
	// to use for log files that have aged out.
	EgoArchiveLogEnv = "EGO_LOG_ARCHIVE"

	// The environment variable that contains the name(s) of the loggers that
	// are to be enabled by default at startup (before command line processing).
	EgoDefaultLogging = "EGO_DEFAULT_LOGGING"

	// The environment variable that contains the default console text output
	// format "TEXT", "INDENTED", or "JSON". This corresponds to the global
	// -- output-type command line option.
	EgoOutputFormatEnv = "EGO_OUTPUT_FORMAT"

	// When set to "true" this value suppresses "chatty" output from the server
	// so the output messages are suppressed for successful operations. This
	// helps support using Ego in scripts that don't want verbose output.
	EgoQuietEnv = "EGO_QUIET"

	// This environment variable sets the maximum number of cpus that Ego will
	// allow itself to use to support goroutines.
	EgoMaxProcsEnv = "EGO_MAX_PROCS"

	// The environment variable that contains the log format to use for log messages.
	// Can be either "json" or "text".
	EgoLogFormatEnv = "EGO_LOG_FORMAT"

	// The environment variable that contains the name to use for writing log
	// file messages. If not specified, defaults to writing to stdout.
	EgoDefaultLogFileName = "EGO_LOG_FILE"

	// The default configuration profile name to use. This corresponds to the
	// global --profile command line option.
	EgoProfileEnv = "EGO_PROFILE"

	// The environment variable that contains the port number to start listening
	// on when in server mode, or the default port to use to connect to a server.
	EgoPortEnv = "EGO_PORT"

	// The environment variable that contains the default file name for the HTTPS
	// CERT file.
	EgoCertFileEnv = "EGO_CERT_FILE"

	// The environment variable that contains the default file name for the HTTPS
	// KEY file.
	EgoKeyFileEnv = "EGO_KEY_FILE"

	// If true, the server runs in insecure mode. This corresponds to the global
	// --insecure command line option.

	EgoInsecureEnv = "EGO_INSECURE"

	// The environment variable that contains "TRUE" if the client is allowed to
	// connect to a server using an insecure connection.
	EgoInsecureClientEnv = "EGO_INSECURE_CLIENT"

	// The port used to accept insecure connections when running as a secure server;
	// connections to this port are redirected to the secure port. If not specified,
	// the default is 80. This can also be set using the command line option
	// --insecure-port.
	EgoInsecurePortEnv = "EGO_INSECURE_PORT"

	// The environment variable that contains the default directory for the Ego
	// library files. If not specified, defaults to the user's home directory.
	EgoLibDirEnv = "EGO_LIB_DIR"

	// If this environment variable is set to "True" it suppresses the default
	// Ego behavior of checking for an ego library, and if one is not found,
	// creating it.
	EnvNoLibInitEnv = "EGO_NO_LIB_INIT"

	// The environment variable that contains the default directory for the Ego
	// runtime files. If not specified, defaults to the user's home directory.
	EgoRuntimeDirEnv = "EGO_RUNTIME_DIR"

	// The environment variable that contains the default directory for the Ego
	// cache files. If not specified, defaults to the user's home directory.
	EgoCacheDirEnv = "EGO_CACHE_DIR"

	// The environment variable that contains the default directory for the Ego
	// log files. If not specified, defaults to the user's home directory.
	// The environment variable that contains the default directory for the Ego
	// configuration files. If not specified, defaults to the user's home directory.
	EgoConfigDirEnv = "EGO_CONFIG_DIR"

	// What variable typing is performed at runtime? This corresponds to the --types
	// command line option.
	EgoTypesEnv = "EGO_TYPES"

	// The realm is the descriptive string sent back when a password challenge is
	// presented to the (web) client. This corresponds to the --realm command line option.
	EgoRealmEnv = "EGO_REALM"

	// This is the location/connection string for the database or file that contains
	// the authentication data. This corresponds to the --users command line option.
	EgoUsersEnv = "EGO_USERS"

	// If this is set to "True" then it enables tracing for Ego programs. This corresponds
	// to the --trace command line option.
	EgoTraceEnv = "EGO_TRACE"

	// The environment variable that contains the default database user. This corresponds
	// to the --user command line option.
	EgoUserEnv = "EGO_USERNAME"

	// The environment variable that contains the default database password. This corresponds
	// to the --password command line option.
	EgoPasswordEnv = "EGO_PASSWORD"

	// This is the environment variable that identifies the default logon server to use
	// for EGO LOGON commands. This corresponds to the --logon-server command line option.
	EgoLogonServerEnv = "EGO_LOGON_SERVER"

	// This environment variable contains a boolean value that tells if symbol table access
	// should be serialized or not. This overrides the defaults for RUN or SERVER modes.
	EgoSerializeSymbolTablesEnv = "EGO_SERIALIZE_SYMBOLTABLES"
)
