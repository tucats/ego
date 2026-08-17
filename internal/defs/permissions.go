package defs

// Define all permission names.

const (
	RootPermission        = "ego.root"
	LogonPermission       = "ego.logon"
	CodeRunPermission     = "ego.code"
	SQLPermission         = "ego.sql"
	TableReadPermission   = "ego.table.read"
	TableWritePermission  = "ego.table.write"
	TableUpdatePermission = "ego.table.update"
	TableDeletePermission = "ego.table.delete"
	TableAdminPermission  = "ego.table.admin"
	DSNAdminPermission    = "ego.dsn.admin"
	DSNReadPermission     = "ego.dsn.read"
	DSNWritePermission    = "ego.dsn.write"
	ServerAdminPermission = "ego.server.admin"
)

// This list is used to validate permission names. It contains a list of all
// possible permission names.
var AllPermissions = []string{
	RootPermission,
	LogonPermission,
	CodeRunPermission,
	SQLPermission,
	TableReadPermission,
	TableWritePermission,
	TableUpdatePermission,
	TableDeletePermission,
	TableAdminPermission,
	DSNAdminPermission,
	ServerAdminPermission,
}
