package defs

// The authorization scheme attached to the bearer token
// in REST calls.
const AuthScheme = "bearer "

// DefaultUserdataFileName is the default file system name of
// the user database file, if not specified by the user.
const DefaultUserdataFileName = "sqlite3://users.db"

// This is the default media type (for Content and Accept headers)
// for REST calls based on JSON.
const JSONMediaType = "application/json"

// This section describes the profile keys used by Ego

const (
	// The prefix for all configuration keys reserved to Ego.
	PrivilegedKeyPrefix = "ego."

	// File system location used to locate services, lib,
	// and test directories.
	EgoPathSetting = PrivilegedKeyPrefix + "runtime.path"

	// Do we normalize the case of all symbols to a common
	// (lower) case string. If not true, symbol names are
	// case-sensitive.
	CaseNormalizedSetting = PrivilegedKeyPrefix + "compiler.normalized"

	// What is the output format that should be used by
	// default for operations that could return either
	// "text" , "indent", or "json" output.
	OutputFormatSetting = PrivilegedKeyPrefix + "console.format"

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

	// Should the Ego program(s) be run with "static" or
	// "dynamic" typing? The default is "dynamic".
	StaticTypesSetting = PrivilegedKeyPrefix + "compiler.types"

	// Feature flag to enable native structures (rather than
	// maps with hidden metadata). Defaults to false.
	NativeStructuresSetting = PrivilegedKeyPrefix + "compiler.native.structures"

	// The base URL of the Ego server providing application services.
	ApplicationServerSetting = PrivilegedKeyPrefix + "application.server"

	// The base URL of the Ego server providing logon services.
	LogonServerSetting = PrivilegedKeyPrefix + "logon.server"

	// The last token created by a ego logon command, which
	// is used by default for server admin commands as well
	// as rest calls.
	LogonTokenSetting = PrivilegedKeyPrefix + "logon.token"

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

	// The URL path for the tables database functionality
	TablesServerDatabase = PrivilegedKeyPrefix + "server.database.url"

	// The user:password credentials to use with the local tables database
	TablesServerDatabaseCredentials = PrivilegedKeyPrefix + "server.database.credentials"

	// The name of the tables database in the local database store (schemas
	// are used to partition this database by Ego username)
	TablesServerDatabaseName = PrivilegedKeyPrefix + "server.database.name"

	// Boolean indicating if the communication with the tables database
	// should be done using SSL secured communications
	TablesServerDatabaseSSLMode = PrivilegedKeyPrefix + "server.database.ssl"

	// The key string used to encrypt authentication tokens.
	ServerTokenKeySetting = PrivilegedKeyPrefix + "server.token.key"

	// A string indicating the duration of a token before it is
	// considered expired. Examples are "15m" or "24h".
	ServerTokenExpirationSetting = PrivilegedKeyPrefix + "server.token.expiration"

	// If true, functions that return multiple values including an
	// error that do not assign that error to a value will result in
	// the error being thrown.
	ThrowUncheckedErrorsSetting = PrivilegedKeyPrefix + "runtime.unchecked.errors"

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
)

// This section contains the names of the command-line options. These often
// (but not always) have parallels in the settings above. Settings typically
// have a structured name (ego.compiler.autoimport) while the option name is
// Unix shell-friendly (auto-import).
const (
	AutoImportOption      = "auto-import"
	DisassembleOption     = "disassemble"
	FullSymbolScopeOption = "full-symbol-scope"
	StaticTypesOption     = "static-types"
	SymbolTableSizeOption = "symbol-table-size"
)

// Agent identifiers for REST calls, which indicate the role of the client.
const (
	AdminAgent  = "admin"
	ClientAgent = "rest client"
	LogonAgent  = "logon"
	TableAgent  = "tables"
)

// This map describes the list of valid settings, and whether they can be set by the
// command line.
var ValidSettings map[string]bool = map[string]bool{
	EgoPathSetting:                  true,
	CaseNormalizedSetting:           true,
	OutputFormatSetting:             true,
	ExtensionsEnabledSetting:        true,
	AutoImportSetting:               true,
	ExitOnBlankSetting:              true,
	NoCopyrightSetting:              true,
	UseReadline:                     true,
	FullStackListingSetting:         true,
	StaticTypesSetting:              true,
	NativeStructuresSetting:         true,
	ApplicationServerSetting:        true,
	LogonServerSetting:              true,
	LogonTokenSetting:               true,
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
}
