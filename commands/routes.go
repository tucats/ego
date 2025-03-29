package commands

import (
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/admin"
	"github.com/tucats/ego/server/admin/caches"
	"github.com/tucats/ego/server/admin/users"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables"
	xui "github.com/tucats/ego/server/ui"
	"github.com/tucats/ego/util"
)

const (
	nameParameter = "{{name}}"
)

func defineStaticRoutes() *server.Router {
	// Let's use a private router for more flexibility with path patterns and providing session
	// context to the handler functions.
	router := server.NewRouter(defs.InstanceID)

	// Establish the admin endpoints
	ui.Log(ui.ServerLogger, "server.endpoints.admin", nil)

	// Define the payload validations
	server.InitializeValidations()

	// Get the current status of the server))
	// Get all config values
	router.New(defs.ConfigPath, admin.GetAllConfigHandler, http.MethodGet).
		Authentication(true, true).
		Class(server.AdminRequestCounter).
		AcceptMedia(defs.ConfigMediaType)

	// Get specific config values
	router.New(defs.ConfigPath, admin.GetConfigHandler, http.MethodPost).
		Authentication(true, true).
		Class(server.AdminRequestCounter).
		AcceptMedia(defs.ConfigMediaType)

	// Get the current memory status
	router.New(defs.AdminMemoryPath, admin.GetMemoryHandler, http.MethodGet).
		Authentication(true, true).
		Class(server.AdminRequestCounter).
		Permissions("admin_read")

	// Get the current validation dictionary. Can request a specific method and
	// path to retrieve using parameters.
	router.New(defs.AdminValidationPath, admin.GetValidationsHandler, http.MethodGet).
		Authentication(true, true).
		Class(server.AdminRequestCounter).
		Parameter("method", util.StringParameterType).
		Parameter("path", util.StringParameterType).
		Parameter("entry", util.StringParameterType).
		Disallow("entry:path,method").
		Permissions("admin_read")

	// Get the current table metadata
	// Read an asset from disk or cache.
	router.New(defs.AssetsPath+"{{item}}", assets.AssetsHandler, http.MethodGet).
		Class(server.AssetRequestCounter)

	// Create a new user
	router.New(defs.AdminUsersPath, users.CreateUserHandler, http.MethodPost).
		Authentication(true, true).
		Class(server.AdminRequestCounter).
		Permissions("admin_users")

	// Delete an existing user
	router.New(defs.AdminUsersPath+nameParameter, users.DeleteUserHandler, http.MethodDelete).
		Authentication(true, true).
		Class(server.AdminRequestCounter).
		Permissions("admin_users")

	// List user(s)
	router.New(defs.AdminUsersPath, users.ListUsersHandler, http.MethodGet).
		Authentication(true, true).
		Class(server.AdminRequestCounter).
		Permissions("admin_users", "admin_read")

	// Get a specific user
	router.New(defs.AdminUsersPath+nameParameter, users.GetUserHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.UserMediaType).
		Class(server.AdminRequestCounter).
		Permissions("admin_users", "admin_read")

	// Modify a specific user
	router.New(defs.AdminUsersPath+nameParameter, users.UpdateUserHandler, http.MethodPatch).
		Authentication(true, true).
		AcceptMedia(defs.UserMediaType).
		Class(server.AdminRequestCounter).
		Permissions("admin_users")

	// Get the status of the server cache.
	router.New(defs.AdminCachesPath, caches.GetCacheHandler, http.MethodGet).
		Authentication(true, true).
		Parameter("order-by", util.StringParameterType).
		Class(server.AdminRequestCounter).
		Permissions("admin_read")

	// Set the size of the cache.
	router.New(defs.AdminCachesPath, caches.SetCacheSizeHandler, http.MethodPost).
		Authentication(true, true).
		Class(server.AdminRequestCounter)

	// Purge all items from the cache.
	router.New(defs.AdminCachesPath, caches.PurgeCacheHandler, http.MethodDelete).
		Authentication(true, true).
		Class(server.AdminRequestCounter).
		Permissions("admin_server")

	// Get the current logging status
	router.New(defs.AdminLoggersPath, admin.GetLoggingHandler, http.MethodGet).
		Authentication(true, true).
		Class(server.AdminRequestCounter).
		Permissions("admin_server")

	// Purge old logs
	router.New(defs.AdminLoggersPath, admin.PurgeLogHandler, http.MethodDelete).
		Authentication(true, true).
		Parameter("keep", util.IntParameterType).
		Class(server.AdminRequestCounter).
		Permissions("admin_server")

	// Set loggers
	router.New(defs.AdminLoggersPath, admin.SetLoggingHandler, http.MethodPost).
		Authentication(true, true).
		Class(server.AdminRequestCounter).
		Permissions("admin_server")

	// Simplest possible "are you there" endpoint.
	router.New(defs.AdminHeartbeatPath, admin.HeartbeatHandler, http.MethodGet).
		LightWeight(true).
		Class(server.HeartbeatRequestCounter)

	ui.Log(ui.ServerLogger, "server.endpoints.dsn", nil)

	// List all DSNS
	router.New(defs.DSNPath, dsns.ListDSNHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.DSNListMediaType).
		Parameter("limit", util.IntParameterType).
		Parameter("start", util.IntParameterType).
		Class(server.TableRequestCounter).
		Permissions("admin_read", "admin_dsns")

	// Create a new DSN
	router.New(defs.DSNPath, dsns.CreateDSNHandler, http.MethodPost).
		Authentication(true, true).
		AcceptMedia(defs.DSNMediaType).
		Class(server.TableRequestCounter).
		Permissions("admin_dsns")

	// Read an existing DSN
	router.New(defs.DSNNamePath, dsns.GetDSNHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.DSNMediaType).
		Class(server.TableRequestCounter).
		Permissions("admin_read", "admin_dsns")

	// Delete an existing DSN
	router.New(defs.DSNNamePath, dsns.DeleteDSNHandler, http.MethodDelete).
		Authentication(true, true).
		AcceptMedia(defs.DSNMediaType).
		Class(server.TableRequestCounter).
		Permissions("admin_dsns")

	// Add or delete DSN permissions
	router.New(defs.DSNPath+defs.PermissionsPseudoTable, dsns.DSNPermissionsHandler, http.MethodPost).
		Authentication(true, true).
		AcceptMedia(defs.DSNPermissionsType).
		Class(server.TableRequestCounter).
		Permissions("admin_dsns")

	// List permissions for a DSN
	router.New(defs.DSNNamePath+defs.PermissionsPseudoTable, dsns.ListDSNPermHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.DSNListPermsMediaType).
		Class(server.TableRequestCounter).
		Permissions("admin_read", "admin_dsns")

	ui.Log(ui.ServerLogger, "server.endpoints.tables", nil)

	// Handlers that manipulate a table are defined the in tables package.
	tables.AddStaticRoutes(router)

	// Handlers for the UI
	router.New("/ui/dsns", xui.HTMLDataSourceNamesHandler, http.MethodGet).
		Authentication(true, false).
		Class(server.TableRequestCounter)

	router.New("/ui/users", xui.HTMLUsersHandler, http.MethodGet).
		Authentication(true, false).
		Class(server.TableRequestCounter)

	return router
}
