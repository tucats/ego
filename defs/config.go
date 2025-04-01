package defs

// This section describes the profile keys used by Ego.
const (
	// The prefix for all configuration keys reserved to Ego. These names must be valid names
	// and there are access control limits on when/where these can be modified. Configuration
	// values that do not start with this value are available to the user, and are restricted
	// or controlled in any way.
	PrivilegedKeyPrefix = "ego."

	// The base URL of the Ego server providing application services. Often, this is the same
	// as the logon server, in which case an explicit application server setting is not needed.
	// However, it may be set to a different value if the logon services are hosted on a
	// different server or port.
	ApplicationServerSetting = PrivilegedKeyPrefix + "application.server"

	// LOGON CONFIGURATION KEYS
	// The prefix for all logon configuration keys.
	LogonKeyPrefix = PrivilegedKeyPrefix + "logon."

	// The base URL of the Ego server providing logon services.
	LogonServerSetting = LogonKeyPrefix + "server"

	// The value of the token created by a ego logon command, which is used by
	// default for server admin commands as well as rest calls.
	LogonTokenSetting = LogonKeyPrefix + "token"

	// Stores the expiration date from the last login. This can be used to detect
	// an expired token and provide a better message to the client user than
	// "not authorized".
	LogonTokenExpirationSetting = LogonKeyPrefix + "token.expiration"

	// LOG CONFIGURATION KEYS
	// The prefix for all log-related configuration keys.
	LogKeyPrefix = PrivilegedKeyPrefix + "log."

	// If specified, has the Go-style format string to be used for log
	// messages showing the time of the event.
	LogTimestampFormat = LogKeyPrefix + "timestamp"

	// This is the file name that is used to store the log file when it
	// rolls over and needs to be added to a zip archive. If not specified,
	// log files that are rolled off are deleted.
	LogArchiveSetting = LogKeyPrefix + "archive"

	// How many old logs do we maintain by default when in server mode?
	LogRetainCountSetting = LogKeyPrefix + "retain"

	// The prefix for all configuration keys.
	ConfigKeyPrefix = PrivilegedKeyPrefix + "config."

	// The prefix for all database configuration keys.
	DatabaseKeyPrefix = PrivilegedKeyPrefix + "database."

	// RUNTIME CONFIGURATION KEYS
	// The prefix for all runtime configuration keys.
	RuntimeKeyPrefix = PrivilegedKeyPrefix + "runtime."

	// File system location used to locate services, lib,
	// and test directories.
	EgoPathSetting = RuntimeKeyPrefix + "path"

	// If specified, all filename references in ego programs (such as the
	// ReadFile() function) must start with this path, or it will be prefixed
	// with this path. This lets you limit where/how the files can be managed
	// by an ego program. This is especially important in server mode.
	SandboxPathSetting = RuntimeKeyPrefix + "sandbox.path"

	// File system location used to locate the lib directory. If this setting
	// isn't defined, it defaults to the runtime.path  concatenated with "/lib".
	// This lets the user set the lib location to be a standard location like
	// /usr/local/lib if desired.
	EgoLibPathSetting = EgoPathSetting + ".lib"

	// Specify if the automatic creation of the lib/ directory
	// should be suppressed.
	SuppressLibraryInitSetting = RuntimeKeyPrefix + "suppress.library.init"

	// If true, the util.Exec() function can be executed to run an arbitrary
	// native shell command. This defaults to being disabled.
	ExecPermittedSetting = RuntimeKeyPrefix + "exec"

	// If true, the REST client will not require SSL/HTTPS for connections.
	// This is useful for testing and development but should not be used in
	// production.
	InsecureClientSetting = RuntimeKeyPrefix + "insecure.client"

	// If true, cast operations that cause a loss of data (casting a value that
	// is larger than 255 into a byte, etc.) will result in an error.
	PrecisionErrorSetting = RuntimeKeyPrefix + "precision.error"

	// Default allocation factor to set on symbol table create/expand
	// operations. Larger numbers are more efficient for larger symbol
	// tables, but too large a number wastes time and memory.
	SymbolTableAllocationSetting = RuntimeKeyPrefix + "symbol.allocation"

	// If true, functions that return multiple values including an error that
	// do not assign that error to a value will result in an error return.
	ThrowUncheckedErrorsSetting = RuntimeKeyPrefix + "unchecked.errors"

	// If true, then an invocation of the panic() function will result in
	// an actual native Go panic, ending the process immediately.
	RuntimePanicsSetting = RuntimeKeyPrefix + "panics"

	// If true, functions do not create symbol scope barriers. This is generally
	// only true when running in test mode.
	RuntimeDeepScopeSetting = RuntimeKeyPrefix + "deep.scope"

	// If true, the TRACE operation will print the full stack instead of
	// a shorter single-line version.
	FullStackTraceSetting = RuntimeKeyPrefix + "stack.trace"

	// REST CONFIGURATION KEYS
	// The prefix for all REST  configuration keys.
	RestKeyPrefix = RuntimeKeyPrefix + "rest."

	// Do we automatically process non-success ErrorResponse payloads from
	// client REST calls as if they were the return code value? Default is
	// true.
	RestClientErrorSetting = RestKeyPrefix + "errors"

	// Is there a timeout on REST client operations? If specified, must be a
	// valid duration string.
	RestClientTimeoutSetting = RestKeyPrefix + "timeout"

	// If set to "system", we do not load a server cert file to trust, and
	// depend on the default system trust store.
	RestClientServerCert = RestKeyPrefix + "server.cert"

	// COMPILER CONFIGURATION KEYS
	// The prefix for compiler configuration keys.
	CompilerKeyPrefix = PrivilegedKeyPrefix + "compiler."

	// Do we normalize the case of all symbols to a common (lower) case
	// string. If not true, symbol names are case-sensitive.
	CaseNormalizedSetting = CompilerKeyPrefix + "normalized"

	// If true, the script language includes language  extensions such as
	// print, call, try/catch.
	ExtensionsEnabledSetting = CompilerKeyPrefix + "extensions"

	// Should an interactive session automatically import all the pre-
	// defined packages?
	AutoImportSetting = CompilerKeyPrefix + "import"

	// Set to true if the full stack should be listed during tracing.
	FullStackListingSetting = CompilerKeyPrefix + "full.stack"

	// Should the bytecode generator attempt an optimization pass?
	OptimizerSetting = CompilerKeyPrefix + "optimize"

	// Should the Ego program(s) be run with "strict" or  "dynamic" typing?
	// The default is "dynamic".
	StaticTypesSetting = CompilerKeyPrefix + "types"

	// Should a variable that is declared but never used be an error?
	UnusedVarsSetting = CompilerKeyPrefix + "unused.var.error"

	// Should the compiler report an unknown symbol error without waiting
	// for the runtime symbol table manager to report it? This is currently
	// somewhat experimental.
	UnknownVarSetting = CompilerKeyPrefix + "unknown.var.error"

	// When true, compiler logging includes tracking  variable usage scope.
	UnusedVarLoggingSetting = CompilerKeyPrefix + "var.usage.logging"

	// CONSOLE CONFIGURATION KEYS
	// The prefix for console configuration keys.
	ConsoleKeyPrefix = PrivilegedKeyPrefix + "console."

	// ConsoleHistorySetting is the name of the readline console history
	// file. This contains a line of text for each command previously read
	// using readline. If not specified in the profile, a default is used.
	ConsoleHistorySetting = ConsoleKeyPrefix + "history"

	// Should the copyright message be omitted when in interactive mode?
	NoCopyrightSetting = ConsoleKeyPrefix + "no.copyright"

	// Should the interactive command input processor use the readline
	// library?
	UseReadline = ConsoleKeyPrefix + "readline"

	// This setting is only used internally in Ego to indicate that the
	// console is operating interactively (i.e acting as a REPL). It is not
	// intended to be set by the user.
	AllowFunctionRedefinitionSetting = ConsoleKeyPrefix + "interactive"

	// What is the output format that should be used by default for operations
	// that could return either "text" , "indented", or "json" output.
	OutputFormatSetting = ConsoleKeyPrefix + "output"

	// What is the log format that should be used by default for logging.
	// Valid choices are "text", "json", and "indented".
	LogFormatSetting = ConsoleKeyPrefix + "log"

	// TABLE CONFIGURATION KEYS
	//The prefix for database table configuration keys.
	TableKeyPrefix = PrivilegedKeyPrefix + "table."

	// If true, command lines that contain "foo.bar" table names will
	// assume the dsn is foo and the table is bar.
	TableAutoparseDSN = TableKeyPrefix + "autoparse.dsn"

	// The default data source name to use for table commands. If not specified,
	// no default is used.
	DefaultDataSourceSetting = TableKeyPrefix + "default.dsn"

	// SERVER CONFIGURATION KEYS
	// The prefix for all server configuration keys.
	ServerKeyPrefix = PrivilegedKeyPrefix + "server."

	// If true, the REST payload will report the fully-qualified domain name for
	// the server. Otherwise, the "shortname" is used, which is the default case.
	ServerReportFQDNSetting = ServerKeyPrefix + "report.fqdn"

	// The default user if no user database has been initialized yet. This is a
	// string of the form "user:password", which is defined as the root user.
	DefaultCredentialSetting = ServerKeyPrefix + "default.credential"

	// If present, this user is always assigned super-user (root) privileges
	// regardless of the user authorization settings.
	LogonSuperuserSetting = ServerKeyPrefix + "superuser"

	// The file system location where the user database is stored.
	LogonUserdataSetting = ServerKeyPrefix + "userdata"

	// The encryption key for the userdata file. If not present the file is
	// not encrypted and is readable json. Note that this should only be used
	// when initially developing and testing a server configuration, but is
	// not safe in production deployments.
	LogonUserdataKeySetting = ServerKeyPrefix + "userdata.key"

	// Interval for server memory usage logging. If not specified, the default
	// is every three minutes. If there is no server activity during an interval,
	// no logging is done, so making this too long will risk dropping data. But
	// making it too frequent will generate logs of logging.
	MemoryLogIntervalSetting = ServerKeyPrefix + "memory.log.interval"

	// The host that provides authentication services on our behalf. If not
	// specified, the current server is also the authentication service.
	ServerAuthoritySetting = ServerKeyPrefix + "authority"

	// The number of seconds between scans to see if cached authentication
	// data from a remote authorization server should be checked for expired
	// values. The default is every 180 seconds (3 minutes).
	AuthCacheScanSetting = ServerKeyPrefix + "auth.cache.scan"

	// If true, when REST logging is enabled, the server log itself will be
	// logged as a response payload to the /log service request. This is
	// normally off and should only be enable when debugging logging.
	ServerLogResponseSetting = ServerKeyPrefix + "log.response"

	// Indicator if /service requests are executed by a child process instead
	// of in-process.
	ChildServicesSetting = ServerKeyPrefix + "child.services"

	// Prefix for server child process configuration settings.
	ChildServicesKeyPrefix = ChildServicesSetting + "."

	// Optional override for the location where request payloads are stored.
	ChildRequestDirSetting = ChildServicesKeyPrefix + "dir"

	// If a positive integer, this limits the number of simultaneous child
	// processes that can be spawned to handle requests.
	ChildRequestLimitSetting = ChildServicesKeyPrefix + "limit"

	// Flag to indicate if the child response payload files should be retained
	// for debugging, etc.  By default they are deleted when the request completes.
	ChildRequestRetainSetting = ChildServicesKeyPrefix + "retain"

	// Duration string indicating how long we wait for an available child
	// process before returning an error.
	ChildRequestTimeoutSetting = ChildServicesKeyPrefix + "timeout"

	// The URL path for the tables database functionality.
	ServerDatabaseKeyPrefix = ServerKeyPrefix + "database."

	// The URL path for the tables database functionality.
	DatabaseServerDatabase = ServerDatabaseKeyPrefix + "url"

	// The user:password credentials to use with the local tables database.
	DatabaseServerDatabaseCredentials = ServerDatabaseKeyPrefix + "credentials"

	// The name of the tables database in the local database store (schemas
	// are used to partition this database by Ego username).
	DatabaseServerDatabaseName = ServerDatabaseKeyPrefix + "name"

	// Boolean indicating if the communication with the tables database
	// should be done using SSL secured communications.
	DatabaseServerDatabaseSSLMode = ServerDatabaseKeyPrefix + "ssl"

	// The URL path for the tables database functionality.
	DatabaseServerEmptyFilterError = ServerDatabaseKeyPrefix + "empty.filter.error"
	TablesServerDatabase           = ServerDatabaseKeyPrefix + "url"

	// The user:password credentials to use with the local tables database.
	TablesServerDatabaseCredentials = ServerDatabaseKeyPrefix + "credentials"

	// The name of the tables database in the local database store (schemas
	// are used to partition this database by Ego username).
	TablesServerDatabaseName = ServerDatabaseKeyPrefix + "name"

	// Boolean indicating if the communication with the tables database
	// should be done using SSL secured communications.
	TablesServerDatabaseSSLMode = ServerDatabaseKeyPrefix + "ssl"

	// The URL path for the tables database functionality.
	TablesServerEmptyFilterError = ServerDatabaseKeyPrefix + "empty.filter.error"

	// The URL path for the tables database functionality.
	TablesServerEmptyRowsetError = ServerDatabaseKeyPrefix + "empty.rowset.error"

	// If true, the insert of a row _must_ specify all values in the table.
	TableServerPartialInsertError = ServerDatabaseKeyPrefix + "partial.insert.error"

	// The key string used to encrypt authentication tokens.
	ServerTokenKeySetting = ServerKeyPrefix + "token.key"

	// A string indicating the duration of a token before it is considered
	// expired. Examples are "15m" or "24h".
	ServerTokenExpirationSetting = ServerKeyPrefix + "token.expiration"

	// A string indicating the default logging to be assigned to a server
	// that is started without an explicit --log setting.
	ServerDefaultLogSetting = ServerKeyPrefix + "default.logging"

	// PidDirectorySettings has the path used to store and find PID files for
	// server invocations and management.
	PidDirectorySetting = ServerKeyPrefix + "piddir"

	// If true, the default state for staring an Ego is to not require HTTPS/SSL
	// but rather run in "insecure" mode.
	InsecureServerSetting = ServerKeyPrefix + "insecure"

	// Maximum cache size for server cache. The default is zero, no caching
	// performed.
	MaxCacheSizeSetting = ServerKeyPrefix + "cache.size"
)

// ValidSettings describes the list of valid settings, and whether they can be set by the
// command line. This is used by the config subcommand to determine if the setting is allowed
// to be set by the user (this test is only done for settings with the "ego." prefix.)
var ValidSettings map[string]bool = map[string]bool{
	EgoPathSetting:                  true,
	EgoLibPathSetting:               true,
	CaseNormalizedSetting:           true,
	OutputFormatSetting:             true,
	ExtensionsEnabledSetting:        true,
	AutoImportSetting:               true,
	NoCopyrightSetting:              false,
	UseReadline:                     true,
	FullStackListingSetting:         true,
	StaticTypesSetting:              true,
	ApplicationServerSetting:        false,
	LogonServerSetting:              true,
	LogonTokenSetting:               false,
	LogonTokenExpirationSetting:     false,
	DefaultCredentialSetting:        true,
	LogonSuperuserSetting:           true,
	LogonUserdataSetting:            true,
	LogonUserdataKeySetting:         true,
	TablesServerDatabase:            true,
	TablesServerDatabaseCredentials: true,
	TablesServerDatabaseName:        true,
	TablesServerDatabaseSSLMode:     true,
	ServerTokenKeySetting:           true,
	ServerTokenExpirationSetting:    true,
	ThrowUncheckedErrorsSetting:     true,
	FullStackTraceSetting:           true,
	LogTimestampFormat:              true,
	LogArchiveSetting:               true,
	SandboxPathSetting:              true,
	PidDirectorySetting:             true,
	RestClientTimeoutSetting:        true,
	InsecureServerSetting:           true,
	MaxCacheSizeSetting:             true,
	RestClientErrorSetting:          true,
	LogRetainCountSetting:           true,
	RuntimePanicsSetting:            true,
	TablesServerEmptyFilterError:    true,
	TablesServerEmptyRowsetError:    true,
	ServerDefaultLogSetting:         true,
	TableServerPartialInsertError:   true,
	SymbolTableAllocationSetting:    true,
	ExecPermittedSetting:            true,
	OptimizerSetting:                true,
	ChildServicesSetting:            true,
	ChildRequestDirSetting:          true,
	ChildRequestRetainSetting:       true,
	ChildRequestLimitSetting:        true,
	DefaultDataSourceSetting:        true,
	RestClientServerCert:            true,
	RuntimeDeepScopeSetting:         true,
	TableAutoparseDSN:               true,
	PrecisionErrorSetting:           true,
	UnusedVarsSetting:               true,
	UnknownVarSetting:               true,
	UnusedVarLoggingSetting:         true,
	ServerReportFQDNSetting:         true,
	LogFormatSetting:                true,
	MemoryLogIntervalSetting:        true,
}

// RestrictedSettings is a list of settings that cannot be read using the
// Ego package for reading profile data. This prevents user injection of code
// that could compromise security. Note that not all settings are in this
// category, only those that contains keys or other secure information.
var RestrictedSettings map[string]bool = map[string]bool{
	ServerTokenKeySetting:           true,
	LogonTokenSetting:               true,
	LogonUserdataKeySetting:         true,
	TablesServerDatabaseCredentials: true,
	TablesServerDatabase:            true,
	ConsoleHistorySetting:           true,
	LogArchiveSetting:               true,
	EgoDefaultLogFileName:           true,
	RestClientServerCert:            true,
}
