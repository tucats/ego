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
//
// DATA-SECURITY.md §3.5: DSNReadPermission and DSNWritePermission used to be
// missing from this list, even though both constants were already defined
// above and already in active use as per-DSN grant action names (see
// commands/dsns.go's setPermissions and DSNPermissionsHandler). Since every
// identity-permission grant path -- the admin REST API
// (server/admin/users/create.go, update.go) and the CLI
// (commands/users.go) -- validates against this exact list, an operator
// could not actually grant a user identity-level ego.dsn.read or
// ego.dsn.write: PATCH /users/{name} or `ego user update --grant
// ego.dsn.read` was rejected with ErrInvalidPermission before ever
// reaching the code (DATA-SECURITY.md §3.4, dsns.IdentityAuthorizesAction)
// that would honor it. Only ego.dsn.admin, already on this list, ever
// worked as an identity-wide DSN permission.
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
	DSNReadPermission,
	DSNWritePermission,
	ServerAdminPermission,
}
