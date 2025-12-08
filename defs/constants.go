package defs

// The authorization scheme attached to the bearer token
// in REST calls.
const AuthScheme = "bearer "

// Name of the default local host, in TCP/IP standards.
const LocalHost = "localhost"

// DefaultUserdataFileName is the default file system name of
// the user database file, if not specified by the user.
const DefaultUserdataFileName = "sqlite3://users.db"

// This section contains constants used by file operations.
const (
	// The subdirectory in "EGO_PATH" where the .ego runtime files and assets
	// are found.
	LibPathName = "lib"

	// The file extension for Ego programs".
	EgoFilenameExtension = ".ego"
)

// When used in a grammar specification for the number of parameters expected
// to a verb, this means it's a variable number of parameters including possibly
// zero parameters.
const VariableParameterCount = -99

// Constants used to define the location of request and payload JSON files
// when running the server in "child services" mode. Typically the string
// is the identity of the server (usually the UUID) and the integer is the
// session sequence number.
const (
	ChildRequestFileFormat  = "ego-request-%s-%d.json"
	ChildResponseFileFormat = "ego-response-%s-%d.json"
)

// This is the name for objects that otherwise have no name.
const Anon = "<anon>"

const ElidedPassword = "********"

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
	VerboseOption         = "verbose"
	DSNOption             = "dsn"
	GrantOption           = "grant"
	RevokeOption          = "revoke"
	UsernameOption        = "username"
	PasswordOption        = "password"
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
	AdminPriv = "admin"
	ReadPriv  = "read"
	WritePriv = "write"
)

const (
	DefaultAdminUsername = "admin"
	DefaultAdminPassword = "password"
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
	// The data types of variables can be changed just by assigning new values to the variable.
	NoTypeEnforcement = 2
)

const (
	APIVersion = 1
)

// Cached items class names.

const (
	AssetCacheClass   = "asset"
	ServiceCacheClass = "service"
)

// Native type names we do special handling for.
const (
	TimeDurationTypeName = "time.Duration"
	TimeMonthTypeName    = "time.Month"
	TimeLocationTypeName = "time.Location"
)
