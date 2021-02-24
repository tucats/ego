package defs

// The authorization scheme attached to the bearer token
// in REST calls.
const AuthScheme = "bearer "

// DefaultUserdataFileName is the default file system name of
// the user database file, if not specified by the user.
const DefaultUserdataFileName = "user-database.json"

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

	// Should the Ego program(s) be run with "static" or
	// "dynamic" typing? The default is "dynamic".
	StaticTypesSetting = "ego.compiler.types"

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
	LogonUserdataSetting = "ego.server.userdata.file"

	// The encryption key for the userdata file. If not present,
	// the file is not encrypted and is readable json.
	LogonUserdataKeySetting = "ego.server.userdata.key"

	// The key string used to encrypt authentication tokens.
	TokenKeySetting = "ego.token.key"

	// A string indicating the duration of a token before it is
	// considered expired. Examples are "15m" or "24h".
	TokenExpirationSetting = "ego.token.expiration"
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
