package defs

const (
	TableParameterName    = "table"
	SchemaParameterName   = "schema"
	UserParameterName     = "user"
	ColumnParameterName   = "columns"
	FilterParameterName   = "filter"
	SortParameterName     = "sort"
	StartParameterName    = "start"
	LimitParameterName    = "limit"
	RowCountParameterName = "rowcounts"
	AbstractParameterName = "abstract"
)

const (
	AdminCachesPath           = "/admin/caches"
	AdminHeartbeatPath        = "/admin/heartbeat"
	AdminLoggersPath          = "/admin/loggers/"
	AdminUsersPath            = "/admin/users/"
	AdminUsersNamePath        = AdminUsersPath + "%s"
	AssetsPath                = "/assets/"
	ServicesPath              = "/services/"
	ServicesDownPath          = ServicesPath + "admin/down/"
	ServicesLogonPath         = ServicesPath + "admin/logon/"
	ServicesUpPath            = ServicesPath + "up/"
	TablesPath                = "/tables/"
	TablesNamePath            = TablesPath + "%s"
	TablesRowsPath            = TablesPath + "{{table}}/rows"
	TablesSQLPath             = TablesPath + "@sql"
	TablesPermissionsPath     = TablesPath + "@permissions"
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

	EgoMediaType            = "application/vnd.ego."
	SQLStatementsMediaType  = EgoMediaType + "sql+json"
	RowSetMediaType         = EgoMediaType + "rows+json"
	AbstractRowSetMediaType = EgoMediaType + "rows.abstract+json"
	RowCountMediaType       = EgoMediaType + "rowcount+json"
	TableMetadataMediaType  = EgoMediaType + "columns+json"
	TablesMediaType         = EgoMediaType + "tables+json"
	ErrorMediaType          = EgoMediaType + "error+json"
	UserMediaType           = EgoMediaType + "user+json"
	UsersMediaType          = EgoMediaType + "users+json"
	LogStatusMediaType      = EgoMediaType + "log.status+json"
	LogLinesMediaType       = EgoMediaType + "log.lines+json"
	CacheMediaType          = EgoMediaType + "cache+json"
)

const (
	UserAuthenticationRequired  = "user"
	TokenRequired               = "token"
	AdminAuthneticationRequired = "admin"
	AdminTokenRequired          = "admintoken"
)

const (
	ContentTypeHeader       = "Content-Type"
	AuthenticateHeader      = "Www-Authenticate"
	EgoServerInstanceHeader = "X-Ego-Server"
)

// ServerInstanceID is the UUID of the current Server Instance.
var ServerInstanceID string
