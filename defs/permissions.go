package defs

// Define all permission names.

const (
	RootPermission        = "ego.root"
	LogonPermission       = "ego.logon"
	TableReadPermission   = "ego.tables.read"
	TableWritePermission  = "ego.tables.write"
	TableUpdatePermission = "ego.tables.update"
	TableDeletePermission = "ego.tables.delete"
	TableAdminPermission  = "ego.tables.admin"
	DSNAdminPermission    = "ego.dsns.admin"
	ServerAdminPermission = "ego.server.admin"
)

// This list is used to validate permission names. It contains a list of all
// possible permission names.
var AllPermissions = []string{
	RootPermission,
	LogonPermission,
	TableReadPermission,
	TableWritePermission,
	TableUpdatePermission,
	TableDeletePermission,
	TableAdminPermission,
	DSNAdminPermission,
	ServerAdminPermission,
}
