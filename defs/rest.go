package defs

const (
	TableParameterName  = "table"
	SchemaParameterName = "schema"
	UserParameterName   = "user"
	ColumnParameterName = "columns"
	FilterParameterName = "filter"
	SortParameterName   = "sort"
	StartParameterName  = "start"
	LimitParameterName  = "limit"
)

const (
	AdminCachesPath           = "/admin/caches"
	AdminHeartbeatPath        = "/admin/heartbeat"
	AdminLoggersPath          = "/admin/loggers/"
	AdminUsersPath            = "/admin/users/"
	AdminUsersNamePath        = AdminUsersPath + "%s"
	AssetsPath                = "/assets/"
	CodePath                  = "/code"
	ServicesPath              = "/services/"
	ServicesLogonPath         = ServicesPath + "admin/logon/"
	ServicesUpPath            = ServicesPath + "up/"
	TablesPath                = "/tables/"
	TablesNamePath            = TablesPath + "%s"
	TablesRowsPath            = TablesNamePath + "/rows"
	TablesSQLPath             = TablesPath + "@sql"
	TablesPermissionsPath     = TablesPath + "@permissions"
	TablesNamePermissionsPath = TablesNamePath + "/permissions"
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
