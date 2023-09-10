package commands

import (
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/server/admin"
	"github.com/tucats/ego/server/assets"
	"github.com/tucats/ego/server/dsns"
	"github.com/tucats/ego/server/server"
	"github.com/tucats/ego/server/tables"
	xui "github.com/tucats/ego/server/ui"
	"github.com/tucats/ego/util"
)

func defineStaticRoutes() *server.Router {
	// Let's use a private router for more flexibility with path patterns and providing session
	// context to the handler functions.
	router := server.NewRouter(defs.ServerInstanceID)

	// Establish the admin endpoints
	ui.Log(ui.ServerLogger, "Enabling /admin endpoints")

	// Read an asset from disk or cache.
	router.New(defs.AssetsPath+"{{item}}", assets.AssetsHandler, http.MethodGet).
		Class(server.AssetRequestCounter)

	// Create a new user
	router.New(defs.AdminUsersPath, admin.CreateUserHandler, http.MethodPost).
		Authentication(true, true).
		Class(server.AdminRequestCounter)

	// Delete an existing user
	router.New(defs.AdminUsersPath+"{{name}}", admin.DeleteUserHandler, http.MethodDelete).
		Authentication(true, true).
		Class(server.AdminRequestCounter)

	// List user(s)
	router.New(defs.AdminUsersPath, admin.ListUsersHandler, http.MethodGet).
		Authentication(true, true).
		Class(server.AdminRequestCounter)

	// Get a specific user
	router.New(defs.AdminUsersPath+"{{name}}", admin.GetUserHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.UserMediaType).
		Class(server.AdminRequestCounter)

	// Modify a specific user
	router.New(defs.AdminUsersPath+"{{name}}", admin.UpdateUserHandler, http.MethodPatch).
		Authentication(true, true).
		AcceptMedia(defs.UserMediaType).
		Class(server.AdminRequestCounter)

	// Get the status of the server cache.
	router.New(defs.AdminCachesPath, admin.GetCacheHandler, http.MethodGet).
		Authentication(true, true).
		Class(server.AdminRequestCounter)

	// Set the size of the cache.
	router.New(defs.AdminCachesPath, admin.SetCacheSizeHandler, http.MethodPost).
		Authentication(true, true).
		Class(server.AdminRequestCounter)

	// Purge all items from the cache.
	router.New(defs.AdminCachesPath, admin.PurgeCacheHandler, http.MethodDelete).
		Authentication(true, true).
		Class(server.AdminRequestCounter)

	// Get the current logging status
	router.New(defs.AdminLoggersPath, admin.GetLoggingHandler, http.MethodGet).
		Authentication(true, true).
		Class(server.AdminRequestCounter)

	// Purge old logs
	router.New(defs.AdminLoggersPath, admin.PurgeLogHandler, http.MethodDelete).
		Authentication(true, true).
		Parameter("keep", util.IntParameterType).
		Class(server.AdminRequestCounter)

	// Set loggers
	router.New(defs.AdminLoggersPath, admin.SetLoggingHandler, http.MethodPost).
		Authentication(true, true).
		Class(server.AdminRequestCounter)

	// Simplest possible "are you there" endpoint.
	router.New(defs.AdminHeartbeatPath, admin.HeartbeatHandler, http.MethodGet).
		LightWeight(true).
		Class(server.HeartbeatRequestCounter)

	ui.Log(ui.ServerLogger, "Enabling /dsn endpoints")

	// List all DSNS
	router.New(defs.DSNPath, dsns.ListDSNHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.DSNListMediaType).
		Class(server.TableRequestCounter)

	// Create a new DSN
	router.New(defs.DSNPath, dsns.CreateDSNHandler, http.MethodPost).
		Authentication(true, true).
		AcceptMedia(defs.DSNMediaType).
		Class(server.TableRequestCounter)

	// Read an existing DSN
	router.New(defs.DSNNamePath, dsns.GetDSNHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.DSNMediaType).
		Class(server.TableRequestCounter)

	// Delete an existing DSN
	router.New(defs.DSNNamePath, dsns.DeleteDSNHandler, http.MethodDelete).
		Authentication(true, true).
		AcceptMedia(defs.DSNMediaType).
		Class(server.TableRequestCounter)

	// Add or delete DSN permissions
	router.New(defs.DSNPath+defs.PermissionsPseudoTable, dsns.DSNPermissionsHandler, http.MethodPost).
		Authentication(true, true).
		AcceptMedia(defs.DSNPermissionsType).
		Class(server.TableRequestCounter)

	// List permissions for a DSN
	router.New(defs.DSNNamePath+defs.PermissionsPseudoTable, dsns.ListDSNPermHandler, http.MethodGet).
		Authentication(true, true).
		AcceptMedia(defs.DSNListPermsMediaType).
		Class(server.TableRequestCounter)

	ui.Log(ui.ServerLogger, "Enabling /tables endpoints")

	// Handlers that manipulate a table are defined the in tables package.
	tables.AddStaticRoutes(router)

	// Handler for the UI
	router.New("/ui/dsns", xui.HTMLdsnsHandler, http.MethodGet).
		Authentication(true, false).
		Class(server.TableRequestCounter)

	return router
}
