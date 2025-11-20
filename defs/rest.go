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
	AdminPath                 = "/admin/"
	AdminCachesPath           = AdminPath + "caches"
	AdminHeartbeatPath        = AdminPath + "heartbeat"
	AdminLoggersPath          = AdminPath + "loggers/"
	AdminUsersPath            = AdminPath + "users/"
	AdminMemoryPath           = AdminPath + "memory"
	AdminTokenPath            = AdminPath + "tokens/"
	AdminTokenIDPath          = AdminTokenPath + "{{id}}"
	AdminValidationPath       = AdminPath + "validation/"
	AdminUsersNamePath        = AdminUsersPath + "%s"
	AdminConfigPath           = AdminPath + "config"
	AssetsPath                = "/assets/"
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
	TokensMediaType               = EgoMediaType + "tokens.list+json"
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

// This is a list of headers we are allowed to report in the log for REST
// operations.
var NonSensitiveRestHeaders = map[string]bool{
	"accept":            true,
	"accept-encoding":   true,
	"accept-language":   true,
	"accept-range":      true,
	"accept-signature":  true,
	"cache-control":     true,
	"content-digest":    true,
	"content-length":    true,
	"content-location":  true,
	"content-md5":       true,
	"content-range":     true,
	"content-type":      true,
	"date":              true,
	"prefer":            true,
	"range":             true,
	"user-agent":        true,
	"from":              true,
	"via":               true,
	"x-forwarded-for":   true,
	"x-forwarded-proto": true,
	"x-real-ip":         true,
	"host":              true,
	"allow":             true,
}
