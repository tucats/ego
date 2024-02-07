package defs

// This section describes the profile keys used by Ego.
const (
	// The prefix for all configuration keys reserved to Ego.
	PrivilegedKeyPrefix = "ego."

	// The prefix for all runtime configuration keys.
	RuntimeKeyPrefix = PrivilegedKeyPrefix + "runtime."

	// The prefix for all server configuration keys.
	ServerKeyPrefix = PrivilegedKeyPrefix + "server."

	// The prefix for comiler configuration keys.
	CompilerKeyPrefix = PrivilegedKeyPrefix + "compiler."

	// The prefix for console configuration keys.
	ConsoleKeyPrefix = PrivilegedKeyPrefix + "console."

	// File system location used to locate services, lib,
	// and test directories.
	EgoPathSetting = RuntimeKeyPrefix + "path"

	// This setting is only used internally in Ego to indicate
	// that the console is operating interactive (i.e acting as
	// a REPL). It is not intended to be set by the user.
	AllowFunctionRedefinitionSetting = ConsoleKeyPrefix + "interactive"

	// What is the output format that should be used by
	// default for operations that could return either
	// "text" , "indented", or "json" output.
	OutputFormatSetting = ConsoleKeyPrefix + "output"

	// The base URL of the Ego server providing application services. This
	// is normally the same as the logon server, but may be set differently
	// if the logon services are hosted on a different server.
	ApplicationServerSetting = PrivilegedKeyPrefix + "application.server"

	// The base URL of the Ego server providing logon services.
	LogonServerSetting = PrivilegedKeyPrefix + "logon.server"

	// The last token created by a ego logon command, which
	// is used by default for server admin commands as well
	// as rest calls.
	LogonTokenSetting = PrivilegedKeyPrefix + "logon.token"

	// Stores the expiration date from the last login. This can be
	// used to detect an expired token and provide a better message
	// to the client user than "not authorized".
	LogonTokenExpirationSetting = PrivilegedKeyPrefix + "logon.token.expiration"

	// If specified, has the Go-style format string to be used for log
	// messages showing the time of the event.
	LogTimestampFormat = PrivilegedKeyPrefix + "log.timestamp"

	// This is the file name that is used to store the log file when it rolls
	// over and needs to be added to a zip archive. If not specified, log files
	// that are rolled off are deleted.
	LogArchiveSetting = PrivilegedKeyPrefix + "log.archive"

	// How many old logs do we maintain by default when in server mode?
	LogRetainCountSetting = PrivilegedKeyPrefix + "log.retain"

	// If specified, all filename references in ego programs (such as the
	// ReadFile() function) must start with this path, or it will be prefixed
	// with this path. This lets you limit where/how the files can be managed
	// by an ego program. This is especially important in server mode.
	SandboxPathSetting = PrivilegedKeyPrefix + "sandbox.path"

	// ConsoleHistorySetting is the name of the readline console history
	// file. This contains a line of text for each command previously read
	// using readline. If not specified in the profile, a default is used.
	ConsoleHistorySetting = ConsoleKeyPrefix + "history"

	// Should the interactive RUN mode exit when the user
	// enters a blank line on the console?
	ExitOnBlankSetting = ConsoleKeyPrefix + "on.blank"

	// Should the copyright message be omitted when in
	// interactive prompt mode?
	NoCopyrightSetting = ConsoleKeyPrefix + "no.copyright"

	// Should the interactive command input processor use
	// readline?
	UseReadline = ConsoleKeyPrefix + "readline"

	// File system location used to locate the lib directory. If
	// this setting isn't set, it defaults to the runtime.path
	// concatenated with "/lib". This lets the user set the lib
	// location to be a standard location like /usr/local/lib
	// if desired.
	EgoLibPathSetting = RuntimeKeyPrefix + "path.lib"

	// Do we automatically process non-success ErrorResponse payloads from
	// client REST calls as if they were the return code value? Default is
	// true.
	RestClientErrorSetting = RuntimeKeyPrefix + "rest.errors"

	// Specify if the automatic creation of the lib/ directory
	// should be suppressed.
	SuppressLibraryInitSetting = RuntimeKeyPrefix + "supress.library.init"

	// If true, the util.Exec() function can be executed to run an arbitrary
	// native shell command. This defaults to being disabled.
	ExecPermittedSetting = RuntimeKeyPrefix + "exec"

	// Default allocation factor to set on symbol table create/expand
	// operations. Larger numbers are more efficient for larger symbol
	// tables, but too large a number wastes time and memory.
	SymbolTableAllocationSetting = RuntimeKeyPrefix + "symbol.allocation"

	// If true, functions that return multiple values including an
	// error that do not assign that error to a value will result in
	// the error being thrown.
	ThrowUncheckedErrorsSetting = RuntimeKeyPrefix + "unchecked.errors"

	// If true, then an error with a message string that starts with "!" will
	// cause an actual go panic abend.
	RuntimePanicsSetting = RuntimeKeyPrefix + "panics"

	// If true, the TRACE operation will print the full stack instead of
	// a shorter single-line version.
	FullStackTraceSetting = RuntimeKeyPrefix + "stack.trace"

	// Do we normalize the case of all symbols to a common
	// (lower) case string. If not true, symbol names are
	// case-sensitive.
	CaseNormalizedSetting = CompilerKeyPrefix + "normalized"

	// If true, the script language includes language
	// extensions such as print, call, try/catch.
	ExtensionsEnabledSetting = CompilerKeyPrefix + "extensions"

	// Should an interactive session automatically import
	// all the pre-defined packages?
	AutoImportSetting = CompilerKeyPrefix + "import"

	// Set to true if the full stack should be listed during
	// tracing.
	FullStackListingSetting = CompilerKeyPrefix + "full.stack"

	// Should the bytecode generator attempt an optimization pass?
	OptimizerSetting = CompilerKeyPrefix + "optimize"

	// Should the Ego program(s) be run with "strict" or
	// "dynamic" typing? The default is "dynamic".
	StaticTypesSetting = CompilerKeyPrefix + "types"

	// The default user if no userdatabase has been initialized
	// yet. This is a strong of the form "user:password", which
	// is defined as the root user.
	DefaultCredentialSetting = ServerKeyPrefix + "default-credential"

	// If present, this user is always assigned super-user (root)
	// privileges regardless of the userdata settings. This can be
	// used to access an encrypted user data file.
	LogonSuperuserSetting = ServerKeyPrefix + "superuser"

	// The file system location where the user database is stored.
	LogonUserdataSetting = ServerKeyPrefix + "userdata"

	// The encryption key for the userdata file. If not present,
	// the file is not encrypted and is readable json.
	LogonUserdataKeySetting = ServerKeyPrefix + "userdata.key"

	// The host that provides authentication services on our behalf. If
	// not specified, the current server is also the authentication service.
	ServerAuthoritySetting = ServerKeyPrefix + "authority"

	// The number of seconds between scans to see if cached authentication
	// data from a remote authoiry server should be checked for expired
	// values. The default is every 180 seconds (3 minutes).
	AuthCacheScanSetting = ServerKeyPrefix + "auth.cache.scan"

	// Indicator if /service requests are executed by a child process instead
	// of in-process.
	ChildServicesSetting = ServerKeyPrefix + "child.services"

	// Optional override for the location where request payloads are stored.
	ChildRequestDirSetting = ServerKeyPrefix + "child.services.dir"

	// If a positive integer, this limits the number of simultaneous child
	// processes that can be spawned to handle requests.
	ChildRequestLimitSetting = ServerKeyPrefix + "child.services.limit"

	// Flag to indicate if the child response payload files should be retained
	// for debugging, etc.  By default they are deleted when the request completes.
	ChildRequestRetainSetting = ServerKeyPrefix + "child.services.retain"

	// Duration string indicating how long we wait for an available child
	// process before returning an error.
	ChildRequestTimeoutSetting = ServerKeyPrefix + "child.services.timeout"

	// The URL path for the tables database functionality.
	TablesServerDatabase = ServerKeyPrefix + "database.url"

	// The user:password credentials to use with the local tables database.
	TablesServerDatabaseCredentials = ServerKeyPrefix + "database.credentials"

	// The name of the tables database in the local database store (schemas
	// are used to partition this database by Ego username).
	TablesServerDatabaseName = ServerKeyPrefix + "database.name"

	// Boolean indicating if the communication with the tables database
	// should be done using SSL secured communications.
	TablesServerDatabaseSSLMode = ServerKeyPrefix + "database.ssl"

	// The URL path for the tables database functionality.
	TablesServerEmptyFilterError = ServerKeyPrefix + "database.empty.filter.error"

	// The URL path for the tables database functionality.
	TablesServerEmptyRowsetError = ServerKeyPrefix + "database.empty.rowset.error"

	// If true, the insert of a row _must_ specify all values in the table.
	TableServerPartialInsertError = ServerKeyPrefix + "database.partial.insert.error"

	// The key string used to encrypt authentication tokens.
	ServerTokenKeySetting = ServerKeyPrefix + "token.key"

	// A string indicating the duration of a token before it is
	// considered expired. Examples are "15m" or "24h".
	ServerTokenExpirationSetting = ServerKeyPrefix + "token.expiration"

	// A string indicating the default logging to be assigned to a server that is
	// started without an explicit --log setting.
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
	ExitOnBlankSetting:              false,
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
}
