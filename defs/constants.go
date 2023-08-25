package defs

// The authorization scheme attached to the bearer token
// in REST calls.
const AuthScheme = "bearer "

// Name of the default local host, in TCP/IP standards.
const LocalHost = "localhost"

// DefaultUserdataFileName is the default file system name of
// the user database file, if not specified by the user.
const DefaultUserdataFileName = "sqlite3://users.db"

// The environment variable that defines where the runtime files are
// to be found.
const EgoPathEnv = "EGO_PATH"

// The subdirectory in "EGO_PATH" where the .ego runtime files and assets
// are found.
const LibPathName = "lib"

// The environment variable that contains the path to which the log file
// is written.
const EgoLogEnv = "EGO_LOG"

// The environment variable that contains the name(s) of the loggers that
// are to be enabled by default at startup (before command line processing).
const EgoDefaultLogging = "EGO_DEFAULT_LOGGING"

// The environment variable that contains the name to use for writing log
// file messages. If not specified, defaults to writing to stdout.
const EgoDefaultLogFileName = "EGO_LOG_FILE"

// The file extension for Ego programs".
const EgoFilenameExtension = ".ego"

// This is the name of a column automatically added to tables created using
// the 'tables' REST API.
const RowIDName = "_row_id_"

// This is the name for objects that otherwise have no name.
const Anon = "<anon>"

// This section describes the profile keys used by Ego.
const (
	// The prefix for all configuration keys reserved to Ego.
	PrivilegedKeyPrefix = "ego."

	// File system location used to locate services, lib,
	// and test directories.
	EgoPathSetting = PrivilegedKeyPrefix + "runtime.path"

	// File system location used to locate the lib directory. If
	// this setting isn't set, it defaults to the runtime.path
	// concatenated with "/lib". This lets the user set the lib
	// location to be a standard location like /usr/local/lib
	// if desired.
	EgoLibPathSetting = PrivilegedKeyPrefix + "runtime.path.lib"

	// Do we normalize the case of all symbols to a common
	// (lower) case string. If not true, symbol names are
	// case-sensitive.
	CaseNormalizedSetting = PrivilegedKeyPrefix + "compiler.normalized"

	// What is the output format that should be used by
	// default for operations that could return either
	// "text" , "indented", or "json" output.
	OutputFormatSetting = PrivilegedKeyPrefix + "console.format"

	// ConsoleHistorySetting is the name of the readline console history
	// file. This contains a line of text for each command previously read
	// using readline. If not specified in the profile, a default is used.
	ConsoleHistorySetting = PrivilegedKeyPrefix + "console.history"

	// If true, the script language includes language
	// extensions such as print, call, try/catch.
	ExtensionsEnabledSetting = PrivilegedKeyPrefix + "compiler.extensions"

	// Should an interactive session automatically import
	// all the pre-defined packages?
	AutoImportSetting = PrivilegedKeyPrefix + "compiler.import"

	// Should the interactive RUN mode exit when the user
	// enters a blank line on the console?
	ExitOnBlankSetting = PrivilegedKeyPrefix + "console.exit.on.blank"

	// Should the copyright message be omitted when in
	// interactive prompt mode?
	NoCopyrightSetting = PrivilegedKeyPrefix + "console.no.copyright"

	// Should the interactive command input processor use
	// readline?
	UseReadline = PrivilegedKeyPrefix + "console.readline"

	// Set to true if the full stack should be listed during
	// tracing.
	FullStackListingSetting = PrivilegedKeyPrefix + "compiler.full.stack"

	// Should the bytecode generator attempt an optimization pass?
	OptimizerSetting = PrivilegedKeyPrefix + "compiler.optimize"

	// Should the Ego program(s) be run with "strict" or
	// "dynamic" typing? The default is "dynamic".
	StaticTypesSetting = PrivilegedKeyPrefix + "compiler.types"

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

	// The default user if no userdatabase has been initialized
	// yet. This is a strong of the form "user:password", which
	// is defined as the root user.
	DefaultCredentialSetting = PrivilegedKeyPrefix + "server.default-credential"

	// If present, this user is always assigned super-user (root)
	// privileges regardless of the userdata settings. This can be
	// used to access an encrypted user data file.
	LogonSuperuserSetting = PrivilegedKeyPrefix + "server.superuser"

	// The file system location where the user database is stored.
	LogonUserdataSetting = PrivilegedKeyPrefix + "server.userdata"

	// The encryption key for the userdata file. If not present,
	// the file is not encrypted and is readable json.
	LogonUserdataKeySetting = PrivilegedKeyPrefix + "server.userdata.key"

	// The host that provides authentication services on our behalf. If
	// not specified, the current server is also the authentication service.
	ServerAuthoritySetting = PrivilegedKeyPrefix + "server.authority"

	// The number of seconds between scans to see if cached authentication
	// data from a remote authoiry server should be checked for expired
	// values. The default is every 180 seconds (3 minutes).
	AuthCacheScanSetting = PrivilegedKeyPrefix + "server.auth.cache.scan"

	// The URL path for the tables database functionality.
	TablesServerDatabase = PrivilegedKeyPrefix + "server.database.url"

	// The user:password credentials to use with the local tables database.
	TablesServerDatabaseCredentials = PrivilegedKeyPrefix + "server.database.credentials"

	// The name of the tables database in the local database store (schemas
	// are used to partition this database by Ego username).
	TablesServerDatabaseName = PrivilegedKeyPrefix + "server.database.name"

	// Boolean indicating if the communication with the tables database
	// should be done using SSL secured communications.
	TablesServerDatabaseSSLMode = PrivilegedKeyPrefix + "server.database.ssl"

	// The URL path for the tables database functionality.
	TablesServerEmptyFilterError = PrivilegedKeyPrefix + "server.database.empty.filter.error"

	// The URL path for the tables database functionality.
	TablesServerEmptyRowsetError = PrivilegedKeyPrefix + "server.database.empty.rowset.error"

	// If true, the insert of a row _must_ specify all values in the table.
	TableServerPartialInsertError = PrivilegedKeyPrefix + "server.database.partial.insert.error"

	// The key string used to encrypt authentication tokens.
	ServerTokenKeySetting = PrivilegedKeyPrefix + "server.token.key"

	// A string indicating the duration of a token before it is
	// considered expired. Examples are "15m" or "24h".
	ServerTokenExpirationSetting = PrivilegedKeyPrefix + "server.token.expiration"

	// A string indicating the default logging to be assigned to a server that is
	// started without an explicit --log setting.
	ServerDefaultLogSetting = PrivilegedKeyPrefix + "server.default.logging"

	// How many old logs do we maintain by default when in server mode?
	LogRetainCountSetting = PrivilegedKeyPrefix + "server.retain.log.count"

	// If true, the util.Exec() function can be executed to run an arbitrary
	// native shell command. This defaults to being disabled.
	ExecPermittedSetting = PrivilegedKeyPrefix + "runtime.exec"

	// Default allocation factor to set on symbol table create/expand
	// operations. Larger numbers are more efficient for larger symbol
	// tables, but too large a number wastes time and memory.
	SymbolTableAllocationSetting = PrivilegedKeyPrefix + "runtime.symbol.allocation"

	// If true, functions that return multiple values including an
	// error that do not assign that error to a value will result in
	// the error being thrown.
	ThrowUncheckedErrorsSetting = PrivilegedKeyPrefix + "runtime.unchecked.errors"

	// If true, then an error with a message string that starts with "!" will
	// cause an actual go panic abend.
	RuntimePanicsSetting = PrivilegedKeyPrefix + "runtime.panics"

	// If true, the TRACE operation will print the full stack instead of
	// a shorter single-line version.
	FullStackTraceSetting = PrivilegedKeyPrefix + "runtime.stack.trace"

	// If specified, has the Go-style format string to be used for log
	// messages showing the time of the event.
	LogTimestampFormat = PrivilegedKeyPrefix + "log.timestamp"

	// If specified, all filename references in ego programs (such as the
	// ReadFile() function) must start with this path, or it will be prefixed
	// with this path. This lets you limit where/how the files can be managed
	// by an ego program. This is especially important in server mode.
	SandboxPathSetting = PrivilegedKeyPrefix + "sandbox.path"

	// PidDirectorySettings has the path used to store and find PID files for
	// server invocations and management.
	PidDirectorySetting = PrivilegedKeyPrefix + "server.piddir"

	// If true, the default state for staring an Ego is to not require HTTPS/SSL
	// but rather run in "insecure" mode.
	InsecureServerSetting = PrivilegedKeyPrefix + "server.insecure"

	// Maximum cache size for server cache. The default is zero, no caching
	// performed.
	MaxCacheSizeSetting = PrivilegedKeyPrefix + "server.cache.size"

	// Do we automatically process non-success ErrorResponse payloads from
	// client REST calls as if they were the return code value? Default is
	// true.
	RestClientErrorSetting = PrivilegedKeyPrefix + "runtime.rest.errors"
)

// This section contains the names of the command-line options. These often
// (but not always) have parallels in the settings above. Settings typically
// have a structured name (ego.compiler.autoimport) while the option name is
// Unix shell-friendly (auto-import).
const (
	AutoImportOption      = "auto-import"
	DisassembleOption     = "disassemble"
	FullSymbolScopeOption = "full-symbol-scope"
	TypingOption          = "types"
	SymbolTableSizeOption = "symbol-allocation"
	OptimizerOption       = "optimize"
)

// Agent identifiers for REST calls, which indicate the role of the client.
const (
	AdminAgent  = "admin"
	ClientAgent = "rest client"
	LogonAgent  = "logon"
	StatusAgent = "status"
	TableAgent  = "tables"
)

const (
	True    = "true"
	False   = "false"
	Any     = "any"
	Strict  = "strict"
	Relaxed = "relaxed"
	Dynamic = "dynamic"
	Main    = "main"
)

const (
	ByteCodeReflectionTypeString = "<*bytecode.ByteCode Value>"

	TypeCheckingVariable   = InvisiblePrefix + "type_checking"
	StrictTypeEnforcement  = 0
	RelaxedTypeEnforcement = 1
	NoTypeEnforcement      = 2

	InvisiblePrefix          = "__"
	ThisVariable             = InvisiblePrefix + "this"
	MainVariable             = InvisiblePrefix + "main"
	ErrorVariable            = InvisiblePrefix + "error"
	ArgumentListVariable     = InvisiblePrefix + "args"
	CLIArgumentListVariable  = InvisiblePrefix + "cli_args"
	ModeVariable             = InvisiblePrefix + "exec_mode"
	DebugServicePathVariable = InvisiblePrefix + "debug_service_path"
	PathsVariable            = InvisiblePrefix + "paths"
	LocalizationVariable     = InvisiblePrefix + "localization"
	LineVariable             = InvisiblePrefix + "line"
	ExtensionsVariable       = InvisiblePrefix + "extensions"
	ModuleVariable           = InvisiblePrefix + "module"
	ResponseHeaderVariable   = InvisiblePrefix + "response_headers"
	RestStatusVariable       = InvisiblePrefix + "rest_status"
	DiscardedVariable        = "_"
	ReadonlyVariablePrefix   = "_"
	SuperUserVariable        = ReadonlyVariablePrefix + "superuser"
	PasswordVariable         = ReadonlyVariablePrefix + "password"
	TokenVariable            = ReadonlyVariablePrefix + "token"
	TokenValidVariable       = ReadonlyVariablePrefix + "token_valid"
	VersionName              = ReadonlyVariablePrefix + "version"
	CopyrightVariable        = ReadonlyVariablePrefix + "copyright"
	InstanceUUIDVariable     = ReadonlyVariablePrefix + "server_instance"
	PidVariable              = ReadonlyVariablePrefix + "pid"
	SessionVariable          = ReadonlyVariablePrefix + "session"
	MethodVariable           = ReadonlyVariablePrefix + "method"
	BuildTimeVariable        = ReadonlyVariablePrefix + "buildtime"
	PlatformVariable         = ReadonlyVariablePrefix + "platform"
	StartTimeVariable        = ReadonlyVariablePrefix + "start_time"
	RequestorVariable        = ReadonlyVariablePrefix + "requestor"
	HeadersMapVariable       = ReadonlyVariablePrefix + "headers"
	ParametersVariable       = ReadonlyVariablePrefix + "parms"
	JSONMediaVariable        = ReadonlyVariablePrefix + "json"
)

// ValidSettings describes the list of valid settings, and whether they can be set by the
// command line.
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
}

const (
	APIVersion = 1
)
