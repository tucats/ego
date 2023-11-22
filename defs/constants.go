package defs

// The authorization scheme attached to the bearer token
// in REST calls.
const AuthScheme = "bearer "

// Name of the default local host, in TCP/IP standards.
const LocalHost = "localhost"

// DefaultUserdataFileName is the default file system name of
// the user database file, if not specified by the user.
const DefaultUserdataFileName = "sqlite3://users.db"

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
)

// This section contains constants used by file operations.
const (
	// The subdirectory in "EGO_PATH" where the .ego runtime files and assets
	// are found.
	LibPathName = "lib"

	// The file extension for Ego programs".
	EgoFilenameExtension = ".ego"
)

// This section contains reserved symbol table names.
const (
	// This prefix means the symbol is both readonly and never displayed to the user
	// when a symbol table is printed, etc.
	InvisiblePrefix = "__"

	// This prefix means the symbol is a readonly value. It can be set only once, when
	// it is first created.
	ReadonlyVariablePrefix = "_"

	// This is the name of a column automatically added to tables created using
	// the 'tables' REST API.
	RowIDName = ReadonlyVariablePrefix + "row_id_"

	// This is the name of a variable that holds the REST response body on behalf
	// of an Ego program executing as a service. This special variable name is used
	// as if it was part of the local symbol table of the service, but is always
	// stored in the parent table, so it is accessible to the rest handler once the
	// service has completed.
	RestResponseName = InvisiblePrefix + "rest_response"

	// This contains the global variable that reports if type checking is strict,
	// relaxed, or dynamic.
	TypeCheckingVariable = InvisiblePrefix + "type_checking"

	// This contains the pointer to the type receiver object that was used to invoke
	// a function based on an object interface. It allows the executing function to
	// fetch the receiver object without knowing it's name or type.
	ThisVariable = InvisiblePrefix + "this"

	// This holds the name of the main program. This is "main" by default.
	MainVariable = InvisiblePrefix + "main"

	// This reports the last error value that was returned by an opcode or runtime
	// function. It is used by the "try" and "catch" operations.
	ErrorVariable = InvisiblePrefix + "error"

	// This is an array of values that contain the argument passed to a function.
	ArgumentListVariable = InvisiblePrefix + "args"

	// This is an array of strings the command line arguments passed to the Ego invocation
	// by the shell.
	CLIArgumentListVariable = InvisiblePrefix + "cli_args"

	// This is a global variable that indicates if Ego is running in interactive, server,
	// or test mode.
	ModeVariable = InvisiblePrefix + "exec_mode"

	// This global variable contains the name of the REST server handler that is to be
	// debugged. This is used to trigger the interactive Ego debugger when a service
	// call starts to run the named service handler. This requires that the server be
	// run with the --debug flag.
	DebugServicePathVariable = InvisiblePrefix + "debug_service_path"

	// This global variable holds the preferred message localization value, either set
	// explicitly by the user or derived from the shell environment on the local system.
	LocalizationVariable = InvisiblePrefix + "localization"

	// This variable contains the line number currently being executed in the Ego source file.
	LineVariable = InvisiblePrefix + "line"

	// This is true if language extensions are enabled for this execution.
	ExtensionsVariable = InvisiblePrefix + "extensions"

	// This contains the module name currently being executed. This is either the source file
	// name or the function name.
	ModuleVariable = InvisiblePrefix + "module"

	// This is an array of string arrays that contain all the response headers set by a REST
	// service invocation. This value is accessed by the native REST dispatcher after the Ego
	// service runs to store the header values in the native HTTP response.
	ResponseHeaderVariable = InvisiblePrefix + "response_headers"

	// This contains the rest status code (200, 404, etc.) that was set by the Ego service
	// handler. This value is accessed by the native REST dispatcher after the Ego service
	// runs to store the header values in the native HTTP response.
	RestStatusVariable = InvisiblePrefix + "rest_status"

	// This is the name of the variable that is ignored. If this is the LVALUE (target) of
	// an assignment or storage operation in Ego, then the value is discarded and not set.
	DiscardedVariable = "_"

	// This contains the name of the superuser, if one is defined, in an Ego service.
	SuperUserVariable = ReadonlyVariablePrefix + "superuser"

	// This contains the password provided as part of Basic REST authentication, in
	// an Ego service.
	PasswordVariable = ReadonlyVariablePrefix + "password"

	// This contains the bearer token provided as part of Bearer REST authentication,
	// in an Ego service.
	TokenVariable = ReadonlyVariablePrefix + "token"

	// This indicates if the bearer tokenw as valid an unexpired, in an Ego service.
	TokenValidVariable = ReadonlyVariablePrefix + "token_valid"

	// This contains the version number of the Ego runtime.
	VersionName = ReadonlyVariablePrefix + "version"

	// This contains the copyright string for the current instance of Ego.
	CopyrightVariable = ReadonlyVariablePrefix + "copyright"

	// This contains the UUID value of this instance of the Ego server. This is reported
	// in the server log, and is always returned as part of a REST call.
	InstanceUUIDVariable = ReadonlyVariablePrefix + "instance"

	// This contains the process id of the currently-executing instance of Ego.
	PidVariable = ReadonlyVariablePrefix + "pid"

	// This contains the atomic integer sequence number for REST sessions that have connected
	// to this instance of the Ego server. This is used to generate unique session ids.
	SessionVariable = ReadonlyVariablePrefix + "session"

	// This contains the REST method string (GET, POST, etc.) for the current REST call.
	MethodVariable = ReadonlyVariablePrefix + "method"

	// This contains a string representation of the build time of the current instance of
	// Ego.
	BuildTimeVariable = ReadonlyVariablePrefix + "buildtime"

	// This contains an Ego structure containing information about the version of native Go
	// used to build Ego, the hardware platform, and the operating system.
	PlatformVariable = ReadonlyVariablePrefix + "platform"

	// This contains the start time for the REST handler. This can be used to calculate elapsed
	// time for the service.
	StartTimeVariable = ReadonlyVariablePrefix + "start_time"

	// This contains the network address of the entity that made the REST request, for an
	// Ego service.
	RequestorVariable = ReadonlyVariablePrefix + "requestor"

	// This contains a map of string arrays, containing the REST header values passed into
	// a REST request, for an Ego service.
	HeadersMapVariable = ReadonlyVariablePrefix + "headers"

	// This contains a map of string arrays, containing the REST query parameters passed into
	// a REST request, for an Ego service.
	ParametersVariable = ReadonlyVariablePrefix + "parms"

	// This global is set to true when the REST request specifies a JSON media type for its
	// response. This is used by the @JSON and @TEXT directives to determine how to format
	// the response.
	JSONMediaVariable = ReadonlyVariablePrefix + "json"
)

// This is the name for objects that otherwise have no name.
const Anon = "<anon>"

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

	// How many old logs do we maintain by default when in server mode?
	LogRetainCountSetting = ServerKeyPrefix + "retain.log.count"

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

// This section contains strings from the Go reflection package that describe the contains of
// opaque interface objects that Ego needs to interpret in more specific ways, typically when
// formatting a value.
const (
	ByteCodeReflectionTypeString        = "<*bytecode.ByteCode Value>"
	RuntimeFunctionReflectionTypeString = "<func(*symbols.SymbolTable, data.List) (interface {}, error) Value>"
	NilTypeString                       = "<nil>"
)

// This section enumerates the kinds of type enforcement that can be active in an Ego execution
// context.
const (
	// Types must match exactly, with no type coercion allowed.
	StrictTypeEnforcement = 0

	// Types are coerced to match, if possible, to match the target type or function argument type.
	RelaxedTypeEnforcement = 1

	// Types are not enforced at all. This is the default. This is the same as "dynamic" typing.
	// Tyeps of variables can be changed by assigning new values to the variable.
	NoTypeEnforcement = 2
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

// Cached items class names.

const (
	AssetCacheClass   = "asset"
	ServiceCacheClass = "service"
)
