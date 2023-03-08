package commands

import (
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/http/admin"
	"github.com/tucats/ego/http/assets"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/http/tables"
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
		Class(server.AdminRequestCounter)

	// Get the status of the server cache.
	router.New(defs.AdminCachesPath, admin.GetCacheHandler, http.MethodGet).
		Authentication(true, true).
		Class(server.AdminRequestCounter)

	// Set the size of the cache.
	router.New(defs.AdminCachesPath, admin.SetCacheSizeHandler, http.MethodPut).
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

	ui.Log(ui.ServerLogger, "Enabling /tables endpoints")

	// Handlers that manipulate a table are defined the in tables package.
	tables.AddStaticRoutes(router)

	return router
}
