package defs

const (
	TableParameterName     = "table"
	SchemaParameterName    = "schema"
	UserParameterName      = "user"
	ColumnParameterName    = "columns"
	FilterParameterName    = "filter"
	SortParameterName      = "sort"
	StartParameterName     = "start"
	LimitParameterName     = "limit"
	RowCountParameterName  = "rowcounts"
	AbstractParameterName  = "abstract"
	UpsertParameterName    = "upsert"
	PermissionsPseudoTable = "@permissions"
	SQLPseudoTable         = "@sql"
)

const (
	AdminCachesPath           = "/admin/caches"
	AdminHeartbeatPath        = "/admin/heartbeat"
	AdminLoggersPath          = "/admin/loggers/"
	AdminUsersPath            = "/admin/users/"
	AdminMemoryPath           = "/admin/memory"
	AdminValidationPath       = "/admin/validation/"
	AdminUsersNamePath        = AdminUsersPath + "%s"
	AssetsPath                = "/assets/"
	ConfigPath                = "/admin/config"
	DSNPath                   = "/dsns/"
	DSNNamePath               = DSNPath + "{{dsn}}/"
	DSNTablesPath             = DSNNamePath + "tables/"
	DSNTablesNamePath         = DSNTablesPath + "%s"
	DSNTablesRowsPath         = DSNTablesPath + "{{table}}/rows"
	DSNSTablesSQLPath         = DSNTablesPath + SQLPseudoTable
	ServicesPath              = "/services/"
	ServicesDownPath          = ServicesPath + "admin/down/"
	ServicesLogonPath         = ServicesPath + "admin/logon/"
	ServicesLogLinesPath      = ServicesPath + "admin/log"
	ServicesAuthenticatePath  = ServicesPath + "admin/authenticate"
	ServicesUpPath            = ServicesPath + "up/"
	TablesPath                = "/tables/"
	TablesNamePath            = TablesPath + "%s"
	TablesRowsPath            = TablesPath + "{{table}}/rows"
	TablesSQLPath             = TablesPath + SQLPseudoTable
	TablesPermissionsPath     = TablesPath + PermissionsPseudoTable
	TablesNamePermissionsPath = TablesPath + "{{table}}/permissions"
)

var TableColumnTypeNames []string = []string{
	"int",
	"int32",
	"int64",
	"string",
	"float",
	"double",
	"float32",
	"float64",
	"time",
	"timestamp",
	"date",
	"bool",
}

const (
	TextMediaType = "application/text"
	JSONMediaType = "application/json"
	HTMLMediaType = "application/html"

	EgoMediaType                  = "application/vnd.ego."
	SQLStatementsMediaType        = EgoMediaType + "sql+json"
	RowSetMediaType               = EgoMediaType + "rows+json"
	AbstractRowSetMediaType       = EgoMediaType + "rows.abstract+json"
	RowCountMediaType             = EgoMediaType + "rowcount+json"
	TableMetadataMediaType        = EgoMediaType + "columns+json"
	TablesMediaType               = EgoMediaType + "tables+json"
	ErrorMediaType                = EgoMediaType + "error+json"
	UserMediaType                 = EgoMediaType + "user+json"
	DSNMediaType                  = EgoMediaType + "dsn+json"
	DSNPermissionsType            = EgoMediaType + "dsn.permissions+json"
	DSNListPermsMediaType         = EgoMediaType + "dsn.permissions.list+json"
	DSNListMediaType              = EgoMediaType + "dsns+json"
	UsersMediaType                = EgoMediaType + "users+json"
	LogStatusMediaType            = EgoMediaType + "log.status+json"
	LogLinesMediaType             = EgoMediaType + "log.lines+json"
	CacheMediaType                = EgoMediaType + "cache+json"
	MemoryMediaType               = EgoMediaType + "memory+json"
	LogonMediaType                = EgoMediaType + "logon+json"
	ConfigListMediaType           = EgoMediaType + "config.list+json"
	ConfigMediaType               = EgoMediaType + "config+json"
	ValidationDictionaryMediaType = EgoMediaType + "validation.dictionary+json"
)

const (
	UserAuthenticationRequired  = "user"
	TokenRequired               = "token"
	AdminAuthenticationRequired = "admin"
	NoAuthenticationRequired    = "none"
	AdminTokenRequired          = "admintoken"
)

const (
	ContentTypeHeader       = "Content-Type"
	AuthenticateHeader      = "WWW-Authenticate"
	EgoServerInstanceHeader = "X-Ego-Server"
)

// InstanceID is the UUID of the current Server Instance.
var InstanceID string
