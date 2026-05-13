package commands

import (
	"net/http"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/router"
	"github.com/tucats/ego/server/admin"
	"github.com/tucats/ego/server/admin/caches"
	"github.com/tucats/ego/server/admin/users"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/tables"
	"github.com/tucats/ego/util"
)

const (
	nameParameter = "{{name}}"
)

func defineStaticRoutes() *router.Router {
	// Let's use a private r for more flexibility with path patterns and providing session
	// context to the handler functions.
	r := router.NewRouter(defs.InstanceID)

	// Establish the admin endpoints
	ui.Log(ui.ServerLogger, "server.endpoints.admin", nil)

	// Define the payload validations
	router.InitializeValidations()

	// Get the current status of the server))
	// Get all config values
	r.New(defs.AdminConfigPath, admin.GetAllConfigHandler, http.MethodGet).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		AcceptMedia(defs.ConfigMediaType)

	// Get specific config values
	r.New(defs.AdminConfigPath, admin.GetConfigHandler, http.MethodPost).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		AcceptMedia(defs.ConfigMediaType)

	// Get the current memory status
	r.New(defs.AdminMemoryPath, admin.GetMemoryHandler, http.MethodGet).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Compile and run Ego code submitted from the dashboard Code tab
	r.New(defs.AdminRunPath, admin.RunCodeHandler, http.MethodPost).
		Authentication(true, false).
		Class(router.AdminRequestCounter).
		Permissions(defs.CodeRunPermission)

	// Get the current validation dictionary. Can request a specific method and
	// path to retrieve using parameters.
	r.New(defs.AdminValidationPath, admin.GetValidationsHandler, http.MethodGet).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		Parameter("method", util.StringParameterType).
		Parameter("path", util.StringParameterType).
		Parameter("entry", util.StringParameterType).
		Disallow("entry:path,method").
		Permissions(defs.ServerAdminPermission)

	// Start the dashboard UI
	r.New(defs.UIPath, admin.UIHandler, http.MethodGet).
		Class(router.AdminRequestCounter)

	// Start the idtrack issue-tracker UI (only when enabled via configuration).
	if settings.GetBool(defs.IDTrackSetting) {
		r.New(defs.IDTrackPath, admin.IDTrackHandler, http.MethodGet).
			Class(router.AdminRequestCounter)
	}

	// Read an asset from disk or cache.
	r.New(defs.AssetsPath+"{{item...}}", assets.AssetsHandler, http.MethodGet).
		Class(router.AssetRequestCounter)

	// Create a new user
	r.New(defs.AdminUsersPath, users.CreateUserHandler, http.MethodPost).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Delete an existing user
	r.New(defs.AdminUsersPath+nameParameter, users.DeleteUserHandler, http.MethodDelete).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// List user(s)
	r.New(defs.AdminUsersPath, users.ListUsersHandler, http.MethodGet).
		Authentication(true, true).
		Parameter(defs.StartParameterName, util.IntParameterType).
		Parameter(defs.LimitParameterName, util.IntParameterType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Get a specific user
	r.New(defs.AdminUsersPath+nameParameter, users.GetUserHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.UserMediaType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Modify a specific user
	r.New(defs.AdminUsersPath+nameParameter, users.UpdateUserHandler, http.MethodPatch).
		Authentication(true, true).
		AcceptMedia(defs.UserMediaType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Get the status of the server cache.
	r.New(defs.AdminCachesPath, caches.GetCacheHandler, http.MethodGet).
		Authentication(true, true).
		Parameter("order-by", util.StringParameterType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Set the size of the cache.
	r.New(defs.AdminCachesPath, caches.SetCacheSizeHandler, http.MethodPost).
		Authentication(true, true).
		Class(router.AdminRequestCounter)

	// Purge items from the cache.
	r.New(defs.AdminCachesPath, caches.PurgeCacheHandler, http.MethodDelete).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission).
		Parameter("class", util.ListParameterType)

	// Get the current logging status
	r.New(defs.AdminLoggersPath, admin.GetLoggingHandler, http.MethodGet).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Purge old logs
	r.New(defs.AdminLoggersPath, admin.PurgeLogHandler, http.MethodDelete).
		Authentication(true, true).
		Parameter("keep", util.IntParameterType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Set loggers
	r.New(defs.AdminLoggersPath, admin.SetLoggingHandler, http.MethodPost).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Simplest possible "are you there" endpoint.
	r.New(defs.AdminHeartbeatPath, admin.HeartbeatHandler, http.MethodGet).
		LightWeight(true).
		Class(router.HeartbeatRequestCounter)

	// Add a token ID to the blacklist for this server
	r.New(defs.AdminTokenPath, admin.TokenRevokeHandler, http.MethodPut).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Get the list of all blacklisted tokens
	r.New(defs.AdminTokenPath, admin.TokenListHandler, http.MethodGet).
		Authentication(true, true).
		Parameter(defs.StartParameterName, util.IntParameterType).
		Parameter(defs.LimitParameterName, util.IntParameterType).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Delete an individual token from the blacklist
	r.New(defs.AdminTokenIDPath, admin.TokenDeleteHandler, http.MethodDelete).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Flush/delete the entire blacklist
	r.New(defs.AdminTokenPath, admin.TokenFlushHandler, http.MethodDelete).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Get overall server status (mashup of memory and caches, really)
	r.New(defs.AdminResourcesPath, admin.GetResourcesHandler, http.MethodGet).
		Authentication(true, true).
		Class(router.AdminRequestCounter).
		Permissions(defs.ServerAdminPermission)

	ui.Log(ui.ServerLogger, "server.endpoints.dsn", nil)

	// List all DSNS
	r.New(defs.DSNPath, dsns.ListDSNHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.DSNListMediaType).
		Parameter("limit", util.IntParameterType).
		Parameter("start", util.IntParameterType).
		Class(router.TableRequestCounter).
		Permissions(defs.ServerAdminPermission)

	// Create a new DSN
	r.New(defs.DSNPath, dsns.CreateDSNHandler, http.MethodPost).
		Authentication(true, true).
		AcceptMedia(defs.DSNMediaType).
		Class(router.TableRequestCounter).
		Permissions(defs.DSNAdminPermission)

	// Read an existing DSN
	r.New(defs.DSNNamePath, dsns.GetDSNHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.DSNMediaType).
		Class(router.TableRequestCounter).
		Permissions(defs.DSNAdminPermission)

	// Delete an existing DSN
	r.New(defs.DSNNamePath, dsns.DeleteDSNHandler, http.MethodDelete).
		Authentication(true, true).
		AcceptMedia(defs.DSNMediaType).
		Class(router.TableRequestCounter).
		Permissions(defs.DSNAdminPermission)

	// Add or delete DSN permissions
	r.New(defs.DSNPath+defs.PermissionsPseudoTable, dsns.DSNPermissionsHandler, http.MethodPost).
		Authentication(true, true).
		AcceptMedia(defs.DSNPermissionsType).
		Class(router.TableRequestCounter).
		Permissions(defs.DSNAdminPermission)

	// List permissions for a DSN
	r.New(defs.DSNNamePath+defs.PermissionsPseudoTable, dsns.ListDSNPermHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.DSNListPermsMediaType).
		Class(router.TableRequestCounter).
		Permissions(defs.DSNAdminPermission)

	ui.Log(ui.ServerLogger, "server.endpoints.tables", nil)

	// Handlers that manipulate a table are defined the in tables package.
	tables.AddStaticRoutes(r)

	return r
}
