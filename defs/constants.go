package defs

// The authorization scheme attached to the bearer token
// in REST calls
const AuthScheme = "token "

// DefaultUserdataFileName is the default file system name of
// the user database file, if not specified by the user.
const DefaultUserdataFileName = "user-database.json"

// This is the default media type (for Content and Accept headers)
// for REST calls based on JSON
const JSONMediaType = "application/json"

// This section describes the profile keys used by Ego

const (

	/////////////////////////////////////////////////////
	//                                                 //
	//       Settings used by all Ego commands         //
	//                                                 //
	/////////////////////////////////////////////////////

	// File system location used to locate services, lib,
	// and test directories
	EgoPathSetting = "ego-path"

	// Do we normalize the case of all symbols to a common
	// (lower) case string. If not true, symbol names are
	// case-sensitive.
	CaseNormalizedSetting = "case-normalized"

	// What is the output format that should be used by
	// default for operations that could return either
	// "text" or "json" output.
	OutputFormatSetting = "output-format"

	// If true, the script language includes language
	// exteions such as print, call, try/catch.
	ExtensionsEnabledSetting = "extensions-enabled"

	/////////////////////////////////////////////////////
	//                                                 //
	//        Settings used by the RUN command         //
	//                                                 //
	/////////////////////////////////////////////////////

	// Should an interactive session automatically import
	// all the pre-defined packages?
	AutoImportSetting = "auto-import"

	// Should the interactive RUN mode exit when the user
	// enters a blank line on the console?
	ExitOnBlankSetting = "exit-on-blank"

	// Should the copyright message be omitted when in
	// interactive prompt mode?
	NoCopyrightSetting = "no-copyright"

	// Should the interactive command input processor use
	// readline?
	UseReadline = "use-readline"

	// Should the Ego program(s) be run with static (true)
	// or dynamic (false) types?
	StaticTypesSetting = "static-types"

	/////////////////////////////////////////////////////
	//                                                 //
	//       Settings used Ego server clients          //
	//                                                 //
	/////////////////////////////////////////////////////

	// The base URL of the Ego server providing application services
	ApplicationServerSetting = "application-server"

	// The base URL of the Ego server providing logon services
	LogonServerSetting = "logon-server"

	// The last token created by a ego logon command, which
	// is used by default for server admin commands as well
	// as rest calls.
	LogonTokenSetting = "logon-token"

	/////////////////////////////////////////////////////
	//                                                 //
	//          Settings used by Ego servers           //
	//                                                 //
	/////////////////////////////////////////////////////

	// The default user if no userdatabase has been initialized
	// yet. This is a strong of the form "user:password", which
	// is defined as the root user.
	DefaultCredentialSetting = "default-credential"

	// If present, this user is always assigned super-user (root)
	// privileges regardless of the userdata settings. This can be
	// used to access an encrypted user data file.
	LogonSuperuserSetting = "logon-superuser"

	// The file system location where the user database is stored.
	LogonUserdataSetting = "logon-userdata"

	// The encryption key for the userdata file. If not present,
	// the file is not encrypted and is readable json.
	LogonUserdataKeySetting = "logon-userdata-key"

	// The key string used to encrypt authentication tokens.
	TokenKeySetting = "token-key"

	// A string indicating the duration of a token before it is
	// considered expired. Examples are "15m" or "24h"
	TokenExpirationSetting = "token-expiration"
)
