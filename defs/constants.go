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
	// File system location used to locate services, lib,
	// and test directories.
	EgoPathSetting = "ego.path"

	// Do we normalize the case of all symbols to a common
	// (lower) case string. If not true, symbol names are
	// case-sensitive.
	CaseNormalizedSetting = "ego.compiler.normalized"

	// What is the output format that should be used by
	// default for operations that could return either
	// "text" , "indent", or "json" output.
	OutputFormatSetting = "ego.output-format"

	// If true, the script language includes language
	// extensions such as print, call, try/catch.
	ExtensionsEnabledSetting = "ego.compiler.extensions"

	// Should an interactive session automatically import
	// all the pre-defined packages?
	AutoImportSetting = "ego.compiler.import"

	// Should the interactive RUN mode exit when the user
	// enters a blank line on the console?
	ExitOnBlankSetting = "ego.console.exit.on.blank"

	// Should the copyright message be omitted when in
	// interactive prompt mode?
	NoCopyrightSetting = "ego.console.no.copyright"

	// Should the interactive command input processor use
	// readline?
	UseReadline = "ego.console.readline"

	// Set to true if the full stack should be listed during
	// tracing.
	FullStackListingSetting = "ego.compiler.full.stack"

	// Should the Ego program(s) be run with "static" or
	// "dynamic" typing? The default is "dynamic".
	StaticTypesSetting = "ego.compiler.types"

	// Feature flag to enable native structures (rather than
	// maps with hidden metadata). Defaults to false.
	NativeStructuresSetting = "ego.compiler.native.structures"

	// The base URL of the Ego server providing application services.
	ApplicationServerSetting = "ego.application.server"

	// The base URL of the Ego server providing logon services.
	LogonServerSetting = "ego.logon.server"

	// The last token created by a ego logon command, which
	// is used by default for server admin commands as well
	// as rest calls.
	LogonTokenSetting = "ego.logon.token"

	// The default user if no userdatabase has been initialized
	// yet. This is a strong of the form "user:password", which
	// is defined as the root user.
	DefaultCredentialSetting = "ego.server.default-credential"

	// If present, this user is always assigned super-user (root)
	// privileges regardless of the userdata settings. This can be
	// used to access an encrypted user data file.
	LogonSuperuserSetting = "ego.server.superuser"

	// The file system location where the user database is stored.
	LogonUserdataSetting = "ego.server.userdata"

	// The encryption key for the userdata file. If not present,
	// the file is not encrypted and is readable json.
	LogonUserdataKeySetting = "ego.server.userdata.key"

	// The URL path for the tables database functionality
	TablesServerDatabase = "ego.server.database.url"

	// The user:password credentials to use with the local tables database
	TablesServerDatabaseCredentials = "ego.server.database.credentials"

	// The name of the tables database in the local database store (schemas
	// are used to partition this database by Ego username)
	TablesServerDatabaseName = "ego.server.database.name"

	// Boolean indicating if the communication with the tables database
	// should be done using SSL secured communications
	TablesServerDatabaseSSLMode = "ego.server.database.ssl"

	// The key string used to encrypt authentication tokens.
	TokenKeySetting = "ego.token.key"

	// A string indicating the duration of a token before it is
	// considered expired. Examples are "15m" or "24h".
	TokenExpirationSetting = "ego.token.expiration"

	// If true, functions that return multiple values including an
	// error that do not assign that error to a value will result in
	// the error being thrown.
	ThrowUncheckedErrorsSetting = "ego.runtime.unchecked.errors"

	// If true, the TRACE operation will print the full stack instead of
	// a shorter single-line version.
	FullStackTraceSetting = "ego.runtime.stack.trace"

	// If specified, has the Go-style format string to be used for log
	// messages showing the time of the event.
	LogTimestampFormat = "ego.log.timestamp"

	// If specified, all filename references in ego programs (such as the
	// ReadFile() function) must start with this path, or it will be prefixed
	// with this path. This lets you limit where/how the files can be managed
	// by an ego program. This is especially important in server mode.
	SandboxPathSetting = "ego.sandbox.path"

	// PidDirectorySettings has the path used to store and find PID files for
	// server invocations and management.
	PidDirectorySetting = "ego.server.piddir"

	// If true, the default state for staring an Ego is to not require HTTPS/SSL
	// but rather run in "insecure" mode.
	InsecureServerSetting = "ego.server.insecure"
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
