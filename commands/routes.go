package commands

import (
	"net/http"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/http/admin"
	"github.com/tucats/ego/http/assets"
	"github.com/tucats/ego/http/server"
	"github.com/tucats/ego/http/services"
	"github.com/tucats/ego/http/tables"
	"github.com/tucats/ego/util"
)

func defineStatusRoutes(includeCode bool) *server.Router {

	// Let's use a private router for more flexibility with path patterns and providing session
	// context to the handler functions.
	router := server.NewRouter(defs.ServerInstanceID)

	// Do we enable the /code endpoint? This is off by default.
	if includeCode {
		router.New(defs.CodePath, services.CodeHandler, server.AnyMethod).
			Authentication(true, true).
			Class(server.CodeRequestCounter)

		ui.Log(ui.ServerLogger, "Enabling /code endpoint")
	}

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

	// Handlers that manipulate a table.

	// Run a transaction script
	router.New(defs.TablesPath+"@transaction", tables.TransactionHandler, http.MethodPost).
		Authentication(true, false).
		Permissions("table_read", "table_modify").
		Parameter(defs.FilterParameterName, defs.Any).
		AcceptMedia(defs.RowCountMediaType).
		Class(server.TableRequestCounter)

	// List all tables.
	router.New(defs.TablesPath, tables.ListTablesHandler, http.MethodGet).
		Authentication(true, false).
		Permissions("table_read").
		Parameter(defs.FilterParameterName, defs.Any).
		AcceptMedia(defs.TablesMediaType).
		Class(server.TableRequestCounter)

	// Read rows from a table.
	router.New(defs.TablesPath+"{{table}}/rows", tables.ReadRows, http.MethodGet).
		Authentication(true, false).
		Permissions("table_read").
		Parameter(defs.StartParameterName, data.IntTypeName).
		Parameter(defs.LimitParameterName, data.IntTypeName).
		Parameter(defs.ColumnParameterName, "list").
		Parameter(defs.SortParameterName, "list").
		Parameter(defs.AbstractParameterName, data.BoolTypeName).
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, data.StringTypeName).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		Class(server.TableRequestCounter)

	// Insert rows into a table.
	router.New(defs.TablesPath+"{{table}}/rows", tables.InsertRows, http.MethodPut).
		Authentication(true, false).
		Permissions("table_modify").
		Parameter(defs.AbstractParameterName, data.BoolTypeName).
		Parameter(defs.UserParameterName, data.StringTypeName).
		AcceptMedia(defs.RowSetMediaType, defs.AbstractRowSetMediaType).
		Class(server.TableRequestCounter)

	// Delete rows from a table.
	router.New(defs.TablesPath+"{{table}}/rows", tables.DeleteRows, http.MethodDelete).
		Authentication(true, false).
		Permissions("table_modify").
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, data.StringTypeName).
		AcceptMedia(defs.RowCountMediaType).
		Class(server.TableRequestCounter)

	// Update rows from a table.
	router.New(defs.TablesPath+"{{table}}/rows", tables.UpdateRows, http.MethodPatch).
		Authentication(true, false).
		Permissions("table_modify").
		Parameter(defs.FilterParameterName, defs.Any).
		Parameter(defs.UserParameterName, data.StringTypeName).
		AcceptMedia(defs.RowCountMediaType).
		Class(server.TableRequestCounter)

	// Read permissions for a table
	router.New(defs.TablesPath+"{{table}}/permissions", tables.ReadPermissions, http.MethodGet).
		Authentication(true, false).
		Permissions("table_admin").
		Parameter(defs.UserParameterName, data.StringTypeName).
		Class(server.TableRequestCounter)

	// Grant permissions for a table
	router.New(defs.TablesPath+"{{table}}/permissions", tables.ReadPermissions, http.MethodPut).
		Authentication(true, false).
		Permissions("table_admin").
		Parameter(defs.UserParameterName, data.StringTypeName).
		Class(server.TableRequestCounter)

	// Revoke permissions from a table
	router.New(defs.TablesPath+"{{table}}/permissions", tables.DeletePermissions, http.MethodDelete).
		Authentication(true, false).
		Permissions("table_admin").
		Parameter(defs.UserParameterName, data.StringTypeName).
		Class(server.TableRequestCounter)

	// Get metadata for a table
	router.New(defs.TablesPath+"{{table}}", tables.ReadTable, http.MethodGet).
		Authentication(true, false).
		Parameter(defs.UserParameterName, data.StringTypeName).
		AcceptMedia(defs.TableMetadataMediaType).
		Class(server.TableRequestCounter)

	// Read all permissions data using the "@permissions" pseudo-table-name.
	router.New(defs.TablesPath+"@permissions", tables.ReadAllPermissions, http.MethodGet).
		Authentication(true, true).
		Parameter(defs.UserParameterName, data.StringTypeName).
		Class(server.TableRequestCounter)

	// Execute arbitrary SQL using the "@sql" pseudo-table-name.
	router.New(defs.TablesPath+"@sql", tables.SQLTransaction, http.MethodPut).
		Authentication(true, true).
		Class(server.TableRequestCounter)

	// Create a new table
	router.New(defs.TablesPath+"{{table}}", tables.TableCreate, http.MethodPut).
		Authentication(true, false).
		Permissions("table_update").
		Class(server.TableRequestCounter)

	// Delete a table
	router.New(defs.TablesPath+"{{table}}", tables.DeleteTable, http.MethodDelete).
		Authentication(true, false).
		Permissions("table_update").
		Class(server.TableRequestCounter)

	return router
}
